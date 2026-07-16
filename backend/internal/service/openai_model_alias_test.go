package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeKnownOpenAICodexModelKeepsGPT55Pro(t *testing.T) {
	t.Parallel()

	require.Equal(t, "gpt-5.5-pro", normalizeKnownOpenAICodexModel("gpt-5.5-pro"))
	require.Equal(t, "gpt-5.5-pro", normalizeKnownOpenAICodexModel("openai/gpt-5.5-pro"))
}
