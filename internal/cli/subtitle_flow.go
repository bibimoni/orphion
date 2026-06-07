package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pterm/pterm"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/common"
	"github.com/distiled/orphion/internal/paths"
	"github.com/distiled/orphion/internal/subtitle"
)

// SubtitleFlowConfig controls the behavior of the subtitle selection flow.
type SubtitleFlowConfig struct {
	// Query is the search query. If empty, the user is prompted.
	Query string

	// BaseDir is the output directory for the subtitle files.
	// If empty, service.OutputDir() is used.
	BaseDir string

	// SkipConfirm skips the "Download subtitles?" confirmation prompt.
	// Used in standalone mode where the user already expressed intent.
	SkipConfirm bool
}

// SubtitleFlowResult holds the result of a completed subtitle flow.
type SubtitleFlowResult struct {
	Subtitle *subtitle.Subtitle // chosen subtitle (nil if canceled)
	OutDir   string             // output directory for the subtitle
}

// RunSubtitleFlow executes the full subtitle selection flow:
//  1. Confirm (unless SkipConfirm)
//  2. Search & auto-match or manual pick
//  3. Season selection (if needed)
//  4. Language filtering
//  5. Table display + subtitle file selection
//  6. Output folder selection
//
// Returns nil result (no error) if the user cancels at any step.
func RunSubtitleFlow(ctx context.Context, service *app.Service, cfg SubtitleFlowConfig) (*SubtitleFlowResult, error) {
	if service.SubtitleProvider() == nil {
		return nil, nil
	}

	// Step 1: Confirm (interactive mode only).
	if !cfg.SkipConfirm {
		wantSubs, _ := pterm.DefaultInteractiveConfirm.
			WithDefaultValue(true).
			WithDefaultText("Download subtitles?").
			Show()
		if !wantSubs {
			return nil, nil
		}
	}

	// Step 2: Get search query.
	query := cfg.Query
	if query == "" {
		if !isTerminal(os.Stdin) {
			return nil, fmt.Errorf("query required in non-interactive mode")
		}
		q, err := pterm.DefaultInteractiveTextInput.WithDefaultText("Search subtitles: ").Show()
		if err != nil {
			return nil, fmt.Errorf("input: %w", err)
		}
		query = cleanUserInput(q)
	}
	if query == "" {
		return nil, nil
	}

	// Step 3: Search & auto-match.
	// Use a simple print instead of a spinner — spinners and interactive
	// prompts (select, text input) fight over the terminal and cause
	// duplicate lines or garbled output.
	pterm.Info.Printfln("Searching subtitles for %q...", query)
	searchCtx, cancel := context.WithTimeout(ctx, common.SubtitleSearchTimeout)
	defer cancel()

	chosenResult, err := selectSubtitleResult(searchCtx, service, query)
	if err != nil {
		return nil, err
	}
	if chosenResult == nil {
		return nil, nil
	}
	pterm.Success.Printfln("Found subtitle result: %s", chosenResult.Title)

	// Step 4: Load subtitle page (auto-fetches first season if needed).
	pageSpinner, _ := pterm.DefaultSpinner.Start("Loading subtitles...")
	page, err := service.SubtitlePage(ctx, chosenResult.ID, chosenResult.Slug, "")
	if err != nil {
		pageSpinner.Fail(fmt.Sprintf("Failed: %s", err))
		return nil, err
	}

	// Step 5: Season selection (if needed).
	seasonSlug := ""
	if len(page.Subtitles) == 0 && len(page.Seasons) > 1 {
		pageSpinner.Success(fmt.Sprintf("Found %d season(s)", len(page.Seasons)))
		seasonOpts := make([]string, len(page.Seasons)+1)
		seasonOpts[0] = backOption
		for i, s := range page.Seasons {
			seasonOpts[i+1] = s.Name
		}
		seasonSel, err := interactiveSelect(seasonOpts, "Select season:")
		if err != nil {
			return nil, fmt.Errorf("season selection: %w", err)
		}
		if seasonSel == backOption {
			return nil, nil
		}
		for _, s := range page.Seasons {
			if s.Name == seasonSel {
				seasonSlug = s.Slug
				break
			}
		}
		// Re-fetch for the selected season.
		page, err = service.SubtitlePage(ctx, chosenResult.ID, chosenResult.Slug, seasonSlug)
		if err != nil {
			return nil, fmt.Errorf("load season: %w", err)
		}
	} else if len(page.Subtitles) > 0 {
		pageSpinner.Success(fmt.Sprintf("Found %d subtitle(s)", len(page.Subtitles)))
	}

	if len(page.Subtitles) == 0 {
		pterm.Info.Println("No subtitles available for this title.")
		return nil, nil
	}

	// Step 6: Filter by preferred language.
	preferredLang := service.SubtitleLang()
	langSubs, hasLangMatch := filterByLang(page.Subtitles, preferredLang)
	if !hasLangMatch && len(page.Subtitles) > 0 {
		pterm.Info.Printfln("No %s subtitles found, showing all languages", preferredLang)
		langSubs = page.Subtitles
	}

	if len(langSubs) == 0 {
		pterm.Info.Println("No subtitles available")
		return nil, nil
	}

	// Step 7: Display subtitle table.
	tableData := pterm.TableData{{pterm.Cyan("#"), pterm.Cyan("Language"), pterm.Cyan("Quality"), pterm.Cyan("Author"), pterm.Cyan("Downloads")}}
	for i, sub := range langSubs {
		tableData = append(tableData, []string{
			fmt.Sprintf("%d", i+1),
			sub.Language,
			sub.Quality,
			sub.Author,
			fmt.Sprintf("%d", sub.Downloads),
		})
	}
	_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

	// Step 8: Select subtitle file.
	subOpts := make([]string, len(langSubs)+1)
	subOpts[0] = backOption
	for i, sub := range langSubs {
		subOpts[i+1] = subtitleFileLabel(sub)
	}
	subSel, err := interactiveSelect(subOpts, "Select subtitle file:")
	if err != nil {
		return nil, fmt.Errorf("subtitle selection: %w", err)
	}
	if subSel == backOption {
		return nil, nil
	}

	var chosenSub *subtitle.Subtitle
	for i, sub := range langSubs {
		if subtitleFileLabel(sub) == subSel {
			chosenSub = &langSubs[i]
			break
		}
	}
	if chosenSub == nil {
		return nil, fmt.Errorf("invalid subtitle selection")
	}

	// Step 9: Select output folder.
	baseDir := cfg.BaseDir
	if baseDir == "" {
		baseDir = service.OutputDir()
	}
	baseDir = paths.ExpandTilde(baseDir)
	outDir, err := selectOutputFolder(baseDir, chosenResult.Title, seasonSlug)
	if err != nil {
		return nil, fmt.Errorf("output folder: %w", err)
	}

	return &SubtitleFlowResult{
		Subtitle: chosenSub,
		OutDir:   outDir,
	}, nil
}

