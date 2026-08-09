package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func experimentalPromptTestAPIKey(group *Group) *APIKey {
	return &APIKey{
		Group: group,
		GroupRoutes: []APIKeyGroupRoute{{
			GroupID: group.ID,
			Enabled: true,
			Group:   group,
		}},
	}
}

func TestAPIKeyOpenAIExperimentalPromptPreferenceRequiresUnlockedUser(t *testing.T) {
	requested := true
	apiKey := experimentalPromptTestAPIKey(&Group{
		ID:                              1,
		Platform:                        PlatformOpenAI,
		OpenAIExperimentalPromptEnabled: true,
	})

	err := applyAPIKeyOpenAIExperimentalPromptPreference(&User{}, apiKey, &requested, false)

	require.ErrorIs(t, err, ErrOpenAIExperimentalPromptLocked)
	require.False(t, apiKey.OpenAIExperimentalPromptEnabled)
}

func TestAPIKeyOpenAIExperimentalPromptPreferenceRequiresSupportedRoute(t *testing.T) {
	requested := true
	apiKey := experimentalPromptTestAPIKey(&Group{
		ID:                              2,
		Platform:                        PlatformAnthropic,
		OpenAIExperimentalPromptEnabled: true,
	})

	err := applyAPIKeyOpenAIExperimentalPromptPreference(&User{OpenAIExperimentalPromptUnlocked: true}, apiKey, &requested, false)

	require.ErrorIs(t, err, ErrOpenAIExperimentalPromptGroupUnsupported)
	require.False(t, apiKey.OpenAIExperimentalPromptEnabled)
}

func TestAPIKeyOpenAIExperimentalPromptPreferenceClearsOnUnsupportedRouteChange(t *testing.T) {
	apiKey := experimentalPromptTestAPIKey(&Group{
		ID:       3,
		Platform: PlatformAnthropic,
	})
	apiKey.OpenAIExperimentalPromptEnabled = true

	err := applyAPIKeyOpenAIExperimentalPromptPreference(nil, apiKey, nil, true)

	require.NoError(t, err)
	require.False(t, apiKey.OpenAIExperimentalPromptEnabled)
}

func TestAPIKeyOpenAIExperimentalPromptPreferencePreservesOnSupportedRouteChange(t *testing.T) {
	apiKey := experimentalPromptTestAPIKey(&Group{
		ID:                              4,
		Platform:                        PlatformOpenAI,
		OpenAIExperimentalPromptEnabled: true,
	})
	apiKey.OpenAIExperimentalPromptEnabled = true

	err := applyAPIKeyOpenAIExperimentalPromptPreference(nil, apiKey, nil, true)

	require.NoError(t, err)
	require.True(t, apiKey.OpenAIExperimentalPromptEnabled)
}
