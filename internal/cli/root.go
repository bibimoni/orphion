package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/pterm/pterm"
	"github.com/spf13/cobra"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/config"
	"github.com/distiled/orphion/internal/ffmpeg"
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
		Use:   "config",
		Short: "Manage Orphion configuration",
	}
	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := configInitPath
			if path == "" {
				path = fmt.Sprintf("%s/.config/orphion/config.yaml", os.Getenv("HOME"))
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
		Use:   "search",
		Short: "Search for titles",
		Args:  cobra.MinimumNArgs(1),
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
		Use:   "download",
		Short: "Download episodes",
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

			// Apply overrides to the actual service config.
			if outputDir != "" {
				service.SetOutputDir(outputDir)
			}
			if quality != "" {
				service.SetPreferredQuality(quality)
			}
			if concurrency > 0 {
				service.SetConcurrency(concurrency)
			}
			if force {
				service.SetForce(true)
			}

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

			// Set up progress display.
			service.SetProgressCallback(downloadProgressCallback)

			result, _, err := service.DownloadEpisodes(cmd.Context(), animeID, episodes, title)
			if err != nil {
				return err
			}

			// Show output directory for completed downloads.
			if len(result.Outputs) > 0 {
				dir := outputDirFor(result.Outputs[0])
				pterm.Success.Printfln("Saved to %s", pterm.LightBlue(dir))
			}

			if result.Failed > 0 {
				pterm.Error.Printfln("%d completed, %d failed", result.Completed, result.Failed)
				return &ExitError{code: 1, msg: "some downloads failed"}
			}
			pterm.Success.Printfln("%d episode(s) downloaded", result.Completed)
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

// downloadProgressCallback displays an animated spinner with live download stats.
func downloadProgressCallback(episode string, progress ffmpeg.Progress) {
	speed := progress.Speed
	if speed == "" {
		speed = "..."
	}
	size := formatBytes(progress.Bytes)
	label := fmt.Sprintf("Episode %s", episode)

	// Write a single updating line to stderr (overwrites itself).
	pterm.Fprinto(os.Stderr, fmt.Sprintf("  %s %s  %s/s  %s",
		pterm.Cyan("↓"), pterm.Bold.Sprint(label), pterm.Yellow(speed), size))
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
	if idx := strings.LastIndexByte(filePath, '/'); idx >= 0 {
		return filePath[:idx]
	}
	return filePath
}
