// Package cmd wires the gdcli cobra command tree.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/tomyud1/godot-mcp/cli/internal/client"
	"github.com/tomyud1/godot-mcp/cli/internal/schema"
)

// NewRootCmd builds the full command tree: builtins + one command per tool.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "gdcli",
		Short:         "Drive a live Godot editor from the shell",
		Long:          "gdcli exposes every Godot MCP tool as a scriptable command, backed by an auto-managed daemon that bridges to the Godot editor.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVarP(&client.OutputRaw, "raw", "r", false, "print compact (unindented) JSON")

	root.AddCommand(newServeCmd(), newStopCmd(), newToolsCmd(), newDescribeCmd(), newStatusCmd(), newCallCmd())

	if tools, err := schema.All(); err == nil {
		for _, t := range tools {
			root.AddCommand(schema.BuildToolCommand(t, client.RunTool))
		}
	}
	return root
}

// Execute runs the CLI and returns a process exit code.
func Execute() int {
	if err := NewRootCmd().Execute(); err != nil {
		var ee *client.ExitError
		if errors.As(err, &ee) {
			if ee.Err != nil {
				// RunTool already printed user-facing output; nothing to add.
			}
			return ee.Code
		}
		fmt.Fprintln(os.Stderr, "Error:", err)
		return client.ExitUsage
	}
	return 0
}
