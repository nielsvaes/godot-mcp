package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/tomyud1/godot-mcp/cli/internal/bridge"
	"github.com/tomyud1/godot-mcp/cli/internal/wire"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, _ := net.Listen("tcp", "127.0.0.1:0")
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return p
}

// A tool call posted to the HTTP API must reach a connected fake Godot over
// the WS bridge and return its result.
func TestEndToEndToolCall(t *testing.T) {
	wsPort := freePort(t)
	b := bridge.New(wsPort, 3*time.Second)
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()

	h := NewHandler(b, "0.1.0", 63)
	api := httptest.NewServer(h)
	defer api.Close()

	// Fake Godot connects and echoes tool calls.
	url := fmt.Sprintf("ws://127.0.0.1:%d/", wsPort)
	var gc *websocket.Conn
	var err error
	for i := 0; i < 50; i++ {
		gc, _, err = websocket.DefaultDialer.Dial(url, nil)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer gc.Close()
	gc.WriteJSON(map[string]any{"type": "godot_ready", "project_path": "/tmp/p"})
	go func() {
		for {
			var m map[string]any
			if gc.ReadJSON(&m) != nil {
				return
			}
			if m["type"] == "tool_invoke" {
				gc.WriteJSON(map[string]any{
					"type":    "tool_result",
					"id":      m["id"],
					"success": true,
					"result":  map[string]any{"ok": true, "created": m["tool"]},
				})
			}
		}
	}()
	for i := 0; i < 50 && !b.Connected(); i++ {
		time.Sleep(20 * time.Millisecond)
	}

	body, _ := json.Marshal(wire.ToolRequest{Name: "add_node", Args: map[string]any{"name": "P"}})
	resp, err := http.Post(api.URL+"/tool", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var res wire.ToolCallResult
	json.NewDecoder(resp.Body).Decode(&res)
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if !strings.Contains(res.Content[0].Text, "add_node") {
		t.Fatalf("result did not echo tool: %s", res.Content[0].Text)
	}
}
