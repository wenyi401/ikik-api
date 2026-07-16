package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDefaultModelsIncludeLatestOAuthModels(t *testing.T) {
	t.Parallel()

	ids := DefaultModelIDs()
	require.Contains(t, ids, "claude-fable-5")
	require.Contains(t, ids, "claude-sonnet-5")
}
