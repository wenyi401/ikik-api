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
		"input":[{"type":"function_call","call_id":"call_1","name":"exec","arguments":{"cmd":"ls"}}]
	}`), &req)
	require.NoError(t, err)

	anth, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	require.Len(t, anth.Messages, 1)

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
		"input":[{"type":"function_call_output","call_id":"call_1","output":[{"type":"output_text","text":"done"}]}]
	}`), &req)
	require.NoError(t, err)

	anth, err := ResponsesToAnthropicRequest(&req)
	require.NoError(t, err)
	require.Len(t, anth.Messages, 1)

	var blocks []AnthropicContentBlock
	require.NoError(t, json.Unmarshal(anth.Messages[0].Content, &blocks))
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
	assert.Empty(t, anth.Tools[1].Type)
	assert.Equal(t, "web_search", anth.Tools[1].Name)
	assert.JSONEq(t, `{"type":"object","properties":{}}`, string(anth.Tools[0].InputSchema))
	assert.JSONEq(t, `{"type":"object","properties":{}}`, string(anth.Tools[1].InputSchema))
}

func TestResponses2916AnthropicStreamMessageDoneCarriesContent(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	var all []ResponsesStreamEvent
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_1",
			Model: "claude-test",
		},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type:         "content_block_start",
		ContentBlock: &AnthropicContentBlock{Type: "text"},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type:  "content_block_delta",
		Delta: &AnthropicDelta{Type: "text_delta", Text: "hello"},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "content_block_stop"}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "message_stop"}, state)...)

	assert.NotNil(t, findEvent2916(all, "response.content_part.added"))
	assert.NotNil(t, findEvent2916(all, "response.content_part.done"))
	done := findDoneItem2916(all, "message")
	require.NotNil(t, done)
	require.Len(t, done.Content, 1)
	assert.Equal(t, "hello", done.Content[0].Text)
}

func TestResponses2916AnthropicStreamFunctionDoneCarriesCall(t *testing.T) {
	state := NewAnthropicEventToResponsesState()
	var all []ResponsesStreamEvent
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type: "message_start",
		Message: &AnthropicResponse{
			ID:    "msg_1",
			Model: "claude-test",
		},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type:         "content_block_start",
		ContentBlock: &AnthropicContentBlock{Type: "tool_use", ID: "toolu_1", Name: "exec"},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{
		Type:  "content_block_delta",
		Delta: &AnthropicDelta{Type: "input_json_delta", PartialJSON: `{"cmd":"ls"}`},
	}, state)...)
	all = append(all, AnthropicEventToResponsesEvents(&AnthropicStreamEvent{Type: "content_block_stop"}, state)...)

	argsDone := findEvent2916(all, "response.function_call_arguments.done")
	require.NotNil(t, argsDone)
	assert.Equal(t, `{"cmd":"ls"}`, argsDone.Arguments)
	done := findDoneItem2916(all, "function_call")
	require.NotNil(t, done)
	assert.Equal(t, "fc_toolu_1", done.CallID)
	assert.Equal(t, "exec", done.Name)
	assert.Equal(t, `{"cmd":"ls"}`, done.Arguments)
}

func systemText2916(t *testing.T, raw json.RawMessage) string {
	t.Helper()
	var parts []ResponsesContentPart
	require.NoError(t, json.Unmarshal(raw, &parts))
	var out string
	for _, p := range parts {
		out += p.Text
	}
	return out
}

func findEvent2916(events []ResponsesStreamEvent, typ string) *ResponsesStreamEvent {
	for i := range events {
		if events[i].Type == typ {
			return &events[i]
		}
	}
	return nil
}

func findDoneItem2916(events []ResponsesStreamEvent, typ string) *ResponsesOutput {
	for i := range events {
		if events[i].Type == "response.output_item.done" && events[i].Item != nil && events[i].Item.Type == typ {
			return events[i].Item
		}
	}
	return nil
}
