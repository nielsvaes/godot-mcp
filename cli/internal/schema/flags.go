package schema

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// BuildToolCommand creates a cobra command for one tool, generating flags from
// its inputSchema. run is invoked with the tool's real (snake_case) name and
// the args the user supplied.
func BuildToolCommand(t Tool, run func(toolName string, args map[string]any) error) *cobra.Command {
	use := strings.ReplaceAll(t.Name, "_", "-")
	cmd := &cobra.Command{
		Use:           use,
		Short:         firstLine(t.Description),
		Long:          t.Description,
		SilenceUsage:  true,
		SilenceErrors: false,
	}

	jsonFlags := map[string]bool{}
	for propName, prop := range t.InputSchema.Properties {
		help := prop.Description
		switch {
		case len(prop.Enum) > 0:
			help = fmt.Sprintf("%s (one of: %s)", help, strings.Join(prop.Enum, ", "))
			cmd.Flags().String(propName, "", help)
		case prop.Type == "string":
			cmd.Flags().String(propName, "", help)
		case prop.Type == "number" || prop.Type == "integer":
			cmd.Flags().Float64(propName, 0, help)
		case prop.Type == "boolean":
			cmd.Flags().Bool(propName, false, help)
		default: // array, object, or untyped → JSON value
			cmd.Flags().String(propName, "", strings.TrimSpace(help)+" (JSON)")
			jsonFlags[propName] = true
		}
	}
	for _, req := range t.InputSchema.Required {
		_ = cmd.MarkFlagRequired(req)
	}

	cmd.RunE = func(c *cobra.Command, _ []string) error {
		args := map[string]any{}
		var ferr error
		c.Flags().Visit(func(f *pflag.Flag) {
			if ferr != nil {
				return
			}
			name := f.Name
			if jsonFlags[name] {
				var v any
				if err := json.Unmarshal([]byte(f.Value.String()), &v); err != nil {
					ferr = fmt.Errorf("flag --%s must be valid JSON: %w", name, err)
					return
				}
				args[name] = v
				return
			}
			prop := t.InputSchema.Properties[name]
			switch {
			case len(prop.Enum) > 0 || prop.Type == "string":
				args[name], _ = c.Flags().GetString(name)
			case prop.Type == "number" || prop.Type == "integer":
				args[name], _ = c.Flags().GetFloat64(name)
			case prop.Type == "boolean":
				args[name], _ = c.Flags().GetBool(name)
			default:
				args[name], _ = c.Flags().GetString(name)
			}
		})
		if ferr != nil {
			return ferr
		}
		return run(t.Name, args)
	}
	return cmd
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
