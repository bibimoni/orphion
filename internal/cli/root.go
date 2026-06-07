package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Version is set at build time.
var Version = "dev"

// New creates the root command for Orphion.
func New() *cobra.Command {
	root := &cobra.Command{
		Use:   "orphion",
		Short: "Download anime and drama episodes",
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Fprintf(cmd.OutOrStdout(), "orphion version %s\n", Version)
			return nil
		},
	}
	root.AddCommand(versionCmd)

	return root
}