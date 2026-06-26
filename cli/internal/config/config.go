// Package config centralises ports, timeouts, and version for gdcli.
// Defaults and env-var names match the Node godot-mcp-server exactly.
package config

import (
	"os"
	"strconv"
	"time"
)

// Version is reported by the daemon's /health endpoint.
const Version = "0.1.0"

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

// WebSocketPort is the port Godot dials into (GODOT_MCP_PORT, default 6505).
func WebSocketPort() int { return envInt("GODOT_MCP_PORT", 6505) }

// HTTPPort is the control API port (GODOT_MCP_HTTP_PORT, default 6506).
func HTTPPort() int { return envInt("GODOT_MCP_HTTP_PORT", 6506) }

// ToolTimeout bounds a single tool round-trip (GODOT_MCP_TIMEOUT_MS, default 30000).
func ToolTimeout() time.Duration {
	return time.Duration(envInt("GODOT_MCP_TIMEOUT_MS", 30000)) * time.Millisecond
}

// IdleTimeout is how long the daemon stays up with no Godot and no HTTP
// activity (GODOT_MCP_IDLE_TIMEOUT_MS, default 30000).
func IdleTimeout() time.Duration {
	return time.Duration(envInt("GODOT_MCP_IDLE_TIMEOUT_MS", 30000)) * time.Millisecond
}
