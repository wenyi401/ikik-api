package service

import "testing"

func TestKiroAccountDefaultModelMapping(t *testing.T) {
	account := &Account{
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{},
	}

	tests := map[string]string{
		"claude-opus-4-8":          "claude-opus-4.8",
		"claude-opus-4-8-thinking": "claude-opus-4.8",
		"claude-sonnet-4-6":        "claude-sonnet-4.6",
		"unknown-model":            "unknown-model",
	}
	for requested, want := range tests {
		if got := account.GetMappedModel(requested); got != want {
			t.Fatalf("GetMappedModel(%q) = %q, want %q", requested, got, want)
		}
	}
}

func TestKiroAccountDefaultMappingRestrictsUnsupportedModels(t *testing.T) {
	account := &Account{
		Platform:    PlatformKiro,
		Type:        AccountTypeAPIKey,
		Credentials: map[string]any{},
	}

	if !account.IsModelSupported("claude-sonnet-4-6") {
		t.Fatal("expected Kiro default model to be supported")
	}
	if account.IsModelSupported("gpt-4o") {
		t.Fatal("expected OpenAI model to be unsupported for Kiro default mapping")
	}
	if account.IsModelSupported("auto") {
		t.Fatal("expected auto to be unsupported for Kiro default mapping")
	}
}

func TestKiroAccountEndpointCapabilities(t *testing.T) {
	account := &Account{
		Platform: PlatformKiro,
		Type:     AccountTypeAPIKey,
	}

	if !account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityChatCompletions) {
		t.Fatal("expected Kiro API key account to support chat completions")
	}
	if account.SupportsOpenAIEndpointCapability(OpenAIEndpointCapabilityEmbeddings) {
		t.Fatal("expected Kiro API key account not to advertise embeddings support")
	}
}
