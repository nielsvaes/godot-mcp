package bridge

import "testing"

func TestRouteIsRuntime(t *testing.T) {
	cases := []struct {
		tool string
		args map[string]any
		want bool
	}{
		{"take_screenshot", nil, true},
		{"send_input", nil, true},
		{"query_runtime_node", nil, true},
		{"get_runtime_log", nil, true},
		{"add_node", nil, false},
		{"list_signal_connections", map[string]any{"source": "runtime"}, true},
		{"list_signal_connections", map[string]any{"source": "scene_file"}, false},
		{"list_signal_connections", nil, false},
	}
	for _, c := range cases {
		if got := RouteIsRuntime(c.tool, c.args); got != c.want {
			t.Errorf("RouteIsRuntime(%q,%v) = %v, want %v", c.tool, c.args, got, c.want)
		}
	}
}

func TestToolErrorMessage(t *testing.T) {
	e := &ToolError{Message: "boom"}
	if e.Error() != "boom" {
		t.Fatalf("Error() = %q", e.Error())
	}
}
