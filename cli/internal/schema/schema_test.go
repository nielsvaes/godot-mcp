package schema

import "testing"

func TestAllLoadsContract(t *testing.T) {
	tools, err := All()
	if err != nil {
		t.Fatalf("All() error: %v", err)
	}
	if len(tools) != 63 {
		t.Fatalf("expected 63 tools, got %d", len(tools))
	}
}

func TestByNameKnownAndUnknown(t *testing.T) {
	got, ok := ByName("add_node")
	if !ok {
		t.Fatal("expected add_node to exist")
	}
	if got.Name != "add_node" {
		t.Fatalf("ByName returned wrong tool: %q", got.Name)
	}
	if _, ok := ByName("does_not_exist"); ok {
		t.Fatal("expected unknown tool to be absent")
	}
}

func TestRuntimeTargetParsed(t *testing.T) {
	got, ok := ByName("take_screenshot")
	if !ok {
		t.Fatal("expected take_screenshot to exist")
	}
	if got.Target != "runtime" {
		t.Fatalf("take_screenshot target = %q, want runtime", got.Target)
	}
}
