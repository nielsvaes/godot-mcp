package schema

import (
	"os"
	"strings"
	"testing"
)

// repoRoot is three levels up from cli/internal/schema.
const repoRoot = "../../../"

func readFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(repoRoot + rel)
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

// Every editor tool must be routable in tool_executor.gd's _tool_map; every
// runtime tool must be dispatched in mcp_runtime.gd. This guards against the
// schema, the Node server, and the GDScript addon drifting apart.
func TestSchemaMatchesAddonHandlers(t *testing.T) {
	tools, err := All()
	if err != nil {
		t.Fatal(err)
	}
	editorMap := readFile(t, "addons/godot_mcp/tool_executor.gd")
	runtimeDispatch := readFile(t, "addons/godot_mcp/runtime/mcp_runtime.gd")

	for _, tool := range tools {
		switch tool.Target {
		case "editor":
			// Routing entries look like:  &"add_node": [_scene_tools, &"add_node"],
			if !strings.Contains(editorMap, "&\""+tool.Name+"\":") {
				t.Errorf("editor tool %q has no _tool_map entry in tool_executor.gd", tool.Name)
			}
		case "runtime":
			// Dispatch entries look like:  "take_screenshot":
			if !strings.Contains(runtimeDispatch, "\""+tool.Name+"\":") {
				t.Errorf("runtime tool %q is not dispatched in mcp_runtime.gd", tool.Name)
			}
		default:
			t.Errorf("tool %q has unexpected target %q", tool.Name, tool.Target)
		}
	}
}
