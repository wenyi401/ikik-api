//go:build unit

package routes

import (
	"testing"

	"ikik-api/internal/service"

	"github.com/stretchr/testify/require"
)

// OpenCode Go 的 /v1/chat/completions 入站必须经 OpenAI 网关转发：
// 通用链路会把带 /v1 的 Chat base 拼成 <base>/v1/messages?beta=true
// （即 /v1/v1/messages），上游返回 404，整个分组不可用。
func TestIsOpenAIChatCompatiblePlatform_IncludesOpenCodeGo(t *testing.T) {
	require.True(t, isOpenAIChatCompatiblePlatform(service.PlatformOpenCodeGo))
}

func TestIsOpenAIChatCompatiblePlatform_KeepsLegacyPlatforms(t *testing.T) {
	for _, platform := range []string{service.PlatformOpenAI, service.PlatformGrok, service.PlatformKiro} {
		require.True(t, isOpenAIChatCompatiblePlatform(platform), platform)
	}
}

// 国产供应商仍走通用链路（base 为 Anthropic 风格，不带 /v1），本改动不碰它们。
func TestIsOpenAIChatCompatiblePlatform_ExcludesCNProviders(t *testing.T) {
	for _, platform := range []string{
		service.PlatformKimi, service.PlatformZhipu, service.PlatformDeepseek, service.PlatformMiniMax,
		service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity, service.PlatformComposite,
	} {
		require.False(t, isOpenAIChatCompatiblePlatform(platform), platform)
	}
}
