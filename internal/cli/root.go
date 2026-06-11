// Package cli implements the command-line interface for Orphion.
package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/bibimoni/orphion/internal/app"
	"github.com/bibimoni/orphion/internal/config"
	"github.com/bibimoni/orphion/internal/ffmpeg"
)

// Version is set at build time.
var Version = "dev"

// configInitPath is the path used by "config init".
var configInitPath string

// SetConfigInitPath sets the path used by "orphion config init".
func SetConfigInitPath(p string) {
	configInitPath = p
}

// New creates the root command for Orphion.
func New(service *app.Service) *cobra.Command {
	root := &cobra.Command{
		Use:   "orphion",
		Short: "Search and download episodes",
		Long: fmt.Sprintf("%s\n\n%s",
			pterm.Cyan("Orphion"),
			"Search a catalog provider and download episodes as MKV files."),
		SilenceUsage: true,
	}
	setInteractiveRoot(root, service)

	root.AddCommand(newVersionCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newSearchCmd(service))
	root.AddCommand(newDownloadCmd(service))
	root.AddCommand(newSubtitlesCmd(service))

	return root
}

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			pterm.Fprintln(cmd.OutOrStdout(), fmt.Sprintf("orphion %s", pterm.Cyan(Version)))
			return nil
		},
	}
}

func newConfigCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "config",
		Short:   "Manage Orphion configuration",
		Example: "  orphion config init",
	}
	root.AddCommand(&cobra.Command{
		Use:     "init",
		Short:   "Create default configuration",
		Example: "  orphion config init",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configInitPath
			if path == "" {
				path = DefaultConfigPath()
			}
			if err := config.Init(path); err != nil {
				return err
			}
			pterm.Success.Printfln("Created config at %s", pterm.LightBlue(path))
			return nil
		},
	})
	return root
}

func newSearchCmd(service *app.Service) *cobra.Command {
	var resType string
	cmd := &cobra.Command{
		Use:     "search",
		Short:   "Search for titles",
		Example: "  orphion search \"Frieren\" --type anime",
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == nil {
				return fmt.Errorf("service not configured")
			}
			resType = strings.TrimSpace(resType)

			// When stdout is not a terminal (piped), output machine-readable
			// tab-separated format for scripting. Otherwise, show a rich TUI.
			if !isTerminal(os.Stdout) {
				result, err := service.Search(cmd.Context(), args[0], resType)
				if err != nil {
					return err
				}
				for _, a := range result.Anime {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", a.ID, a.Title)
				}
				return nil
			}

			spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Searching for %q...", args[0]))
			result, err := service.Search(cmd.Context(), args[0], resType)
			if err != nil {
				spinner.Fail(fmt.Sprintf("Search failed: %s", err))
				return err
			}
			spinner.Success(fmt.Sprintf("Found %d result(s)", len(result.Anime)))

			if len(result.Anime) == 0 {
				pterm.Info.Println("No results found.")
				return nil
			}

			// Render results as a styled table.
			tableData := pterm.TableData{{pterm.Cyan("ID"), pterm.Cyan("Title")}}
			for _, a := range result.Anime {
				tableData = append(tableData, []string{a.ID, a.Title})
			}
			_ = pterm.DefaultTable.WithHasHeader().WithData(tableData).Render()

			return nil
		},
	}
	cmd.Flags().StringVar(&resType, "type", "", "Content type: anime, drama, or empty for both")
	return cmd
}

