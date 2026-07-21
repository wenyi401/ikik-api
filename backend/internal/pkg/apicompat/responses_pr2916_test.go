package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponses2916NormalizeArguments(t *testing.T) {
	tests := []struct {
		name string
		raw  json.RawMessage
		want string
	}{
		{name: "stringified object", raw: json.RawMessage(`"{\"cmd\":\"ls\"}"`), want: `{"cmd":"ls"}`},
		{name: "raw object", raw: json.RawMessage(`{"cmd":"ls"}`), want: `{"cmd":"ls"}`},
		{name: "empty string", raw: json.RawMessage(`""`), want: `{}`},
		{name: "invalid string", raw: json.RawMessage(`"not json"`), want: `{}`},
		{name: "null", raw: json.RawMessage(`null`), want: `{}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.JSONEq(t, tt.want, string(normalizeResponsesArguments(tt.raw)))
		})
	}
}

func TestResponses2916ExtractOutputText(t *testing.T) {
	assert.Equal(t, "ok", extractResponsesOutputText(json.RawMessage(`"ok"`)))
	assert.Equal(t, "one\n\ntwo", extractResponsesOutputText(json.RawMessage(`[
		{"type":"output_text","text":"one"},
		{"type":"output_text","text":"two"}
	]`)))
	assert.Equal(t, "", extractResponsesOutputText(nil))
}

func TestResponses2916ToAnthropicObjectArguments(t *testing.T) {
	var req ResponsesRequest
	err := json.Unmarshal([]byte(`{
		"model":"claude-test",
		"input":[
			{"type":"function_call","call_id":"call_1","name":"exec","arguments":{"cmd":"ls"}},
			{"type":"function_call_output","call_id":"call_1","output":"done"}
		]
	}`), &req)
	require.NoError(t, err)

	anth, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	require.Len(t, anth.Messages, 2)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(anth.Messages[0].Content, &blocks))
	require.Len(t, blocks, 1)
	assert.Equal(t, "tool_use", blocks[0].Type)
	assert.JSONEq(t, `{"cmd":"ls"}`, string(blocks[0].Input))
}

func TestResponses2916ToAnthropicOutputArray(t *testing.T) {
	var req ResponsesRequest
	err := json.Unmarshal([]byte(`{
		"model":"claude-test",
		"input":[
			{"type":"function_call","call_id":"call_1","name":"exec","arguments":{"cmd":"ls"}},
			{"type":"function_call_output","call_id":"call_1","output":[{"type":"output_text","text":"done"}]}
		]
	}`), &req)
	require.NoError(t, err)

	anth, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	require.Len(t, anth.Messages, 2)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(anth.Messages[1].Content, &blocks))
	require.Len(t, blocks, 1)
	var content string
	require.NoError(t, json.Unmarshal(blocks[0].Content, &content))
	assert.Equal(t, "done", content)
}

func TestResponses2916InstructionsAndDeveloperBecomeSystem(t *testing.T) {
	var req ResponsesRequest
	err := json.Unmarshal([]byte(`{
		"model":"claude-test",
		"instructions":"top system",
		"input":[
			{"role":"developer","content":"dev system"},
			{"role":"user","content":"hello"}
		]
	}`), &req)
	require.NoError(t, err)

	anth, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	assert.Equal(t, "top system\n\ndev system", systemText2916(t, anth.System))
}

func TestResponses2916EmptySystemOmitted(t *testing.T) {
	var req ResponsesRequest
	err := json.Unmarshal([]byte(`{
		"model":"claude-test",
		"instructions":"   ",
		"input":[{"role":"user","content":"hello"}]
	}`), &req)
	require.NoError(t, err)

	anth, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	assert.Empty(t, anth.System)
}

func TestResponses2916ToolSchemaAndWebSearch(t *testing.T) {
	req := &ResponsesRequest{
		Model: "claude-test",
		Input: json.RawMessage(`[{"role":"user","content":"hello"}]`),
		Tools: []ResponsesTool{
			{Type: "function", Name: "run"},
			{Type: "web_search"},
		},
	}

	anth, err := ResponsesToAnthropicRequest(req)
	require.NoError(t, err)
	require.Len(t, anth.Tools, 2)
	assert.Equal(t, "web_search_20250305", anth.Tools[1].Type)
	assert.Equal(t, "web_search", anth.Tools[1].Name)
	assert.JSONEq(t, `{"type":"object","properties":{}}`, string(anth.Tools[0].InputSchema))
	assert.Empty(t, anth.Tools[1].InputSchema)
}

func systemText2916(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return text
	}
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(raw, &parts))
	var out string
	for _, p := range parts {
		out += p.Text
	}
	return out
}
