//go:build unit

package service

import (
	"testing"

	"github.com/stretchr/testify/require"

	"ikik-api/internal/pkg/antigravity"
	"ikik-api/internal/pkg/claude"
	"ikik-api/internal/pkg/geminicli"
	"ikik-api/internal/pkg/openai"
)

// TestDefaultModelIDsForPlatform 锁定四平台 + 未知平台兜底的 ID 来源
// （Phase-3 TASK-003：两处同构 switch 收敛后的单一来源回归测试）。
func TestDefaultModelIDsForPlatform(t *testing.T) {
	require.Equal(t, openai.DefaultModelIDs(), DefaultModelIDsForPlatform(PlatformOpenAI))

	geminiIDs := DefaultModelIDsForPlatform(PlatformGemini)
	require.Len(t, geminiIDs, len(geminicli.DefaultModels))
	for i, m := range geminicli.DefaultModels {
		require.Equal(t, m.ID, geminiIDs[i])
	}

	agModels := antigravity.DefaultModels()
	agIDs := DefaultModelIDsForPlatform(PlatformAntigravity)
	require.Len(t, agIDs, len(agModels))
	for i, m := range agModels {
		require.Equal(t, m.ID, agIDs[i])
	}

	for _, unknown := range []string{PlatformAnthropic, "", "no-such-platform"} {
		ids := DefaultModelIDsForPlatform(unknown)
		require.Len(t, ids, len(claude.DefaultModels), "platform=%q 应回退 Claude 默认集", unknown)
		for i, m := range claude.DefaultModels {
			require.Equal(t, m.ID, ids[i])
		}
	}
}
