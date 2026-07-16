package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNormalizeOpenAIPlusConcurrency_DefaultAndAdminConfiguredValue(t *testing.T) {
	got, err := NormalizeOpenAIPlusConcurrency(PlatformOpenAI, AccountLevelPlus, 0)
	require.NoError(t, err)
	require.Equal(t, OpenAIPlusDefaultConcurrency, got)

	got, err = NormalizeOpenAIPlusConcurrency(PlatformOpenAI, AccountLevelPlus, 5)
	require.NoError(t, err)
	require.Equal(t, 5, got)
}

func TestDefaultOAuthAccountConcurrencyForPlatform(t *testing.T) {
	require.Equal(t, OpenAIPlusDefaultConcurrency, DefaultOAuthAccountConcurrencyForPlatform(PlatformOpenAI))
	require.Equal(t, OAuthAccountDefaultConcurrency, DefaultOAuthAccountConcurrencyForPlatform(PlatformAnthropic))
	require.Equal(t, OAuthAccountDefaultConcurrency, DefaultOAuthAccountConcurrencyForPlatform(PlatformGemini))
}

func TestNormalizeOpenAIAccountLevel_FromPlanType(t *testing.T) {
	require.Equal(t, AccountLevelPlus, NormalizeOpenAIAccountLevel(
		PlatformOpenAI,
		AccountLevelUnknown,
		map[string]any{"plan_type": "plus"},
		nil,
	))
}

func TestNormalizeOpenAIAccountLevel_ManualLevelTakesPriority(t *testing.T) {
	require.Equal(t, AccountLevelPro, NormalizeOpenAIAccountLevel(
		PlatformOpenAI,
		AccountLevelPro,
		map[string]any{"plan_type": "plus"},
		nil,
	))
	require.Equal(t, AccountLevelUnknown, NormalizeOpenAIAccountLevel(
		PlatformOpenAI,
		AccountLevelUnknown,
		map[string]any{"account_level": "pro"},
		nil,
	))
	require.Equal(t, AccountLevelUnknown, NormalizeOpenAIAccountLevel(
		PlatformAnthropic,
		AccountLevelUnknown,
		map[string]any{"plan_type": "plus"},
		nil,
	))
}

func TestValidateAccountLoadFactor_Max(t *testing.T) {
	loadFactor := AccountMaxLoadFactor + 1
	require.Error(t, ValidateAccountLoadFactor(&loadFactor))

	loadFactor = AccountMaxLoadFactor
	require.NoError(t, ValidateAccountLoadFactor(&loadFactor))
}
