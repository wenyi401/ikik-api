package claudeweb

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSupportedModelsAndValidation(t *testing.T) {
	models := SupportedModels()
	require.Contains(t, models, DefaultModel)
	require.Contains(t, models, "claude-opus-4-8")
	require.NoError(t, ValidateModel(DefaultModel))
	require.ErrorAs(t, ValidateModel("claude-unknown"), new(*UnsupportedModelError))

	models[0] = "mutated"
	require.NotEqual(t, "mutated", SupportedModels()[0])
}
