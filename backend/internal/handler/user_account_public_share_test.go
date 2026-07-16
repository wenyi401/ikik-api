package handler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsOpenAIUsageLimitReachedValidationError(t *testing.T) {
	require.True(t, isOpenAIUsageLimitReachedValidationError(`API returned 429: {"error":{"type":"usage_limit_reached","message":"The usage limit has been reached"}}`))
	require.True(t, isOpenAIUsageLimitReachedValidationError(`API returned 429: {"error": {"type": "usage_limit_reached"}}`))
	require.False(t, isOpenAIUsageLimitReachedValidationError(`API returned 429: {"error":{"type":"rate_limit_exceeded"}}`))
	require.False(t, isOpenAIUsageLimitReachedValidationError(`API returned 401: {"error":{"type":"usage_limit_reached"}}`))
	require.False(t, isOpenAIUsageLimitReachedValidationError(`Request failed: dial tcp timeout`))
}
