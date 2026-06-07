package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/distiled/orphion/internal/config"
	"github.com/spf13/cobra"
)

// configDefaultPath is the default configuration file path.
const configDefaultPath = "~/.config/orphion/config.yaml"

func newConfigCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "config",
		Short: "Manage Orphion configuration",
	}
	root.AddCommand(&cobra.Command{
		Use:   "init",
		Short: "Create default configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			path := os.ExpandEnv(strings.ReplaceAll(configDefaultPath, "~", "$HOME"))
			if err := config.Init(path); err != nil {
				return fmt.Errorf("config init: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "configuration written to %s\n", path)
			return nil
		},
	})
	return root
}