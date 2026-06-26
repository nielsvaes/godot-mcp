package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	p := ln.Addr().(*net.TCPAddr).Port
	ln.Close()
	return p
}

// dialGodot connects as a fake Godot editor and sends godot_ready.
func dialGodot(t *testing.T, port int, role string) *websocket.Conn {
	t.Helper()
	url := fmt.Sprintf("ws://127.0.0.1:%d/", port)
	var c *websocket.Conn
	var err error
	for i := 0; i < 50; i++ {
		c, _, err = websocket.DefaultDialer.Dial(url, nil)
		if err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	ready := map[string]any{"type": "godot_ready", "project_path": "/tmp/proj"}
	if role != "" {
		ready["role"] = role
	}
	if err := c.WriteJSON(ready); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestInvokeToolRoundTrip(t *testing.T) {
	port := freePort(t)
	b := New(port, 2*time.Second)
	if err := b.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer b.Stop()

	c := dialGodot(t, port, "editor")
	defer c.Close()

	// Wait until the editor slot is registered.
	for i := 0; i < 50 && !b.Connected(); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	if !b.Connected() {
		t.Fatal("editor never connected")
	}

	// Fake Godot: read the tool_invoke and reply with a result.
	go func() {
		for {
			var m map[string]any
			if err := c.ReadJSON(&m); err != nil {
				return
			}
			if m["type"] == "tool_invoke" {
				_ = c.WriteJSON(map[string]any{
					"type":    "tool_result",
					"id":      m["id"],
					"success": true,
					"result":  map[string]any{"ok": true, "echo": m["tool"]},
				})
			}
		}
	}()

	res, err := b.InvokeTool(context.Background(), "add_node", map[string]any{"name": "P"})
	if err != nil {
		t.Fatalf("InvokeTool: %v", err)
	}
	var parsed map[string]any
	if err := json.Unmarshal(res, &parsed); err != nil {
		t.Fatalf("result not JSON: %v", err)
	}
	if parsed["echo"] != "add_node" {
		t.Fatalf("unexpected result: %v", parsed)
	}
}

func TestInvokeToolNotConnected(t *testing.T) {
	port := freePort(t)
	b := New(port, time.Second)
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	_, err := b.InvokeTool(context.Background(), "add_node", nil)
	if err == nil {
		t.Fatal("expected error when Godot not connected")
	}
}

func TestInvokeToolTimeout(t *testing.T) {
	port := freePort(t)
	b := New(port, 200*time.Millisecond)
	if err := b.Start(); err != nil {
		t.Fatal(err)
	}
	defer b.Stop()
	c := dialGodot(t, port, "editor")
	defer c.Close()
	for i := 0; i < 50 && !b.Connected(); i++ {
		time.Sleep(20 * time.Millisecond)
	}
	// Fake Godot never replies → timeout.
	_, err := b.InvokeTool(context.Background(), "add_node", nil)
	if err == nil {
		t.Fatal("expected timeout error")
	}
}
