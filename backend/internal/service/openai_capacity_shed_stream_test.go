package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIStreamTransientErrorDoesNotStartClientOutput(t *testing.T) {
	require.False(t, openAIStreamDataStartsClientOutput(
		`{"type":"error","error":{"code":"server_is_overloaded","message":"overloaded"}}`,
		"error",
	))
	require.True(t, openAIStreamDataStartsClientOutput(
		`{"type":"error","error":{"code":"content_policy_violation","message":"blocked"}}`,
		"error",
	))
}

func TestSanitizeOpenAICapacityShedErrorCodeForClientStreamFrames(t *testing.T) {
	for _, payload := range []string{
		`{"type":"error","error":{"code":"server_is_overloaded","message":"overloaded"}}`,
		`{"type":"response.failed","response":{"error":{"code":"slow_down","message":"slow down"}}}`,
	} {
		updated, changed := sanitizeOpenAICapacityShedErrorCodeForClient([]byte(payload))
		require.True(t, changed)
		require.Contains(t, string(updated), `"code":"server_error"`)
	}

	updated, changed := sanitizeOpenAICapacityShedErrorCodeForClient([]byte(`{"error":{"code":"rate_limit_exceeded"}}`))
	require.False(t, changed)
	require.Equal(t, `{"error":{"code":"rate_limit_exceeded"}}`, string(updated))
}
