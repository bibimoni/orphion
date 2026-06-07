package cli

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mattn/go-runewidth"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/config"
	"github.com/distiled/orphion/internal/provider"
)

// imeArtifactRe matches common IME/terminal escape artifacts that pterm
// captures as literal text (e.g. "alt+", "ctrl+", ESC sequences).
var imeArtifactRe = regexp.MustCompile(`(?i)^(?:alt\+|ctrl\+|meta\+|esc\b\s*)`)

var interactiveSelect = func(options []string, defaultText string) (string, error) {
	return pterm.DefaultInteractiveSelect.
		WithOptions(options).
		WithDefaultText(defaultText).
		Show()
}

var interactiveMultiSelect = func(options []string, defaultText string) ([]string, error) {
	return pterm.DefaultInteractiveMultiselect.
		WithOptions(options).
		WithDefaultText(defaultText).
		WithCheckmark(&pterm.Checkmark{Checked: pterm.Green("✓"), Unchecked: " "}).
		WithMaxHeight(8).
		Show()
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

func runInteractive(cmd *cobra.Command, service *app.Service) error {
	ctx := cmd.Context()

	// Branded header.
	_ = pterm.DefaultBigText.WithLetters(
		putils.LettersFromStringWithStyle("Orphion", pterm.NewStyle(pterm.FgCyan)),
	).Render()
	pterm.Println(pterm.Gray("Search and download episodes"))

	// Step 1: Search text
	query, err := pterm.DefaultInteractiveTextInput.WithDefaultText("Search: ").Show()
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	query = cleanUserInput(query)
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	// Step 2: Provider
	if err := selectInteractiveProvider(service); err != nil {
		return err
	}

	// Step 3: Search for titles with spinner
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Searching for %q...", query))
	result, err := service.Search(ctx, query, "")
	if err != nil {
		spinner.Fail(fmt.Sprintf("Search failed: %s", err))
		return fmt.Errorf("search: %w", err)
	}
	if len(result.Anime) == 0 {
		spinner.Fail("No results found")
		return fmt.Errorf("no results for %q", query)
	}
	spinner.Success(fmt.Sprintf("Found %d result(s)", len(result.Anime)))

	// Step 4: Select an anime
	opts := make([]string, len(result.Anime))
	titleToID := make(map[string]string, len(result.Anime))
	for i, a := range result.Anime {
		opts[i] = a.Title
		titleToID[a.Title] = a.ID
	}
	selectedTitle, err := pterm.DefaultInteractiveSelect.
		WithOptions(opts).
		WithDefaultText("Select title:").
		Show()
	if err != nil {
		return fmt.Errorf("title selection: %w", err)
	}

	animeID := titleToID[selectedTitle]

	// Step 5: Select provider sources.
	srcSpinner, _ := pterm.DefaultSpinner.Start("Getting sources...")
	episodes, err := service.GetEpisodes(ctx, animeID)
	if err != nil {
		srcSpinner.Fail(fmt.Sprintf("Sources failed: %s", err))
		return fmt.Errorf("sources: %w", err)
	}
	if len(episodes) == 0 {
		srcSpinner.Fail("No sources found")
		return fmt.Errorf("no sources for %q", selectedTitle)
	}
	srcSpinner.Success(fmt.Sprintf("Found %d source(s)", len(episodes)))

	selectedEpisodes, err := selectInteractiveEpisodes(episodes)
	if err != nil {
		return err
	}

	// Step 6: Review/edit session config before downloading.
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

	// Step 7: Download with animated progress
	dlSpinner, _ := pterm.DefaultSpinner.Start("Getting episodes...")
	service.SetProgressCallback(newProgressCallback(dlSpinner))

	downloadResult, _, err := service.DownloadSelectedEpisodes(ctx, selectedEpisodes, selectedTitle)
	if err != nil {
		dlSpinner.Fail(fmt.Sprintf("Failed: %s", err))
		return err
	}

	// Show per-episode failures.
	if downloadResult.Failed > 0 {
		dlSpinner.Fail(fmt.Sprintf("%d completed, %d failed", downloadResult.Completed, downloadResult.Failed))
		for ep, epErr := range downloadResult.Errors {
			pterm.Error.Printfln("  Episode %s: %s", ep, epErr)
		}
		return fmt.Errorf("%d download(s) failed", downloadResult.Failed)
	}

	// Show output directory for completed downloads.
	if len(downloadResult.Outputs) > 0 {
		dir := outputDirFor(downloadResult.Outputs[0])
		dlSpinner.Success(fmt.Sprintf("Saved to %s", pterm.LightBlue(dir)))
	} else {
		dlSpinner.Success(fmt.Sprintf("%d episode(s) downloaded", downloadResult.Completed))
	}

	return nil
}

func selectInteractiveEpisodes(episodes []provider.Episode) ([]provider.Episode, error) {
	options := make([]string, len(episodes))
	optionToEpisode := make(map[string]provider.Episode, len(episodes))
	for i, ep := range episodes {
		option := episodeOption(ep)
		if _, exists := optionToEpisode[option]; exists {
			option = fmt.Sprintf("%s [%d]", option, i+1)
		}
		options[i] = option
		optionToEpisode[option] = ep
	}
	selectedOptions, err := interactiveMultiSelect(options, "Select source(s) (Enter select, Tab confirm):")
	if err != nil {
		return nil, fmt.Errorf("source selection: %w", err)
	}
	if len(selectedOptions) == 0 {
		return nil, fmt.Errorf("no sources selected")
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

func episodeOption(ep provider.Episode) string {
	parts := []string{fmt.Sprintf("Ep %s", ep.Number)}
	if ep.Size != "" {
		parts = append(parts, ep.Size)
	}
	if ep.Seeders > 0 {
		parts = append(parts, fmt.Sprintf("seeders=%d", ep.Seeders))
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
	selected, err := interactiveSelect(providers, "Select provider:")
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
