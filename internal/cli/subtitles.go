package cli

import (
	"fmt"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/distiled/orphion/internal/app"
)

const (
	// customFolderOption is the label for "type a custom folder name".
	customFolderOption = "✎ Type custom folder name"

	// useDefaultOption is the label for using the auto-generated default.
	useDefaultOption = "↵ Use default"
)

// newSubtitlesCmd creates the "subtitles" command.
func newSubtitlesCmd(service *app.Service) *cobra.Command {
	var (
		lang      string
		outputDir string
	)

	cmd := &cobra.Command{
		Use:   "subtitles",
		Short: "Search and download subtitles",
		Long:  "Search for subtitles on SubDL and download them as .srt/.ass files.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if service.SubtitleProvider() == nil {
				return fmt.Errorf("subtitle provider not configured")
			}

			// Override language if flag provided.
			if lang != "" {
				service.SetSubtitleLang(lang)
			}

			// Get query from arg or use empty to trigger prompt in the flow.
			var query string
			if len(args) > 0 {
				query = strings.TrimSpace(args[0])
			}

			ctx := cmd.Context()

			result, err := RunSubtitleFlow(ctx, service, SubtitleFlowConfig{
				Query:       query,
				BaseDir:     outputDir,
				SkipConfirm: true, // standalone mode — user already chose this command
			})
			if err != nil {
				return err
			}
			if result == nil || result.Subtitle == nil {
				// User canceled or no results.
				return nil
			}

			// Download and extract.
			dlSpinner, _ := pterm.DefaultSpinner.Start("Downloading subtitle...")
			files, err := service.DownloadSubtitle(ctx, *result.Subtitle, result.OutDir)
			if err != nil {
				dlSpinner.Fail(fmt.Sprintf("Download failed: %s", err))
				return err
			}
			if len(files) == 0 {
				dlSpinner.Warning("No subtitle files extracted (archive may be empty or unsupported format)")
				return nil
			}
			dlSpinner.Success(fmt.Sprintf("Downloaded %d subtitle file(s) to %s", len(files), pterm.LightBlue(result.OutDir)))
			for _, f := range files {
				pterm.Fprintln(cmd.OutOrStdout(), pterm.LightBlue(f))
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&lang, "lang", "", "Subtitle language (default: english)")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory (default: anime output dir)")

	return cmd
}
