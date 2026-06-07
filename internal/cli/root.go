package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/config"
	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "dev"

// configInitPath is the path used by "config init". Overridden by main
// when the config path is resolved before service initialization.
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
		Long: `Orphion searches a catalog provider for content and downloads
selected episodes as MKV files through system FFmpeg.

Run without arguments to start interactive mode.`,
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
			fmt.Fprintf(cmd.OutOrStdout(), "orphion version %s\n", Version)
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
			return config.Init(path)
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
			result, err := service.Search(cmd.Context(), args[0], resType)
			if err != nil {
				return err
			}
			for _, a := range result.Anime {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", a.ID, a.Title)
			}
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
			// Trim whitespace from all string flags to guard against
			// copy-paste or shell expansion artifacts.
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

			id := animeID
			if id == "" {
				var err error
				id, err = service.ResolveID(cmd.Context(), title, resType)
				if err != nil {
					return err
				}
			}

			result, _, err := service.DownloadEpisodes(cmd.Context(), id, episodes, title)
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "%d completed, %d failed\n", result.Completed, result.Failed)
			if result.Failed > 0 {
				return &ExitError{code: 1, msg: "some downloads failed"}
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
