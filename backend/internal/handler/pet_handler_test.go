package handler

import (
	"strings"
	"testing"

	"ikik-api/internal/service"
)

func TestShouldTrackPetActivity(t *testing.T) {
	tests := []struct {
		name   string
		method string
		path   string
		want   bool
	}{
		{name: "chat generation", method: "POST", path: "/chat/completions", want: true},
		{name: "image edit", method: "POST", path: "/v1/images/edits", want: true},
		{name: "gemini generation", method: "POST", path: "/v1beta/models/gemini-3:generateContent", want: true},
		{name: "responses websocket", method: "GET", path: "/responses", want: true},
		{name: "model list", method: "GET", path: "/v1/models", want: false},
		{name: "usage", method: "GET", path: "/v1/usage", want: false},
		{name: "count tokens", method: "POST", path: "/v1/messages/count_tokens", want: false},
		{name: "async status", method: "GET", path: "/v1/images/tasks/task-1", want: false},
		{name: "batch cancel", method: "POST", path: "/v1/images/batches/batch-1/cancel", want: false},
		{name: "voice metadata edit", method: "PATCH", path: "/custom-voices/voice-1", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldTrackPetActivity(tt.method, tt.path); got != tt.want {
				t.Fatalf("shouldTrackPetActivity(%q, %q) = %v, want %v", tt.method, tt.path, got, tt.want)
			}
		})
	}
}

func TestPlaygroundTextModelsFiltersMediaAndWildcards(t *testing.T) {
	got := playgroundTextModels([]string{
		"gpt-5.6-sol", "gpt-image-1", "text-embedding-3-large", "claude-*", "gpt-5.6-sol",
	})
	if len(got) != 1 || got[0] != "gpt-5.6-sol" {
		t.Fatalf("unexpected assistant models: %#v", got)
	}
}

func TestPetCompletionContent(t *testing.T) {
	content, err := petCompletionContent([]byte(`{"choices":[{"message":{"content":"请先刷新账号状态。"}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if content != "请先刷新账号状态。" {
		t.Fatalf("unexpected content: %q", content)
	}
}

func TestBuildPetSupportQuestionContainsOnlyQuestionAndCitations(t *testing.T) {
	prompt := buildPetSupportQuestion(service.PetAnswerInput{
		Question:  "账号为什么限流？",
		Citations: []service.PetCitation{{Title: "账号状态", Version: 3, Excerpt: "刷新状态。"}},
	})
	for _, want := range []string{"账号为什么限流？", "账号状态", "刷新状态。"} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q: %s", want, prompt)
		}
	}
}
