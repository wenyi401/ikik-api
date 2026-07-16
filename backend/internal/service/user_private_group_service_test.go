//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultPrivateGroupAllowMessagesDispatch(t *testing.T) {
	require.True(t, defaultPrivateGroupAllowMessagesDispatch(PlatformOpenAI))
	require.True(t, defaultPrivateGroupAllowMessagesDispatch(" OpenAI "))

	require.False(t, defaultPrivateGroupAllowMessagesDispatch(PlatformAnthropic))
	require.False(t, defaultPrivateGroupAllowMessagesDispatch(PlatformGemini))
	require.False(t, defaultPrivateGroupAllowMessagesDispatch(PlatformAntigravity))
}