func newDownloadCmd(service *app.Service) *cobra.Command {
	var (
		episodes    string
		title       string
		animeID     string
		resType     string
		quality     string
		outputDir   string
		concurrency int
		force       bool
	)

	cmd := &cobra.Command{
		Use:     "download",
		Short:   "Download episodes",
		Example: "  orphion download --title \"Frieren\" --episodes 1-4\n  orphion download --title-id \"allanime:abc123\" --episodes 1,3,7",
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == nil {
				return fmt.Errorf("service not configured")
			}
			animeID = strings.TrimSpace(animeID)
			title = strings.TrimSpace(title)
			episodes = strings.TrimSpace(episodes)
			resType = strings.TrimSpace(resType)
			quality = strings.TrimSpace(quality)
			outputDir = strings.TrimSpace(outputDir)

			if animeID == "" && title == "" {
				return fmt.Errorf("--title-id or --title is required")
			}
			if episodes == "" {
				return fmt.Errorf("--episodes is required")
			}

			// Apply CLI flag overrides to the session config.
			sessCfg := &sessionConfig{
				OutputDir:   service.Config().OutputDir,
				Quality:     service.Config().PreferredQty,
				Concurrency: service.Config().Concurrency,
				Force:       false,
			}
			if outputDir != "" {
				sessCfg.OutputDir = outputDir
			}
			if quality != "" {
				sessCfg.Quality = quality
			}
			if concurrency > 0 {
				sessCfg.Concurrency = concurrency
			}
			if force {
				sessCfg.Force = true
			}

			// In interactive terminals, show config and offer to edit.
			// Non-interactive (piped) skips the prompt.
			if isTerminal(os.Stdout) && !allFlagsSet(cmd, []string{"output", "quality", "concurrency", "force"}) {
				edited, err := showConfigAndEdit(&config.Config{
					OutputDir:        sessCfg.OutputDir,
					PreferredQuality: sessCfg.Quality,
					Concurrency:      sessCfg.Concurrency,
				})
				if err != nil {
					return fmt.Errorf("session config: %w", err)
				}
				sessCfg = edited
			}

			applySessionConfig(service, sessCfg)

			// Resolve title to ID if needed.
			if animeID == "" {
				spinner, _ := pterm.DefaultSpinner.Start(fmt.Sprintf("Resolving %q...", title))
				var err error
				animeID, err = service.ResolveID(cmd.Context(), title, resType)
				if err != nil {
					spinner.Fail(fmt.Sprintf("Resolve failed: %s", err))
					return err
				}
				spinner.Success(fmt.Sprintf("Resolved to %s", pterm.Cyan(animeID)))
			}

			// Set up multi-line progress display for concurrent downloads.
			tracker := newDownloadTracker()
			service.SetProgressCallback(func(episode string, progress ffmpeg.Progress) {
				tracker.update(episode, progress)
			})
			service.SetCompletedCallback(func(episode string) {
				tracker.markDone(episode)
			})

			result, _, err := service.DownloadEpisodes(cmd.Context(), animeID, episodes, title)
			tracker.stop()
			if err != nil {
				return err
			}

			// Show per-episode failures.
			if result.Failed > 0 {
				for ep, epErr := range result.Errors {
					pterm.Error.Printfln("Episode %s: %s", ep, epErr)
				}
				return &ExitError{code: 1, msg: "some downloads failed"}
			}

			// Show output directory for completed downloads.
			if len(result.Outputs) > 0 {
				dir := outputDirFor(result.Outputs[0])
				pterm.Success.Printfln("Saved to %s", pterm.LightBlue(dir))
			} else {
				pterm.Success.Printfln("%d episode(s) downloaded", result.Completed)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&episodes, "episodes", "", "Episode expression (e.g. 1-4,7)")
	cmd.Flags().StringVar(&title, "title", "", "Search query")
	cmd.Flags().StringVar(&animeID, "title-id", "", "Content ID")
	cmd.Flags().StringVar(&resType, "type", "", "Content type: anime, drama, or empty for both")
	cmd.Flags().StringVar(&quality, "quality", "", "Preferred quality (e.g. 1080p)")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Download concurrency (1-4)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files")

	return cmd
}

func formatProgressLine(episode string, progress ffmpeg.Progress) string {
	if progress.Phase == "resolving" {
		return fmt.Sprintf("%s Episode %s  resolving stream...",
			pterm.Cyan("↓"), episode)
	}

	// Segment download phase (bettermelon and similar HLS providers).
	if progress.Phase == "segments" && progress.SegmentsTotal > 0 {
		pct := float64(progress.SegmentsDone) / float64(progress.SegmentsTotal) * 100
		bar := progressBar(progress.SegmentsDone, progress.SegmentsTotal, 20)
		return fmt.Sprintf("%s Episode %s  %s %d/%d segments (%.0f%%)",
			pterm.Cyan("↓"), episode, bar, progress.SegmentsDone, progress.SegmentsTotal, pct)
	}

	if progress.Speed == "" && progress.Bytes == 0 && progress.TotalBytes == 0 {
		return fmt.Sprintf("%s Episode %s  connecting...",
			pterm.Cyan("↓"), episode)
	}

	speed := progress.Speed
	if speed == "" {
		speed = "..."
	}

	// Build size string; omit when no download data yet.
	var size string
	if progress.Bytes > 0 || progress.TotalBytes > 0 {
		size = formatBytes(progress.Bytes)
		if progress.TotalBytes > 0 {
			size = fmt.Sprintf("%s / %s", size, formatBytes(progress.TotalBytes))
		}
	}

	if size != "" {
		return fmt.Sprintf("%s Episode %s  %s  %s",
			pterm.Cyan("↓"), episode, pterm.Yellow(speed), size)
	}
	return fmt.Sprintf("%s Episode %s  %s",
		pterm.Cyan("↓"), episode, pterm.Yellow(speed))
}

// progressBar renders a text progress bar like [████░░░░░░].
func progressBar(done, total, width int) string {
	if total <= 0 {
		return ""
	}
	filled := int(float64(done) / float64(total) * float64(width))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return pterm.Cyan("[") + bar + pterm.Cyan("]")
}

// formatBytes returns a human-readable byte string.
func formatBytes(b int64) string {
	const (
		kiB = 1024
		miB = 1024 * kiB
		giB = 1024 * miB
	)
	switch {
	case b >= giB:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(giB))
	case b >= miB:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(miB))
	case b >= kiB:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(kiB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// isTerminal reports whether f is a terminal (character device).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

// outputDirFor extracts the directory portion of a file path.
func outputDirFor(filePath string) string {
	if idx := strings.LastIndexAny(filePath, "/\\"); idx >= 0 {
		return filePath[:idx]
	}
	return filePath
}

// allFlagsSet reports whether all of the named flags were explicitly set
// on the command line. Used to skip the interactive config prompt when
// the user has provided all overrides via flags.
func allFlagsSet(cmd *cobra.Command, names []string) bool {
	for _, name := range names {
		if !cmd.Flags().Changed(name) {
			return false
		}
	}
	return true
}
