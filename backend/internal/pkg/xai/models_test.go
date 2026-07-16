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
