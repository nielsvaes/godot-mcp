package schema

import (
	"reflect"
	"testing"
)

func sampleTool() Tool {
	return Tool{
		Name:   "demo_tool",
		Target: "editor",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"name":    {Type: "string", Description: "a name"},
				"count":   {Type: "number", Description: "a number"},
				"enabled": {Type: "boolean", Description: "a flag"},
				"event":   {Type: "object", Description: "nested"},
				"mode":    {Enum: []string{"a", "b"}, Description: "choice"},
			},
			Required: []string{"name"},
		},
	}
}

func TestBuildToolCommandName(t *testing.T) {
	cmd := BuildToolCommand(Tool{Name: "add_node"}, func(string, map[string]any) error { return nil })
	if cmd.Use != "add-node" {
		t.Fatalf("Use = %q, want add-node", cmd.Use)
	}
}

func TestBuildToolCommandSendsTypedArgs(t *testing.T) {
	var gotName string
	var gotArgs map[string]any
	cmd := BuildToolCommand(sampleTool(), func(name string, args map[string]any) error {
		gotName, gotArgs = name, args
		return nil
	})
	cmd.SetArgs([]string{
		"--name", "Player",
		"--count", "5",
		"--enabled",
		"--event", `{"type":"key","pressed":true}`,
		"--mode", "b",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if gotName != "demo_tool" {
		t.Fatalf("tool name = %q", gotName)
	}
	want := map[string]any{
		"name":    "Player",
		"count":   float64(5),
		"enabled": true,
		"event":   map[string]any{"type": "key", "pressed": true},
		"mode":    "b",
	}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Fatalf("args = %#v\nwant %#v", gotArgs, want)
	}
}

func TestBuildToolCommandOnlySendsSetFlags(t *testing.T) {
	var gotArgs map[string]any
	cmd := BuildToolCommand(sampleTool(), func(_ string, args map[string]any) error {
		gotArgs = args
		return nil
	})
	cmd.SetArgs([]string{"--name", "Only"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if len(gotArgs) != 1 || gotArgs["name"] != "Only" {
		t.Fatalf("expected only name, got %#v", gotArgs)
	}
}

func TestBuildToolCommandRequiredFlag(t *testing.T) {
	cmd := BuildToolCommand(sampleTool(), func(string, map[string]any) error { return nil })
	cmd.SetArgs([]string{"--count", "1"})
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	if err := cmd.Execute(); err == nil {
		t.Fatal("expected error for missing required --name")
	}
}
