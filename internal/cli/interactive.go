package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/config"
	"github.com/distiled/orphion/internal/ffmpeg"
	"github.com/distiled/orphion/internal/provider"
)

// imeArtifactRe matches common IME/terminal escape artifacts that pterm
// captures as literal text (e.g. "alt+", "ctrl+", ESC sequences).
var imeArtifactRe = regexp.MustCompile(`(?i)^(?:alt\+|ctrl\+|meta\+|esc\b\s*)`)

// backOption is the label used for the "go back" choice in select prompts.
const backOption = "← Back"

// optionStyle is a brighter style for select options so they don't
// appear as grey text (pterm's ThemeDefault.DefaultText is FgDefault
// which renders as grey on most terminals).
var optionStyle = pterm.NewStyle(pterm.FgWhite)

var interactiveSelect = func(options []string, defaultText string) (string, error) {
	s := pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText(defaultText).
		WithMaxHeight(common.InteractiveMaxHeight)
	s.OptionStyle = optionStyle
	return s.Show()
}

var interactiveMultiSelect = func(options []string, defaultText string) ([]string, error) {
	s := pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithDefaultText(defaultText).
		WithCheckmark(&pterm.Checkmark{Checked: pterm.Green("✓"), Unchecked: " "}).
		WithMaxHeight(common.InteractiveMaxHeight)
	s.OptionStyle = optionStyle
	return s.Show()
}

// setInteractiveRoot configures the root command for interactive mode when
// invoked without a subcommand.
func setInteractiveRoot(root *cobra.Command, service *app.Service) {
	if service == nil {
		return
	}

	root.RunE = func(cmd *cobra.Command, args []string) error {
		return runInteractive(cmd, service)
	}
}

// step represents the interactive flow steps.
type step int

const (
	stepSearch    step = iota // Search query input
	stepProvider              // Provider selection
	stepTitle                 // Title selection from search results
	stepSource                // Source/episode selection
	stepConfig                // Config review
	stepSubtitles             // Subtitle selection (before download)
	stepDownload              // Download episodes + subtitles
)

