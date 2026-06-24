package cmd

import (
	"strings"
	"testing"
)

func TestRootHasBuiltinsAndToolCommands(t *testing.T) {
	root := NewRootCmd()
	names := map[string]bool{}
	for _, c := range root.Commands() {
		names[c.Name()] = true
	}
	for _, want := range []string{"serve", "stop", "tools", "describe", "status", "call", "add-node", "list-dir"} {
		if !names[want] {
			t.Errorf("missing subcommand %q", want)
		}
	}
}

func TestToolsCommandLists(t *testing.T) {
	root := NewRootCmd()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"tools"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "add_node") {
		t.Fatalf("tools output missing add_node:\n%s", out.String())
	}
}

func TestDescribeShowsFlags(t *testing.T) {
	root := NewRootCmd()
	var out strings.Builder
	root.SetOut(&out)
	root.SetArgs([]string{"describe", "add-node"})
	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "add_node") {
		t.Fatalf("describe output missing tool name:\n%s", out.String())
	}
}
