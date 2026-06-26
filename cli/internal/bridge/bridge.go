package bridge

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const pingInterval = 10 * time.Second

var upgrader = websocket.Upgrader{
	CheckOrigin: func(*http.Request) bool { return true },
}

// conn wraps a Godot websocket connection with a write mutex (gorilla allows
// only one concurrent writer).
type conn struct {
	ws          *websocket.Conn
	mu          sync.Mutex
	projectPath string
}

func (c *conn) writeJSON(v any) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.ws.WriteJSON(v)
}

type pendingReq struct {
	ch     chan toolResult
	target string
}

type toolResult struct {
	result json.RawMessage
	err    error
}

// Status is a snapshot of the bridge state.
type Status struct {
	Connected   bool
	Port        int
	ProjectPath string
	Runtime     bool
	Pending     int
}

// Bridge hosts the WebSocket server Godot dials into.
type Bridge struct {
	port    int
	timeout time.Duration

	httpSrv  *http.Server
	listener net.Listener

	mu        sync.Mutex
	editor    *conn
	runtime   *conn
	pending   map[string]pendingReq
	listening bool

	stopPing chan struct{}
}

// New creates a Bridge bound (on Start) to 127.0.0.1:port.
func New(port int, timeout time.Duration) *Bridge {
	return &Bridge{
		port:     port,
		timeout:  timeout,
		pending:  make(map[string]pendingReq),
		stopPing: make(chan struct{}),
	}
}

// Start binds the listener (synchronously, so bind errors surface) and serves
// in the background.
func (b *Bridge) Start() error {
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", b.port))
	if err != nil {
		return err
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", b.handleWS)
	b.listener = ln
	b.httpSrv = &http.Server{Handler: mux}
	b.listening = true
	go b.pingLoop()
	go func() { _ = b.httpSrv.Serve(ln) }()
	return nil
}

// Stop closes the server and fails all pending requests.
func (b *Bridge) Stop() {
	b.mu.Lock()
	if !b.listening {
		b.mu.Unlock()
		return
	}
	b.listening = false
	close(b.stopPing)
	for id, p := range b.pending {
		p.ch <- toolResult{err: errors.New("server shutting down")}
		delete(b.pending, id)
	}
	e, r := b.editor, b.runtime
	b.editor, b.runtime = nil, nil
	b.mu.Unlock()

	if e != nil {
		_ = e.ws.Close()
	}
	if r != nil {
		_ = r.ws.Close()
	}
	if b.httpSrv != nil {
		_ = b.httpSrv.Close()
	}
}

func (b *Bridge) IsListening() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.listening
}

func (b *Bridge) handleWS(w http.ResponseWriter, r *http.Request) {
	ws, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	b.readLoop(&conn{ws: ws})
}

func (b *Bridge) readLoop(c *conn) {
	assigned := "" // "", "editor", or "runtime"

	b.mu.Lock()
	if b.editor == nil {
		b.editor = c
		assigned = "editor"
	}
	b.mu.Unlock()

	defer func() {
		var notifyEditor *conn
		b.mu.Lock()
		if assigned == "editor" && b.editor == c {
			b.editor = nil
			b.failPendingLocked("editor", errors.New("Godot disconnected"))
		} else if assigned == "runtime" && b.runtime == c {
			b.runtime = nil
			b.failPendingLocked("runtime", errors.New("Godot runtime disconnected"))
			notifyEditor = b.editor
		}
		b.mu.Unlock()
		if notifyEditor != nil {
			_ = notifyEditor.writeJSON(map[string]any{"type": "runtime_status", "connected": false})
		}
		_ = c.ws.Close()
	}()

	for {
		_, data, err := c.ws.ReadMessage()
		if err != nil {
			return
		}
		var m inbound
		if json.Unmarshal(data, &m) != nil {
			continue
		}
		switch m.Type {
		case "godot_ready":
			if a := b.handleReady(c, assigned, m); a == "closed" {
				return
			} else {
				assigned = a
			}
		case "tool_result":
			b.handleResult(m)
		case "pong":
			// keepalive ack
		}
	}
}

