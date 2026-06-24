package cmd

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/tomyud1/godot-mcp/cli/internal/client"
)

func newCallCmd() *cobra.Command {
	var jsonArgs string
	cmd := &cobra.Command{
		Use:   "call <tool> [--json '{...}']",
		Short: "Invoke any tool with a raw JSON args object (escape hatch)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := strings.ReplaceAll(args[0], "-", "_")
			parsed := map[string]any{}
			if strings.TrimSpace(jsonArgs) != "" {
				if err := json.Unmarshal([]byte(jsonArgs), &parsed); err != nil {
					return fmt.Errorf("--json must be a valid JSON object: %w", err)
				}
			}
			return client.RunTool(name, parsed)
		},
	}
	cmd.Flags().StringVar(&jsonArgs, "json", "", "tool arguments as a JSON object")
	return cmd
}
