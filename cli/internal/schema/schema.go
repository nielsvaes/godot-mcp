// Package schema parses the embedded tool contract into typed structs.
package schema

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tomyud1/godot-mcp/schemas"
)

// Property is one entry under an InputSchema's properties map.
type Property struct {
	Type        string              `json:"type,omitempty"`
	Description string              `json:"description,omitempty"`
	Enum        []string            `json:"enum,omitempty"`
	Items       *Property           `json:"items,omitempty"`
	Properties  map[string]Property `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
}

// InputSchema is a tool's JSON-Schema object.
type InputSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Tool is one entry in the shared contract.
type Tool struct {
	Name        string      `json:"name"`
	Category    string      `json:"category"`
	Target      string      `json:"target"`
	Description string      `json:"description"`
	InputSchema InputSchema `json:"inputSchema"`
}

var (
	loadOnce sync.Once
	loaded   []Tool
	loadErr  error
)

// All returns every tool in the embedded contract.
func All() ([]Tool, error) {
	loadOnce.Do(func() {
		if err := json.Unmarshal(schemas.ToolsJSON, &loaded); err != nil {
			loadErr = fmt.Errorf("parse embedded tools.json: %w", err)
		}
	})
	return loaded, loadErr
}

// ByName returns the tool with the given name and whether it was found.
func ByName(name string) (Tool, bool) {
	tools, err := All()
	if err != nil {
		return Tool{}, false
	}
	for _, t := range tools {
		if t.Name == name {
			return t, true
		}
	}
	return Tool{}, false
}
