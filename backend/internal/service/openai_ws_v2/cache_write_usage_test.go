package openai_ws_v2

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseUsageAndAccumulateIncludesCacheWrite(t *testing.T) {
	state := &relayState{}
	message := []byte(`{
		"type":"response.completed",
		"response":{"usage":{
			"input_tokens":100,
			"output_tokens":20,
			"input_tokens_details":{"cached_tokens":30,"cache_write_tokens":25}
		}}
	}`)

	usage := parseUsageAndAccumulate(state, message, "response.completed", nil)
	require.Equal(t, 100, usage.InputTokens)
	require.Equal(t, 20, usage.OutputTokens)
	require.Equal(t, 30, usage.CacheReadInputTokens)
	require.Equal(t, 25, usage.CacheCreationInputTokens)
	require.Equal(t, usage, state.usage)
}

func TestParseUsageNestedZeroOverridesTopLevelCacheWrite(t *testing.T) {
	state := &relayState{}
	message := []byte(`{
		"type":"response.completed",
		"response":{"usage":{
			"input_tokens":100,
			"cache_creation_input_tokens":25,
			"input_tokens_details":{"cache_write_tokens":0}
		}}
	}`)

	usage := parseUsageAndAccumulate(state, message, "response.completed", nil)
	require.Zero(t, usage.CacheCreationInputTokens)
}
