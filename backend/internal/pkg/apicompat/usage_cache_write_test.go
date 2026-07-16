package apicompat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResponsesUsageUnmarshalAndChatConversionPreserveCacheWrite(t *testing.T) {
	var usage ResponsesUsage
	err := json.Unmarshal([]byte(`{
		"prompt_tokens": 100,
		"completion_tokens": 20,
		"prompt_tokens_details": {"cached_tokens": 30, "cache_write_tokens": 25},
		"completion_tokens_details": {"reasoning_tokens": 7}
	}`), &usage)
	require.NoError(t, err)
	require.Equal(t, 100, usage.InputTokens)
	require.Equal(t, 20, usage.OutputTokens)
	require.Equal(t, 120, usage.TotalTokens)
	require.NotNil(t, usage.InputTokensDetails)
	require.Equal(t, 25, usage.InputTokensDetails.CacheWriteTokens)

	chat := ResponsesToChatCompletions(&ResponsesResponse{Usage: &usage}, "gpt-5.6-sol")
	require.NotNil(t, chat.Usage)
	require.NotNil(t, chat.Usage.PromptTokensDetails)
	require.Equal(t, 30, chat.Usage.PromptTokensDetails.CachedTokens)
	require.Equal(t, 25, chat.Usage.PromptTokensDetails.CacheWriteTokens)
	require.NotNil(t, chat.Usage.CompletionTokensDetails)
	require.Equal(t, 7, chat.Usage.CompletionTokensDetails.ReasoningTokens)
}

func TestResponsesUsageTopLevelCacheWriteFallback(t *testing.T) {
	var usage ResponsesUsage
	require.NoError(t, json.Unmarshal([]byte(`{
		"input_tokens": 50,
		"output_tokens": 5,
		"cache_write_input_tokens": 12
	}`), &usage))
	require.Equal(t, 12, usage.CacheCreationInputTokens)

	chat := chatUsageFromResponsesUsage(&usage)
	require.NotNil(t, chat.PromptTokensDetails)
	require.Equal(t, 12, chat.PromptTokensDetails.CacheCreationTokens)
}

func TestResponsesUsageNestedZeroOverridesTopLevelCacheWrite(t *testing.T) {
	var usage ResponsesUsage
	require.NoError(t, json.Unmarshal([]byte(`{
		"input_tokens": 50,
		"cache_write_input_tokens": 12,
		"input_tokens_details": {"cache_write_tokens": 0}
	}`), &usage))
	require.Zero(t, usage.CacheCreationInputTokens)
}
