package daemon

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/tomyud1/godot-mcp/cli/internal/bridge"
	"github.com/tomyud1/godot-mcp/cli/internal/wire"
)

func TestHealth(t *testing.T) {
	b := bridge.New(0, time.Second)
	h := NewHandler(b, "0.1.0", 63)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var hr wire.HealthResponse
	json.Unmarshal(rec.Body.Bytes(), &hr)
	if hr.Server != "gdcli" || hr.Version != "0.1.0" || hr.ToolCount != 63 {
		t.Fatalf("health = %+v", hr)
	}
}

func TestToolMissingName(t *testing.T) {
	b := bridge.New(0, time.Second)
	h := NewHandler(b, "0.1.0", 63)
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(map[string]any{"args": map[string]any{}})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tool", bytes.NewReader(body)))
	if rec.Code != 400 {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestToolGodotStatusLocal(t *testing.T) {
	b := bridge.New(0, time.Second)
	h := NewHandler(b, "0.1.0", 63)
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(wire.ToolRequest{Name: "get_godot_status"})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tool", bytes.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var res wire.ToolCallResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	if res.IsError || len(res.Content) == 0 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestToolNotConnectedIsError(t *testing.T) {
	b := bridge.New(0, time.Second)
	h := NewHandler(b, "0.1.0", 63)
	rec := httptest.NewRecorder()
	body, _ := json.Marshal(wire.ToolRequest{Name: "add_node", Args: map[string]any{"name": "P"}})
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/tool", bytes.NewReader(body)))
	if rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	var res wire.ToolCallResult
	json.Unmarshal(rec.Body.Bytes(), &res)
	if !res.IsError {
		t.Fatal("expected isError when Godot not connected")
	}
}

func TestLastActivityAdvancesOnRequest(t *testing.T) {
	b := bridge.New(0, time.Second)
	h := NewHandler(b, "0.1.0", 63)
	before := h.LastActivity()
	time.Sleep(5 * time.Millisecond)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if !h.LastActivity().After(before) {
		t.Fatalf("LastActivity did not advance: before=%v after=%v", before, h.LastActivity())
	}
}
