package wire

import (
	"encoding/json"
	"testing"
)

func TestToolCallResultOmitsIsErrorWhenFalse(t *testing.T) {
	b, err := json.Marshal(ToolCallResult{Content: []Content{{Type: "text", Text: "{}"}}})
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	want := `{"content":[{"type":"text","text":"{}"}]}`
	if got != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestHealthResponseTags(t *testing.T) {
	b, err := json.Marshal(HealthResponse{Server: "gdcli", Version: "0.1.0", ToolCount: 63})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"server":"gdcli","version":"0.1.0","tool_count":63}`
	if string(b) != want {
		t.Fatalf("got %s, want %s", string(b), want)
	}
}
