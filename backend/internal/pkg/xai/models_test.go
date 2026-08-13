package xai

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsIncludeLatestOAuthModels(t *testing.T) {
	t.Parallel()

	require.Contains(t, DefaultModelIDs(), "grok-4.5")
	require.Contains(t, DefaultModelIDs(), "grok-composer-2.5-fast")
}

func TestDefaultModelMappingUsesLatestAliases(t *testing.T) {
	t.Parallel()

	mapping := DefaultModelMapping()
	require.Equal(t, "grok-4.5", mapping["grok-latest"])
	require.Equal(t, "grok-4.5", mapping["grok-4.5-latest"])
	require.Equal(t, "grok-composer-2.5-fast", mapping["grok-composer"])
}

func TestIsGrokModelID(t *testing.T) {
	t.Parallel()
	require.True(t, IsGrokModelID("grok-4.5"))
	require.True(t, IsGrokModelID("grok-4.6"))
	require.True(t, IsGrokModelID("x-ai/grok-4.3"))
	require.False(t, IsGrokModelID("gpt-5"))
	require.False(t, IsGrokModelID("claude-sonnet-4"))
}

func TestDefaultModelsIncludesGrok46(t *testing.T) {
	t.Parallel()
	ids := DefaultModelIDs()
	require.Contains(t, ids, "grok-4.6")
	require.Equal(t, "grok-4.6", ResolveGrokTextResponsesModelID("grok-4.6"))
	require.Equal(t, "grok-4.6", ResolveGrokTextResponsesModelID("grok-4.6-latest"))
}

func TestResolveGrokTextResponsesModelID(t *testing.T) {
	t.Parallel()
	require.Equal(t, "grok-4.5", ResolveGrokTextResponsesModelID(""))
	require.Equal(t, "grok-4.3", ResolveGrokTextResponsesModelID("grok", "grok-4.3"))
	require.Equal(t, "grok-4.20-multi-agent-0309", ResolveGrokTextResponsesModelID("grok-4.20-multi-agent"))
}