func (b *Bridge) handleReady(c *conn, assigned string, m inbound) string {
	desired := "editor"
	if m.Role == "runtime" {
		desired = "runtime"
	}

	var notifyEditor *conn
	result := assigned

	b.mu.Lock()
	if desired == "runtime" {
		if b.runtime != nil && b.runtime != c {
			b.mu.Unlock()
			_ = c.ws.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(4001, "Another Godot runtime is already connected"),
				time.Now().Add(time.Second))
			return "closed"
		}
		if assigned == "editor" && b.editor == c {
			b.editor = nil
		}
		c.projectPath = m.ProjectPath
		b.runtime = c
		notifyEditor = b.editor
		result = "runtime"
	} else {
		if b.editor != nil && b.editor != c {
			b.mu.Unlock()
			_ = c.ws.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(4000, "Another Godot editor is already connected"),
				time.Now().Add(time.Second))
			return "closed"
		}
		if b.editor == nil {
			b.editor = c
		}
		c.projectPath = m.ProjectPath
		notifyEditor = b.editor
		if assigned == "" {
			result = "editor"
		}
	}
	runtimeConnected := b.runtime != nil
	b.mu.Unlock()

	if notifyEditor != nil {
		_ = notifyEditor.writeJSON(map[string]any{"type": "runtime_status", "connected": runtimeConnected})
	}
	return result
}

func (b *Bridge) handleResult(m inbound) {
	b.mu.Lock()
	p, ok := b.pending[m.ID]
	if ok {
		delete(b.pending, m.ID)
	}
	b.mu.Unlock()
	if !ok {
		return
	}
	if m.Success {
		p.ch <- toolResult{result: m.Result}
		return
	}
	msg := m.Error
	if msg == "" {
		msg = "Tool execution failed"
	}
	p.ch <- toolResult{err: &ToolError{Message: msg, Details: m.Result}}
}

func (b *Bridge) failPendingLocked(target string, err error) {
	for id, p := range b.pending {
		if p.target == target {
			p.ch <- toolResult{err: err}
			delete(b.pending, id)
		}
	}
}

func (b *Bridge) pingLoop() {
	t := time.NewTicker(pingInterval)
	defer t.Stop()
	for {
		select {
		case <-b.stopPing:
			return
		case <-t.C:
			b.mu.Lock()
			e, r := b.editor, b.runtime
			b.mu.Unlock()
			if e != nil {
				_ = e.writeJSON(map[string]string{"type": "ping"})
			}
			if r != nil {
				_ = r.writeJSON(map[string]string{"type": "ping"})
			}
		}
	}
}

// InvokeTool routes a call to the editor or runtime slot and waits for the result.
func (b *Bridge) InvokeTool(ctx context.Context, tool string, args map[string]any) (json.RawMessage, error) {
	target := "editor"
	if RouteIsRuntime(tool, args) {
		target = "runtime"
	}
	if args == nil {
		args = map[string]any{}
	}

	b.mu.Lock()
	var c *conn
	if target == "editor" {
		c = b.editor
	} else {
		c = b.runtime
	}
	if c == nil {
		b.mu.Unlock()
		if target == "runtime" {
			return nil, fmt.Errorf("Runtime helper is not connected. Tool '%s' requires the game running with the MCPRuntime autoload. Call run_scene with wait_for_runtime=true.", tool)
		}
		return nil, errors.New("Godot is not connected")
	}
	id := newID()
	ch := make(chan toolResult, 1)
	b.pending[id] = pendingReq{ch: ch, target: target}
	b.mu.Unlock()

	if err := c.writeJSON(toolInvoke{Type: "tool_invoke", ID: id, Tool: tool, Args: args}); err != nil {
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, fmt.Errorf("failed to send tool to Godot: %w", err)
	}

	select {
	case res := <-ch:
		return res.result, res.err
	case <-time.After(b.timeout):
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, fmt.Errorf("Tool %s timed out after %dms (%s)", tool, b.timeout.Milliseconds(), target)
	case <-ctx.Done():
		b.mu.Lock()
		delete(b.pending, id)
		b.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Status returns a snapshot of the bridge state.
func (b *Bridge) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()
	s := Status{Port: b.port, Pending: len(b.pending)}
	if b.editor != nil {
		s.Connected = true
		s.ProjectPath = b.editor.projectPath
	}
	if b.runtime != nil {
		s.Runtime = true
		if s.ProjectPath == "" {
			s.ProjectPath = b.runtime.projectPath
		}
	}
	return s
}

// Connected reports whether the editor connection is live.
func (b *Bridge) Connected() bool { return b.Status().Connected }

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		log.Printf("gdcli: rand: %v", err)
	}
	return hex.EncodeToString(buf)
}
