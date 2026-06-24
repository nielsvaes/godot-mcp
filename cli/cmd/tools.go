package cmd

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/tomyud1/godot-mcp/cli/internal/schema"
)

func newToolsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tools",
		Short: "List all available tools",
		RunE: func(c *cobra.Command, _ []string) error {
			tools, err := schema.All()
			if err != nil {
				return err
			}
			sort.Slice(tools, func(i, j int) bool {
				if tools[i].Category != tools[j].Category {
					return tools[i].Category < tools[j].Category
				}
				return tools[i].Name < tools[j].Name
			})
			tw := tabwriter.NewWriter(c.OutOrStdout(), 0, 0, 2, ' ', 0)
			for _, t := range tools {
				fmt.Fprintf(tw, "%s\t%s\t%s\n", t.Name, t.Category, firstLineOf(t.Description))
			}
			return tw.Flush()
		},
	}
}

func newDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe <tool>",
		Short: "Show a tool's description and flags",
		Args:  cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			name := strings.ReplaceAll(args[0], "-", "_")
			t, ok := schema.ByName(name)
			if !ok {
				return fmt.Errorf("unknown tool: %s", args[0])
			}
			out := c.OutOrStdout()
			fmt.Fprintf(out, "%s (%s, target=%s)\n\n%s\n\n", t.Name, t.Category, t.Target, t.Description)
			if len(t.InputSchema.Properties) == 0 {
				fmt.Fprintln(out, "(no arguments)")
				return nil
			}
			required := map[string]bool{}
			for _, r := range t.InputSchema.Required {
				required[r] = true
			}
			tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "FLAG\tTYPE\tREQUIRED\tDESCRIPTION")
			for propName, prop := range t.InputSchema.Properties {
				typ := prop.Type
				if len(prop.Enum) > 0 {
					typ = "enum(" + strings.Join(prop.Enum, "|") + ")"
				}
				if typ == "" {
					typ = "json"
				}
				fmt.Fprintf(tw, "--%s\t%s\t%v\t%s\n", propName, typ, required[propName], firstLineOf(prop.Description))
			}
			return tw.Flush()
		},
	}
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
