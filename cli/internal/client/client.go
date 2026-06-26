// Package client is the thin CLI side: it ensures a bridge is reachable
// (reusing a running primary or spawning a gdcli daemon), forwards tool calls,
// and renders results with appropriate exit codes.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/tomyud1/godot-mcp/cli/internal/config"
	"github.com/tomyud1/godot-mcp/cli/internal/wire"
)

// OutputRaw, when set by the root command's --raw flag, prints compact JSON.
var OutputRaw bool

// Exit codes.
const (
	ExitUsage        = 1
	ExitToolError    = 2
	ExitNotConnected = 3
	ExitDaemon       = 4
	ExitTimeout      = 5
)

// ExitError carries a process exit code up to main.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Err.Error()
}
func (e *ExitError) Unwrap() error { return e.Err }

func baseURL() string {
	return fmt.Sprintf("http://127.0.0.1:%d", config.HTTPPort())
}

// Probe reports whether a usable bridge (gdcli daemon or Node primary) is up.
func Probe() bool {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(baseURL() + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return false
	}
	var hr wire.HealthResponse
	if json.NewDecoder(resp.Body).Decode(&hr) != nil {
		return false
	}
	return hr.Version != ""
}

// EnsureBridge guarantees a reachable bridge, spawning a daemon if needed.
func EnsureBridge() error {
	if Probe() {
		return nil
	}
	if err := spawnDaemon(); err != nil {
		return &ExitError{Code: ExitDaemon, Err: fmt.Errorf("could not start gdcli daemon: %w", err)}
	}
	for i := 0; i < 50; i++ {
		if Probe() {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return &ExitError{Code: ExitDaemon, Err: fmt.Errorf("gdcli daemon did not become healthy on :%d", config.HTTPPort())}
}

func spawnDaemon() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logPath := daemonLogPath()
	var out io.Writer = io.Discard
	if logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		out = logFile
		defer logFile.Close()
	}
	cmd := exec.Command(exe, "serve")
	cmd.Stdout = out
	cmd.Stderr = out
	cmd.SysProcAttr = detachSysProcAttr()
	return cmd.Start()
}

func daemonLogPath() string {
	dir, err := os.UserCacheDir()
	if err != nil {
		dir = os.TempDir()
	}
	d := filepath.Join(dir, "gdcli")
	_ = os.MkdirAll(d, 0o755)
	return filepath.Join(d, "daemon.log")
}

// CallTool POSTs a tool call to the bridge and returns the result.
func CallTool(name string, args map[string]any) (wire.ToolCallResult, error) {
	if args == nil {
		args = map[string]any{}
	}
	body, _ := json.Marshal(wire.ToolRequest{Name: name, Args: args})
	client := &http.Client{Timeout: config.ToolTimeout() + 5*time.Second}
	resp, err := client.Post(baseURL()+"/tool", "application/json", bytes.NewReader(body))
	if err != nil {
		return wire.ToolCallResult{}, err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(resp.Body)
	var res wire.ToolCallResult
	if err := json.Unmarshal(data, &res); err != nil {
		return wire.ToolCallResult{}, fmt.Errorf("bad response from daemon: %s", string(data))
	}
	return res, nil
}

// RunTool ensures a bridge, calls the tool, prints output, and returns an
// *ExitError on failure.
func RunTool(name string, args map[string]any) error {
	if err := EnsureBridge(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return err
	}
	res, err := CallTool(name, args)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gdcli: daemon error:", err)
		return &ExitError{Code: ExitDaemon, Err: err}
	}
	text := ""
	if len(res.Content) > 0 {
		text = res.Content[0].Text
	}
	if res.IsError {
		fmt.Fprintln(os.Stderr, render(text))
		return &ExitError{Code: classifyError(text)}
	}
	fmt.Fprintln(os.Stdout, render(text))
	return nil
}

// render compacts JSON when --raw is set; otherwise returns it as-is.
func render(text string) string {
	if !OutputRaw {
		return text
	}
	var v any
	if json.Unmarshal([]byte(text), &v) != nil {
		return text
	}
	b, err := json.Marshal(v)
	if err != nil {
		return text
	}
	return string(b)
}

// classifyError maps an error payload's message to an exit code.
func classifyError(text string) int {
	msg := text
	var payload struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(text), &payload) == nil && payload.Error != "" {
		msg = payload.Error
	}
	lower := strings.ToLower(msg)
	switch {
	case strings.Contains(lower, "not connected"):
		return ExitNotConnected
	case strings.Contains(lower, "timed out"):
		return ExitTimeout
	default:
		return ExitToolError
	}
}
