package claudeweb

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func testToolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "get_weather",
			Description: "Get current weather",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"city":{"type":"string"}},"required":["city"]}`),
		},
		{
			Name:        "read_file",
			Description: "Read a file",
			InputSchema: json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
	}
}

func TestToolBridgeConfigSupportsChoiceModes(t *testing.T) {
	auto, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"auto","disable_parallel_tool_use":true}`))
	require.NoError(t, err)
	require.True(t, auto.Enabled())
	require.False(t, auto.AllowParallel)

	required, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"any"}`))
	require.NoError(t, err)
	require.True(t, required.RequiresTool())

	specific, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"tool","name":"read_file"}`))
	require.NoError(t, err)
	require.Equal(t, ToolChoiceSpecific, specific.ChoiceMode)
	require.Equal(t, "read_file", specific.ForcedTool)
	require.False(t, specific.AllowParallel)
	require.Len(t, specific.Tools, 1)
	require.Equal(t, "read_file", specific.Tools[0].Name)

	none, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"none"}`))
	require.NoError(t, err)
	require.False(t, none.Enabled())

	_, err = NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"tool","name":"missing"}`))
	require.ErrorContains(t, err, "unknown tool")
}

func TestToolBridgePromptIncludesFullSchemasAndConstraints(t *testing.T) {
	config, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"tool","name":"read_file"}`))
	require.NoError(t, err)
	prompt, err := config.Prompt()
	require.NoError(t, err)
	require.Contains(t, prompt, toolBridgeCallOpen)
	require.Contains(t, prompt, toolBridgeFinalOpen)
	require.Contains(t, prompt, "Name: read_file")
	require.NotContains(t, prompt, `"required":["city"]`)
	require.Contains(t, prompt, "You must call one available tool")
	require.NotContains(t, prompt, "get_weather")
}

func TestToolBridgeParserStreamsFinalAnswerWithoutTags(t *testing.T) {
	config, err := NewToolBridgeConfig(testToolDefinitions(), nil)
	require.NoError(t, err)
	parser := NewToolBridgeParser(config)

	var output strings.Builder
	for _, fragment := range []string{"  <ikik_f", "inal>Hello ", "from Claude", " Web</ikik_", "final>  "} {
		text, calls, feedErr := parser.Feed(fragment)
		require.NoError(t, feedErr)
		require.Empty(t, calls)
		output.WriteString(text)
	}
	text, calls, err := parser.Finish()
	require.NoError(t, err)
	require.Empty(t, calls)
	output.WriteString(text)
	require.Equal(t, "Hello from Claude Web", output.String())
}

func TestToolBridgeParserParsesParallelCallsAcrossFragments(t *testing.T) {
	config, err := NewToolBridgeConfig(testToolDefinitions(), nil)
	require.NoError(t, err)
	parser := NewToolBridgeParser(config)

	fragments := []string{
		"<ikik_tool_",
		"calls>[{\"name\":\"get_weather\",\"arguments\":{\"city\":\"Tokyo\"}},",
		"{\"name\":\"read_file\",\"arguments\":{\"path\":\"/tmp/a\"}}]</ikik_tool_calls>",
	}
	var calls []ToolCall
	for _, fragment := range fragments {
		text, parsed, feedErr := parser.Feed(fragment)
		require.NoError(t, feedErr)
		require.Empty(t, text)
		if len(parsed) > 0 {
			calls = parsed
		}
	}
	require.Len(t, calls, 2)
	require.Equal(t, "get_weather", calls[0].Name)
	require.JSONEq(t, `{"city":"Tokyo"}`, string(calls[0].Arguments))
	require.Equal(t, "read_file", calls[1].Name)
}

func TestToolBridgeParserParsesWeb2APIThinkingAndSingleToolCall(t *testing.T) {
	config, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"tool","name":"get_weather"}`))
	require.NoError(t, err)
	parser := NewToolBridgeParser(config)

	var calls []ToolCall
	for _, fragment := range []string{
		"<think>I should use the weather tool.</think>\n<tool_",
		"call>{\"name\":\"get_weather\",\"arguments\":{\"city\":\"Tokyo\"}}</tool_call>",
	} {
		text, parsed, feedErr := parser.Feed(fragment)
		require.NoError(t, feedErr)
		require.Empty(t, text)
		if len(parsed) > 0 {
			calls = parsed
		}
	}

	require.Len(t, calls, 1)
	require.Equal(t, "get_weather", calls[0].Name)
	require.JSONEq(t, `{"city":"Tokyo"}`, string(calls[0].Arguments))
}

func TestToolBridgeParserEnforcesRequiredAndKnownTools(t *testing.T) {
	required, err := NewToolBridgeConfig(testToolDefinitions(), json.RawMessage(`{"type":"any"}`))
	require.NoError(t, err)
	_, _, err = NewToolBridgeParser(required).Feed("I will answer without a tool")
	require.ErrorContains(t, err, "required tagged tool call")

	auto, err := NewToolBridgeConfig(testToolDefinitions(), nil)
	require.NoError(t, err)
	_, _, err = NewToolBridgeParser(auto).Feed(`<ikik_tool_calls>[{"name":"unknown","arguments":{}}]</ikik_tool_calls>`)
	require.ErrorContains(t, err, "unknown tool")
}
