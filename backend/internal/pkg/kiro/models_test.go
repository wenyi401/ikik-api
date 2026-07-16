package kiro

import "testing"

func TestDefaultModelMappingUsesKiroUpstreamNames(t *testing.T) {
	mapping := DefaultModelMapping()

	tests := map[string]string{
		"claude-opus-4-8":                    "claude-opus-4.8",
		"claude-opus-4-8-thinking":           "claude-opus-4.8",
		"claude-sonnet-4-6":                  "claude-sonnet-4.6",
		"claude-haiku-4-5-20251001-thinking": "claude-haiku-4.5",
	}
	for from, want := range tests {
		if got := mapping[from]; got != want {
			t.Fatalf("DefaultModelMapping()[%q] = %q, want %q", from, got, want)
		}
	}
}

func TestDefaultModelMappingReturnsCopy(t *testing.T) {
	first := DefaultModelMapping()
	first["claude-opus-4-8"] = "changed"

	second := DefaultModelMapping()
	if got := second["claude-opus-4-8"]; got != "claude-opus-4.8" {
		t.Fatalf("DefaultModelMapping returned shared map, got %q", got)
	}
}
