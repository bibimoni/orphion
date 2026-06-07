package cli

import (
	"fmt"
	"os"

	"github.com/distiled/orphion/internal/app"
	"github.com/distiled/orphion/internal/config"
	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "dev"

// New creates the root command for Orphion.
func New(service *app.Service) *cobra.Command {
	root := &cobra.Command{
		Use:   "orphion",
		Short: "Download episodes",
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
			path := fmt.Sprintf("%s/.config/orphion/config.yaml", os.Getenv("HOME"))
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
			result, err := service.Search(cmd.Context(), args[0], resType)
			if err != nil {
				return err
			}
			for _, a := range result.Anime {
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\n", a.ID, a.Title)
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
	cmd.Flags().StringVar(&animeID, "title-id", "", "Anime ID")
	cmd.Flags().StringVar(&resType, "type", "", "Content type: anime, drama, or empty for both")
	cmd.Flags().StringVar(&quality, "quality", "", "Preferred quality (e.g. 1080p)")
	cmd.Flags().StringVar(&outputDir, "output", "", "Output directory")
	cmd.Flags().IntVar(&concurrency, "concurrency", 0, "Download concurrency (1-4)")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite existing files")

	return cmd
}