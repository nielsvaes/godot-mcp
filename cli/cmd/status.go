package cmd

import (
	"github.com/spf13/cobra"

	"github.com/tomyud1/godot-mcp/cli/internal/client"
)

func newStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether Godot is connected to the bridge",
		RunE: func(*cobra.Command, []string) error {
			return client.RunTool("get_godot_status", nil)
		},
	}
}
