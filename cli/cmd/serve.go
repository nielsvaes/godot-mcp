package cmd

import (
	"github.com/spf13/cobra"

	"github.com/tomyud1/godot-mcp/cli/internal/daemon"
)

func newServeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the gdcli daemon (hosts the Godot bridge); normally auto-started",
		RunE: func(*cobra.Command, []string) error {
			return daemon.Serve()
		},
	}
}
