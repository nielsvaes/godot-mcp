// Package wire defines the HTTP DTOs shared by the daemon and the client.
// Field tags match the Node server's control API byte-for-byte.
package wire

// ToolRequest is the POST /tool body.
type ToolRequest struct {
	Name string         `json:"name"`
	Args map[string]any `json:"args"`
}

// Content is one item of a ToolCallResult.
type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ToolCallResult is the POST /tool response (and the Node server's shape).
type ToolCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// HealthResponse is the GET /health body.
type HealthResponse struct {
	Server    string `json:"server"`
	Version   string `json:"version"`
	ToolCount int    `json:"tool_count"`
}
