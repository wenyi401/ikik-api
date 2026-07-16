package claudeweb

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConvertStreamSupportsEventTypeOutsidePayload(t *testing.T) {
	source := strings.NewReader(strings.Join([]string{
		"event: content_block_start",
		`data: {"index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"index":0,`,
		`data: "delta":{"type":"text_delta","text":"Hello"}}`,
		"",
		"event: content_block_stop",
		`data: {"index":0}`,
		"",
	}, "\n"))

	var destination bytes.Buffer
	usage, err := ConvertStream(context.Background(), source, &destination, DefaultModel, 11)
	require.NoError(t, err)
	require.Equal(t, 11, usage.InputTokens)
	require.Positive(t, usage.OutputTokens)
	require.Contains(t, destination.String(), `"type":"content_block_start"`)
	require.Contains(t, destination.String(), `"type":"content_block_delta"`)
	require.Contains(t, destination.String(), `"text":"Hello"`)
	require.Contains(t, destination.String(), "event: message_stop")
}

func TestConvertStreamPropagatesUpstreamErrorEvent(t *testing.T) {
	source := strings.NewReader("event: error\ndata: {\"error\":{\"message\":\"session expired\"}}\n\n")

	var destination bytes.Buffer
	_, err := ConvertStream(context.Background(), source, &destination, DefaultModel, 0)
	require.EqualError(t, err, "session expired")
}

func TestConvertStreamToolBridgeEmitsStructuredToolUse(t *testing.T) {
	config, err := NewToolBridgeConfig([]ToolDefinition{{
		Name:        "get_weather",
		Description: "Get weather",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}}}`),
	}}, nil)
	require.NoError(t, err)
	source := strings.NewReader(strings.Join([]string{
		"event: content_block_start",
		`data: {"index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"index":0,"delta":{"type":"text_delta","text":"<ikik_tool_"}}`,
		"",
		"event: content_block_delta",
		`data: {"index":0,"delta":{"type":"text_delta","text":"calls>[{\"name\":\"get_weather\",\"arguments\":{\"city\":\"Tokyo\"}}]</ikik_tool_calls>"}}`,
		"",
		"event: content_block_stop",
		`data: {"index":0}`,
		"",
		"event: message_delta",
		`data: {"delta":{"stop_reason":"end_turn"}}`,
		"",
	}, "\n"))

	var destination bytes.Buffer
	_, err = ConvertStreamWithOptions(context.Background(), source, &destination, StreamOptions{
		Model:       DefaultModel,
		InputTokens: 17,
		ToolBridge:  config,
	})
	require.NoError(t, err)
	output := destination.String()
	require.NotContains(t, output, "ikik_tool_calls")
	require.Contains(t, output, `"type":"tool_use"`)
	require.Contains(t, output, `"name":"get_weather"`)
	require.Contains(t, output, `"partial_json":"{\"city\":\"Tokyo\"}"`)
	require.Contains(t, output, `"stop_reason":"tool_use"`)
}

func TestConvertStreamToolBridgeStreamsFinalAnswer(t *testing.T) {
	config, err := NewToolBridgeConfig([]ToolDefinition{{
		Name:        "get_weather",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`),
	}}, nil)
	require.NoError(t, err)
	source := strings.NewReader(strings.Join([]string{
		"event: content_block_start",
		`data: {"index":0,"content_block":{"type":"text","text":""}}`,
		"",
		"event: content_block_delta",
		`data: {"index":0,"delta":{"type":"text_delta","text":"<ikik_final>Hello "}}`,
		"",
		"event: content_block_delta",
		`data: {"index":0,"delta":{"type":"text_delta","text":"world</ikik_final>"}}`,
		"",
		"event: content_block_stop",
		`data: {"index":0}`,
		"",
	}, "\n"))

	var destination bytes.Buffer
	_, err = ConvertStreamWithOptions(context.Background(), source, &destination, StreamOptions{
		Model:      DefaultModel,
		ToolBridge: config,
	})
	require.NoError(t, err)
	output := destination.String()
	require.NotContains(t, output, "ikik_final")
	require.Contains(t, output, `"text":"Hello world"`)
	require.Contains(t, output, `"stop_reason":"end_turn"`)
}

func TestConvertStreamPreservesUpstreamStopReason(t *testing.T) {
	source := strings.NewReader(strings.Join([]string{
		"event: content_block_start",
		`data: {"index":0,"content_block":{"type":"tool_use","id":"toolu_1","name":"get_weather","input":{}}}`,
		"",
		"event: content_block_stop",
		`data: {"index":0}`,
		"",
		"event: message_delta",
		`data: {"delta":{"stop_reason":"tool_use"}}`,
		"",
	}, "\n"))

	var destination bytes.Buffer
	_, err := ConvertStream(context.Background(), source, &destination, DefaultModel, 0)
	require.NoError(t, err)
	require.Contains(t, destination.String(), `"stop_reason":"tool_use"`)
}
