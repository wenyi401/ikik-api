//go:build unit

package handler

import (
	"context"
	"testing"

	"ikik-api/internal/service"

	"github.com/stretchr/testify/require"
)

// 回归：OpenCode Go 分组经 /v1/chat/completions 入站时，调度平台必须是
// opencode_go 本身。若归一为 openai，调度器会按 platform=openai 查该分组的
// 账号（0 个）→ pool=0 → 503，整个分组不可用。
func TestOpenAICompatibleRequestPlatform_KeepsOpenCodeGo(t *testing.T) {
	apiKey := &service.APIKey{Group: &service.Group{Platform: service.PlatformOpenCodeGo}}
	require.Equal(t, service.PlatformOpenCodeGo, openAICompatibleRequestPlatform(context.Background(), apiKey))
}

func TestOpenAICompatibleRequestPlatform_KeepsCNProvidersAndGrokKiro(t *testing.T) {
	for _, platform := range []string{
		service.PlatformGrok, service.PlatformKiro, service.PlatformKimi, service.PlatformZhipu,
		service.PlatformDeepseek, service.PlatformMiniMax,
	} {
		apiKey := &service.APIKey{Group: &service.Group{Platform: platform}}
		require.Equal(t, platform, openAICompatibleRequestPlatform(context.Background(), apiKey), platform)
	}
}

func TestOpenAICompatibleRequestPlatform_FallsBackToOpenAI(t *testing.T) {
	for _, platform := range []string{
		service.PlatformAnthropic, service.PlatformGemini, service.PlatformAntigravity, service.PlatformComposite,
	} {
		require.Equal(t, service.PlatformOpenAI, openAICompatiblePlatformOrOpenAI(platform), platform)
	}
	require.Equal(t, service.PlatformOpenAI, openAICompatibleRequestPlatform(context.Background(), nil))
}
