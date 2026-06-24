package client

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/tomyud1/godot-mcp/cli/internal/wire"
)

// startFakeDaemon points config's HTTP port at a test server by overriding the
// env var, and returns a cleanup func.
func startFakeDaemon(t *testing.T, h http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(h)
	// httptest binds an arbitrary port on 127.0.0.1; extract it.
	addr := strings.TrimPrefix(srv.URL, "http://")
	port := addr[strings.LastIndex(addr, ":")+1:]
	old := os.Getenv("GODOT_MCP_HTTP_PORT")
	os.Setenv("GODOT_MCP_HTTP_PORT", port)
	return func() {
		srv.Close()
		os.Setenv("GODOT_MCP_HTTP_PORT", old)
	}
}

func TestProbeTrueFalse(t *testing.T) {
	if Probe() {
		// Best-effort: a stray daemon could exist; not a hard failure.
		t.Skip("a bridge is already running on the configured port")
	}
	cleanup := startFakeDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health" {
			json.NewEncoder(w).Encode(wire.HealthResponse{Server: "gdcli", Version: "0.1.0", ToolCount: 63})
			return
		}
		w.WriteHeader(404)
	})
	defer cleanup()
	if !Probe() {
		t.Fatal("Probe should be true against fake daemon")
	}
}

func TestCallTool(t *testing.T) {
	cleanup := startFakeDaemon(t, func(w http.ResponseWriter, r *http.Request) {
		var req wire.ToolRequest
		json.NewDecoder(r.Body).Decode(&req)
		json.NewEncoder(w).Encode(wire.ToolCallResult{
			Content: []wire.Content{{Type: "text", Text: `{"ok":true,"tool":"` + req.Name + `"}`}},
		})
	})
	defer cleanup()
	res, err := CallTool("add_node", map[string]any{"name": "P"})
	if err != nil {
		t.Fatal(err)
	}
	if res.IsError || !strings.Contains(res.Content[0].Text, "add_node") {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestExitErrorClassification(t *testing.T) {
	if code := classifyError(`{"error":"Godot is not connected"}`); code != ExitNotConnected {
		t.Fatalf("not-connected code = %d", code)
	}
	if code := classifyError(`{"error":"Tool add_node timed out after 30000ms (editor)"}`); code != ExitTimeout {
		t.Fatalf("timeout code = %d", code)
	}
	if code := classifyError(`{"error":"something else"}`); code != ExitToolError {
		t.Fatalf("default code = %d", code)
	}
}