func runInteractive(cmd *cobra.Command, service *app.Service) error {
	ctx := cmd.Context()

	// Clear the terminal screen (like ani-cli).
	pterm.Print("\x1b[2J\x1b[H")

	// Branded header.
	pterm.Println()
	_ = pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("Orphion", pterm.NewStyle(pterm.FgCyan)),
	).Render()
	pterm.Println(pterm.Gray("Search and download episodes"))

	var (
		query            string
		searchResult     app.SearchResult
		animeID          string
		selectedTitle    string
		episodes         []provider.Episode
		selectedEpisodes []provider.Episode
		pendingSubResult *SubtitleFlowResult // selected subtitle + output dir from the flow
	)

	current := stepSearch

	for {
		switch current {
		case stepSearch:
			q, err := pterm.DefaultInteractiveTextInput.WithDefaultText("Search").Show()
			if err != nil {
				return fmt.Errorf("search: %w", err)
			}
			q = cleanUserInput(q)
			if q == "" {
				pterm.Warning.Println("Search query cannot be empty")
				continue // stay on search step
			}
			query = q
			current = stepProvider

		case stepProvider:
			if err := selectInteractiveProvider(service); err != nil {
				return err
			}
			// Now search with the selected provider.
			spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Searching for %q...", query))
			result, err := service.Search(ctx, query, "")
			if err != nil {
				spinner.Fail(fmt.Sprintf("Search failed: %s", err))
				return fmt.Errorf("search: %w", err)
			}
			if len(result.Anime) == 0 {
				spinner.Fail("No results found")
				pterm.Info.Println("Try a different search query.")
				current = stepSearch
				continue
			}
			spinner.Success(fmt.Sprintf("Found %d result(s)", len(result.Anime)))
			searchResult = result
			current = stepTitle

		case stepTitle:
			opts := make([]string, len(searchResult.Anime)+1)
			titleToID := make(map[string]string, len(searchResult.Anime))
			// Add "Back" as the first option.
			opts[0] = backOption
			for i, a := range searchResult.Anime {
				opts[i+1] = a.Title
				titleToID[a.Title] = a.ID
			}
			titleSel := pterm.DefaultInteractiveSelect.
				WithOptions(opts).
				WithDefaultText("Select title")
			titleSel.OptionStyle = optionStyle
			selected, err := titleSel.Show()
			if err != nil {
				return fmt.Errorf("title selection: %w", err)
			}
			if selected == backOption {
				current = stepSearch
				continue
			}
			selectedTitle = selected
			animeID = titleToID[selected]
			current = stepSource

		case stepSource:
			srcSpinner, _ := pterm.DefaultSpinner.Start("Loading episodes...")
			eps, err := service.GetEpisodes(ctx, animeID)
			if err != nil {
				srcSpinner.Fail(fmt.Sprintf("Episode lookup failed: %s", err))
				return fmt.Errorf("episodes: %w", err)
			}
			if len(eps) == 0 {
				srcSpinner.Fail("No episodes found")
				pterm.Info.Println("Try selecting a different title.")
				current = stepTitle
				continue
			}
			srcSpinner.Success(fmt.Sprintf("Found %d episode(s)", len(eps)))
			episodes = eps

			sel, err := selectInteractiveEpisodes(episodes)
			if err != nil {
				if err == errBackSelected {
					current = stepTitle
					continue
				}
				return err
			}
			selectedEpisodes = sel
			current = stepConfig

		case stepConfig:
			cfg := service.Config()
			sessCfg, err := showConfigAndEdit(&config.Config{
				OutputDir:        cfg.OutputDir,
				PreferredQuality: cfg.PreferredQty,
				Concurrency:      cfg.Concurrency,
			})
			if err != nil {
				return fmt.Errorf("session config: %w", err)
			}
			applySessionConfig(service, sessCfg)
			current = stepSubtitles

		case stepSubtitles:
			selectedNumbers := make([]string, len(selectedEpisodes))
			for i, episode := range selectedEpisodes {
				selectedNumbers[i] = episode.Number
			}
			subResult, err := RunSubtitleFlow(ctx, service, SubtitleFlowConfig{
				Query:    selectedTitle,
				BaseDir:  service.OutputDir(),
				Episodes: selectedNumbers,
			})
			if err != nil {
				pterm.Info.Printfln("Subtitle selection failed: %s", err)
			} else if subResult != nil {
				pendingSubResult = subResult
			}
			current = stepDownload

		case stepDownload:
			tracker := newDownloadTracker()
			service.SetProgressCallback(func(episode string, progress ffmpeg.Progress) {
				tracker.update(episode, progress)
			})
			service.SetCompletedCallback(func(episode string) {
				tracker.markDone(episode)
			})

			downloadResult, _, err := service.DownloadSelectedEpisodes(ctx, selectedEpisodes, selectedTitle)
			tracker.stop()
			if err != nil {
				return err
			}

			// Show per-episode failures.
			if downloadResult.Failed > 0 {
				for ep, epErr := range downloadResult.Errors {
					pterm.Error.Printfln("Episode %s: %s", ep, epErr)
				}
				return fmt.Errorf("%d download(s) failed", downloadResult.Failed)
			}

			// Show output directory for completed downloads.
			if len(downloadResult.Outputs) > 0 {
				dir := outputDirFor(downloadResult.Outputs[0])
				pterm.Success.Printfln("Saved to %s", pterm.LightBlue(dir))
			} else {
				pterm.Success.Printfln("%d episode(s) downloaded", downloadResult.Completed)
			}

			// Download selected subtitle if one was chosen.
			if pendingSubResult != nil && len(pendingSubResult.Subtitles) > 0 {
				extractedCount := 0
				failedCount := 0
				for i, selectedSub := range pendingSubResult.Subtitles {
					dlSpinner, _ := pterm.DefaultSpinner.Start(
						fmt.Sprintf("Downloading subtitle %d/%d...", i+1, len(pendingSubResult.Subtitles)),
					)
					extracted, err := service.DownloadSubtitle(ctx, selectedSub, pendingSubResult.OutDir)
					if err != nil {
						failedCount++
						dlSpinner.Fail(fmt.Sprintf("Subtitle download failed: %s", err))
						continue
					}
					extractedCount += len(extracted)
					dlSpinner.Success(fmt.Sprintf("Extracted %d file(s)", len(extracted)))
				}
				if failedCount == 0 {
					pterm.Success.Printfln(
						"Saved %d subtitle file(s) to %s",
						extractedCount,
						pterm.LightBlue(pendingSubResult.OutDir),
					)
				} else {
					pterm.Warning.Printfln(
						"Saved %d subtitle file(s); %d download(s) failed",
						extractedCount,
						failedCount,
					)
				}
			}

			return nil
		}
	}
}

