package dto

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/service"
)

func TestGroupFromService_IncludesScope(t *testing.T) {
	got := GroupFromService(&service.Group{
		ID:           42,
		Name:         "OpenAI Private",
		Platform:     "openai",
		Scope:        service.GroupScopeUserPrivate,
		IsSharedPool: true,
	})

	require.NotNil(t, got)
	require.Equal(t, service.GroupScopeUserPrivate, got.Scope)
	require.True(t, got.IsSharedPool)

	raw, err := json.Marshal(got)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"scope":"user_private"`)
	require.Contains(t, string(raw), `"is_shared_pool":true`)
}