// selectOutputFolder presents an interactive folder picker that:
//  1. Lists existing folders in baseDir ranked by similarity to the title
//  2. Offers a default path (baseDir/TitleToDir(title))
//  3. Lets the user type a custom folder name
//
// Returns the full output path.
func selectOutputFolder(baseDir, title, seasonSlug string) (string, error) {
	// Compute the default subfolder name from the title.
	defaultName := paths.TitleToDir(title)
	if seasonSlug != "" {
		defaultName = filepath.Join(defaultName, seasonSlug)
	}
	defaultPath := filepath.Join(baseDir, defaultName)

	// Read existing folders in the base directory.
	folders := listFolders(baseDir)
	if len(folders) == 0 {
		// No existing folders — just use the default.
		pterm.Info.Printfln("Output: %s", pterm.LightBlue(defaultPath))
		return defaultPath, nil
	}

	// Rank folders by similarity to the title.
	ranked := subtitle.FolderMatch(title, folders)

	// Build options: default first, then ranked folders, then custom.
	opts := make([]string, 0, len(ranked)+2)
	opts = append(opts, useDefaultOption)
	for _, f := range ranked {
		if f == defaultName || f == title {
			continue // skip the default (already shown as option 0)
		}
		opts = append(opts, f)
	}
	opts = append(opts, customFolderOption)

	// Show the default path in the prompt text.
	selected, err := interactiveSelect(opts, fmt.Sprintf("Save to: %s", pterm.LightBlue(defaultPath)))
	if err != nil {
		return "", fmt.Errorf("folder selection: %w", err)
	}

	switch selected {
	case useDefaultOption:
		return defaultPath, nil
	case customFolderOption:
		custom, err := pterm.DefaultInteractiveTextInput.
			WithDefaultText("Folder name: ").
			Show()
		if err != nil {
			return "", fmt.Errorf("custom folder: %w", err)
		}
		custom = cleanUserInput(custom)
		if custom == "" {
			return defaultPath, nil
		}
		return filepath.Join(baseDir, custom), nil
	default:
		// User picked an existing folder name.
		return filepath.Join(baseDir, selected), nil
	}
}

// listFolders returns the names of immediate subdirectories in dir.
// Returns an empty slice if the directory doesn't exist or can't be read.
func listFolders(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	return names
}


