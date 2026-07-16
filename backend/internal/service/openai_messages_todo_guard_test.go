package service

import (
	"encoding/json"
	"testing"

	"ikik-api/internal/pkg/apicompat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAppendOpenAICompatClaudeCodeTodoGuard(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`[{"type":"message","role":"developer","content":[{"type":"input_text","text":"existing"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]`)
	req := &apicompat.ResponsesRequest{Input: input}

	appended := appendOpenAICompatClaudeCodeTodoGuard(req)

	require.True(t, appended)
	require.Equal(t, "existing", gjson.GetBytes(req.Input, "0.content.0.text").String())
	require.Contains(t, gjson.GetBytes(req.Input, "1.content.0.text").String(), openAICompatClaudeCodeTodoGuardMarker)
	require.Equal(t, "hello", gjson.GetBytes(req.Input, "2.content.0.text").String())
}

func TestAppendOpenAICompatClaudeCodeTodoGuardSkipsExistingMarker(t *testing.T) {
	t.Parallel()

	input := json.RawMessage(`[{"type":"message","role":"developer","content":[{"type":"input_text","text":"<ikik-api-claude-code-todo-guard>"}]},{"type":"message","role":"user","content":[{"type":"input_text","text":"hello"}]}]`)
	req := &apicompat.ResponsesRequest{Input: input}

	appended := appendOpenAICompatClaudeCodeTodoGuard(req)

	require.False(t, appended)
	require.JSONEq(t, string(input), string(req.Input))
}
