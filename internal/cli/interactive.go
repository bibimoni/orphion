package cli

import (
	"fmt"
	"strings"

	"github.com/distiled/orphion/internal/app"
	"github.com/pterm/pterm"
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

	// Welcome
	pterm.DefaultBasicText.Println("Orphion - Anime & Drama Downloader")
	pterm.DefaultBasicText.Println("")

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

	// Step 3: Search for titles
	result, err := service.Search(ctx, query, resType)
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}
	if len(result.Anime) == 0 {
		return fmt.Errorf("no results for %q", query)
	}

	// Step 4: Select an anime
	opts := make([]string, len(result.Anime))
	titleToID := make(map[string]string, len(result.Anime))
	for i, a := range result.Anime {
		opts[i] = a.Title
		titleToID[a.Title] = a.ID
	}
	selectedTitle, err := pterm.DefaultInteractiveSelect.
		WithOptions(opts).
		WithDefaultText("Select anime:").
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

	pterm.Info.Println(fmt.Sprintf("Downloading episodes: %s", epExpr))

	// Step 6: Download
	downloadResult, _, err := service.DownloadEpisodes(ctx, animeID, epExpr, selectedTitle)
	if err != nil {
		return err
	}

	pterm.Success.Println(
		fmt.Sprintf("Download complete: %d completed, %d failed",
			downloadResult.Completed, downloadResult.Failed))

	return nil
}