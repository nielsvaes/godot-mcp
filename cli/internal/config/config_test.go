package config

import (
	"os"
	"testing"
	"time"
)

func TestDefaults(t *testing.T) {
	os.Unsetenv("GODOT_MCP_PORT")
	os.Unsetenv("GODOT_MCP_HTTP_PORT")
	os.Unsetenv("GODOT_MCP_TIMEOUT_MS")
	if WebSocketPort() != 6505 {
		t.Fatalf("WebSocketPort default = %d, want 6505", WebSocketPort())
	}
	if HTTPPort() != 6506 {
		t.Fatalf("HTTPPort default = %d, want 6506", HTTPPort())
	}
	if ToolTimeout() != 30000*time.Millisecond {
		t.Fatalf("ToolTimeout default = %v", ToolTimeout())
	}
}

func TestEnvOverride(t *testing.T) {
	os.Setenv("GODOT_MCP_PORT", "7000")
	defer os.Unsetenv("GODOT_MCP_PORT")
	if WebSocketPort() != 7000 {
		t.Fatalf("WebSocketPort override = %d, want 7000", WebSocketPort())
	}
}
