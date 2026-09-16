//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// OpenCode 账号只有 API Key 类型，但它的订阅（Zen / GO）需要能像 OAuth 账号一样
// 进入共享号池与拼车池；这里锁定「按平台放开」的行为，避免以后被误删。
func TestSharedPoolWhitelistsIncludeOpenCodeGo(t *testing.T) {
	require.True(t, IsSupportedCarpoolPlatform(PlatformOpenCodeGo))
	require.True(t, IsSupportedUserCarpoolGroupPlatform(PlatformOpenCodeGo))
	require.True(t, IsSupportedUserPrivateGroupPlatform(PlatformOpenCodeGo))
	require.True(t, supportsOwnedPublicSharePoolPlatform(PlatformOpenCodeGo))
}

func TestSharedPoolWhitelistsKeepLegacyPlatforms(t *testing.T) {
	for _, platform := range []string{PlatformOpenAI, PlatformAnthropic, PlatformGemini, PlatformAntigravity} {
		require.True(t, IsSupportedCarpoolPlatform(platform), platform)
		require.True(t, IsSupportedUserCarpoolGroupPlatform(platform), platform)
		require.True(t, supportsOwnedPublicSharePoolPlatform(platform), platform)
	}
	require.True(t, IsSupportedUserPrivateGroupPlatform(PlatformKiro))
	require.True(t, supportsOwnedPublicSharePoolPlatform(PlatformGrok))
}

func TestSharedPoolWhitelistsStillRejectOtherPlatforms(t *testing.T) {
	require.False(t, IsSupportedCarpoolPlatform(PlatformKimi))
	require.False(t, IsSupportedUserCarpoolGroupPlatform(PlatformZhipu))
	require.False(t, supportsOwnedPublicSharePoolPlatform(PlatformDeepseek))
	require.False(t, IsSupportedUserPrivateGroupPlatform(PlatformMiniMax))
}

func TestOwnedAccountForcesPrivateShareForPlatform(t *testing.T) {
	// OpenCode：API Key 账号允许公开共享进号池。
	require.False(t, ownedAccountForcesPrivateShareForPlatform(PlatformOpenCodeGo, AccountTypeAPIKey))

	// 其他平台：API Key / Bedrock / ServiceAccount 仍然强制私有。
	require.True(t, ownedAccountForcesPrivateShareForPlatform(PlatformAnthropic, AccountTypeAPIKey))
	require.True(t, ownedAccountForcesPrivateShareForPlatform(PlatformOpenAI, AccountTypeAPIKey))
	require.True(t, ownedAccountForcesPrivateShareForPlatform(PlatformGrok, AccountTypeBedrock))
	require.True(t, ownedAccountForcesPrivateShareForPlatform(PlatformGemini, AccountTypeServiceAccount))

	// OAuth / SetupToken 本来就可以公开共享。
	require.False(t, ownedAccountForcesPrivateShareForPlatform(PlatformAnthropic, AccountTypeOAuth))
	require.False(t, ownedAccountForcesPrivateShareForPlatform(PlatformOpenCodeGo, AccountTypeOAuth))
}
