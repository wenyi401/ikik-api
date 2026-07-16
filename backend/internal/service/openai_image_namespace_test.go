package service

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageGenerationIntentRecognizesNamespaceDeclarations(t *testing.T) {
	tests := []struct {
		name string
		body string
		want bool
	}{
		{
			name: "top-level namespace",
			body: `{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen"}]}`,
			want: true,
		},
		{
			name: "responses lite additional tools",
			body: `{"model":"gpt-5.5","input":[{"type":"additional_tools","tools":[{"type":"namespace","name":"image_gen"}]}]}`,
			want: true,
		},
		{
			name: "namespace tool choice",
			body: `{"model":"gpt-5.5","tool_choice":{"type":"namespace","name":"image_gen"}}`,
			want: true,
		},
		{
			name: "custom function is not image generation",
			body: `{"model":"gpt-5.5","tool_choice":{"function":{"name":"imagegen"}}}`,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, IsImageGenerationIntent("/v1/responses", "gpt-5.5", []byte(tt.body)))
		})
	}
}

func TestStripOpenAIImageGenerationToolsRemovesNamespaceEverywhere(t *testing.T) {
	imageNamespace := func() map[string]any {
		return map[string]any{"type": "namespace", "name": "image_gen"}
	}
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{"type": "function", "name": "shell"},
			imageNamespace(),
		},
		"input": []any{
			map[string]any{"type": "message", "role": "user", "content": "hello"},
			map[string]any{"type": "additional_tools", "tools": []any{imageNamespace()}},
		},
		"tool_choice": map[string]any{"type": "namespace", "name": "image_gen"},
	}

	require.True(t, stripOpenAIImageGenerationTools(reqBody))
	require.False(t, hasOpenAIImageGenerationTool(reqBody))
	require.NotContains(t, reqBody, "tool_choice")
	require.Len(t, reqBody["tools"], 1)
	require.Len(t, reqBody["input"], 1)
	require.False(t, stripOpenAIImageGenerationTools(reqBody))
}

func TestStripOpenAIImageGenerationToolsFromRawPayload(t *testing.T) {
	payload := []byte(`{"model":"gpt-5.5","tools":[{"type":"namespace","name":"image_gen"}],"tool_choice":{"type":"namespace","name":"image_gen"}}`)
	stripped, changed, err := stripOpenAIImageGenerationToolsFromRawPayload(payload)
	require.NoError(t, err)
	require.True(t, changed)

	var decoded map[string]any
	require.NoError(t, json.Unmarshal(stripped, &decoded))
	require.NotContains(t, decoded, "tools")
	require.NotContains(t, decoded, "tool_choice")
}
