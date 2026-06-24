package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/tomyud1/godot-mcp/cli/internal/bridge"
	"github.com/tomyud1/godot-mcp/cli/internal/wire"
)

const maxBody = 1 << 20 // 1 MiB

// Handler serves the daemon's control API on :6506.
type Handler struct {
	bridge    *bridge.Bridge
	version   string
	toolCount int

	mu           sync.Mutex
	lastActivity time.Time
	shutdown     func()
}

// NewHandler builds the control-API handler.
func NewHandler(b *bridge.Bridge, version string, toolCount int) *Handler {
	return &Handler{bridge: b, version: version, toolCount: toolCount, lastActivity: time.Now()}
}

// SetShutdown registers the callback invoked by POST /shutdown.
func (h *Handler) SetShutdown(fn func()) {
	h.mu.Lock()
	h.shutdown = fn
	h.mu.Unlock()
}

// LastActivity returns the time of the most recent request.
func (h *Handler) LastActivity() time.Time {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastActivity
}

func (h *Handler) touch() {
	h.mu.Lock()
	h.lastActivity = time.Now()
	h.mu.Unlock()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/health":
		h.touch()
		_ = json.NewEncoder(w).Encode(wire.HealthResponse{
			Server: "gdcli", Version: h.version, ToolCount: h.toolCount,
		})
	case r.Method == http.MethodPost && r.URL.Path == "/tool":
		h.touch()
		h.handleTool(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/shutdown":
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
		h.mu.Lock()
		fn := h.shutdown
		h.mu.Unlock()
		if fn != nil {
			go fn()
		}
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":"Not found"}`))
	}
}

func (h *Handler) handleTool(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBody))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"read body"}`))
		return
	}
	var req wire.ToolRequest
	if err := json.Unmarshal(body, &req); err != nil || req.Name == "" {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":"Missing or invalid \"name\" field"}`))
		return
	}
	res := h.execute(req.Name, req.Args)
	_ = json.NewEncoder(w).Encode(res)
}

func (h *Handler) execute(name string, args map[string]any) wire.ToolCallResult {
	if name == "get_godot_status" {
		st := h.bridge.Status()
		mode := "waiting"
		msg := "Godot is not connected. Open a Godot project with the MCP plugin enabled to connect."
		if st.Connected {
			mode = "live"
			msg = "Godot is connected. Tools will execute in the Godot editor."
		}
		payload := map[string]any{
			"connected":         st.Connected,
			"server_version":    h.version,
			"websocket_port":    st.Port,
			"mode":              mode,
			"runtime_connected": st.Runtime,
			"pending_requests":  st.Pending,
			"project_path":      nilIfEmpty(st.ProjectPath),
			"message":           msg,
		}
		return okResult(payload)
	}

	raw, err := h.bridge.InvokeTool(context.Background(), name, args)
	if err != nil {
		payload := map[string]any{"error": err.Error(), "tool": name, "args": args, "mode": "live"}
		var te *bridge.ToolError
		if errors.As(err, &te) && len(te.Details) > 0 {
			var d map[string]any
			if json.Unmarshal(te.Details, &d) == nil {
				delete(d, "ok")
				delete(d, "error")
				for k, v := range d {
					payload[k] = v
				}
			}
		}
		return errResult(payload)
	}
	return okResultRaw(raw)
}

func nilIfEmpty(s string) any {
	if s == "" {
		return nil
	}
	return s
}

func okResult(payload map[string]any) wire.ToolCallResult {
	txt, _ := json.MarshalIndent(payload, "", "  ")
	return wire.ToolCallResult{Content: []wire.Content{{Type: "text", Text: string(txt)}}}
}

func errResult(payload map[string]any) wire.ToolCallResult {
	txt, _ := json.MarshalIndent(payload, "", "  ")
	return wire.ToolCallResult{Content: []wire.Content{{Type: "text", Text: string(txt)}}, IsError: true}
}

func okResultRaw(raw json.RawMessage) wire.ToolCallResult {
	text := "null"
	if len(raw) > 0 {
		var v any
		if json.Unmarshal(raw, &v) == nil {
			if pretty, err := json.MarshalIndent(v, "", "  "); err == nil {
				text = string(pretty)
			}
		} else {
			text = string(raw)
		}
	}
	return wire.ToolCallResult{Content: []wire.Content{{Type: "text", Text: text}}}
}
