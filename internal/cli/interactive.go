package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/distiled/orphion/internal/app"
	"github.com/pterm/pterm"
	"github.com/pterm/pterm/putils"
	"github.com/spf13/cobra"
)

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
	query = strings.TrimSpace(query)
	if query == "" {
		return fmt.Errorf("search query cannot be empty")
	}

	// Step 2: Content type
	resType, err := pterm.DefaultInteractiveSelect.
		WithOptions([]string{"anime", "drama"}).
		WithDefaultText("Select type:").
		Show()
	if err != nil {
		return fmt.Errorf("type selection: %w", err)
	}

	// Step 3: Search for titles with spinner
	spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Searching for %q...", query))
	result, err := service.Search(ctx, query, resType)
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

	// Step 5: Episode expression
	epExpr, err := pterm.DefaultInteractiveTextInput.
		WithDefaultText("Episodes (e.g. 1-4,7,all): ").
		Show()
	if err != nil {
		return fmt.Errorf("episode input: %w", err)
	}

	// Step 6: Download with animated progress
	pterm.Info.Printfln("Downloading %s episodes: %s", pterm.Cyan(selectedTitle), epExpr)

	service.SetProgressCallback(downloadProgressCallback)

	downloadResult, _, err := service.DownloadEpisodes(ctx, animeID, epExpr, selectedTitle)
	if err != nil {
		return err
	}

	// Clear progress line and show final results.
	fmt.Fprintln(os.Stderr)

	if len(downloadResult.Outputs) > 0 {
		dir := outputDirFor(downloadResult.Outputs[0])
		pterm.Success.Printfln("Saved to %s", pterm.LightBlue(dir))
	}

	if downloadResult.Failed > 0 {
		pterm.Error.Printfln("%d completed, %d failed",
			downloadResult.Completed, downloadResult.Failed)
	} else {
		pterm.Success.Printfln("%d episode(s) downloaded", downloadResult.Completed)
	}

	return nil
}