func selectInteractiveEpisodes(episodes []provider.Episode) ([]provider.Episode, error) {
	options := make([]string, len(episodes)+1)
	optionToEpisode := make(map[string]provider.Episode, len(episodes))
	// Add "Back" as the first option.
	options[0] = backOption
	for i, ep := range episodes {
		option := episodeOption(ep)
		if _, exists := optionToEpisode[option]; exists {
			option = fmt.Sprintf("%s [%d]", option, i+1)
		}
		options[i+1] = option
		optionToEpisode[option] = ep
	}
	selectedOptions, err := interactiveMultiSelect(options, "Select episodes (Enter toggles, Tab confirms)")
	if err != nil {
		return nil, fmt.Errorf("episode selection: %w", err)
	}
	if len(selectedOptions) == 0 {
		return nil, fmt.Errorf("no sources selected")
	}
	// Check if "Back" was selected.
	for _, opt := range selectedOptions {
		if opt == backOption {
			return nil, errBackSelected
		}
	}
	selected := make([]provider.Episode, 0, len(selectedOptions))
	for _, option := range selectedOptions {
		ep, ok := optionToEpisode[option]
		if !ok {
			return nil, fmt.Errorf("unknown source selection %q", option)
		}
		selected = append(selected, ep)
	}
	return selected, nil
}

// errBackSelected is returned when the user selects "← Back" in a multi-select.
var errBackSelected = errors.New("back selected")

func episodeOption(ep provider.Episode) string {
	parts := []string{fmt.Sprintf("Ep %s", ep.Number)}
	if ep.Size != "" {
		parts = append(parts, ep.Size)
	}
	if ep.Title != "" {
		parts = append(parts, ep.Title)
	}
	return truncateDisplay(strings.Join(parts, " | "), episodeOptionWidth())
}

func episodeOptionWidth() int {
	width := pterm.GetTerminalWidth()
	if width <= 0 {
		width = 80
	}
	// Reserve space for pterm's selector, checkbox, and margin.
	width -= 8
	if width < 32 {
		return 32
	}
	return width
}

func truncateDisplay(s string, maxWidth int) string {
	if maxWidth <= 0 || runewidth.StringWidth(s) <= maxWidth {
		return s
	}
	const suffix = "..."
	limit := maxWidth - runewidth.StringWidth(suffix)
	if limit <= 0 {
		return suffix
	}
	var b strings.Builder
	width := 0
	for _, r := range s {
		rw := runewidth.RuneWidth(r)
		if width+rw > limit {
			break
		}
		b.WriteRune(r)
		width += rw
	}
	return strings.TrimRight(b.String(), " |") + suffix
}

func selectInteractiveProvider(service *app.Service) error {
	providers := service.ProviderNames()
	if len(providers) == 0 {
		return nil
	}
	selected, err := interactiveSelect(providers, "Select provider")
	if err != nil {
		return fmt.Errorf("provider selection: %w", err)
	}
	if err := service.SetProvider(selected); err != nil {
		return fmt.Errorf("provider selection: %w", err)
	}
	return nil
}

// cleanUserInput trims whitespace and strips IME/terminal escape artifacts
// that pterm's InteractiveTextInput may capture as literal text on macOS
// (e.g. "alt+" prefix from Alt+key used to switch input methods).
func cleanUserInput(s string) string {
	s = strings.TrimSpace(s)
	// Remove leading IME artifacts (e.g. "alt+", "ctrl+").
	s = imeArtifactRe.ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}
