package bridge

import "encoding/json"

// runtimeOnlyTools always route to the in-game runtime helper.
var runtimeOnlyTools = map[string]bool{
	"take_screenshot":    true,
	"send_input":         true,
	"query_runtime_node": true,
	"get_runtime_log":    true,
}

// RouteIsRuntime reports whether a tool call should go to the runtime helper
// rather than the editor. Mirrors GodotBridge.routeIsRuntime in the Node server.
func RouteIsRuntime(tool string, args map[string]any) bool {
	if runtimeOnlyTools[tool] {
		return true
	}
	if tool == "list_signal_connections" {
		if s, ok := args["source"].(string); ok && s == "runtime" {
			return true
		}
	}
	return false
}

// inbound is a superset of every message the bridge receives from Godot.
type inbound struct {
	Type        string          `json:"type"`
	ID          string          `json:"id"`
	Success     bool            `json:"success"`
	Result      json.RawMessage `json:"result"`
	Error       string          `json:"error"`
	ProjectPath string          `json:"project_path"`
	Role        string          `json:"role"`
}

// toolInvoke is the message the bridge sends to request a tool call.
type toolInvoke struct {
	Type string         `json:"type"`
	ID   string         `json:"id"`
	Tool string         `json:"tool"`
	Args map[string]any `json:"args"`
}

// ToolError carries a failed tool result, preserving the structured details
// dict (open_in_editor, where, clamped, …) the tool shipped back.
type ToolError struct {
	Message string
	Details json.RawMessage
}

func (e *ToolError) Error() string { return e.Message }
