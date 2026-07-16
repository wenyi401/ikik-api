package claudeweb

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildCompletionRequestMatchesBrowserShape(t *testing.T) {
	request, humanUUID, assistantUUID := buildCompletionRequest(CompletionOptions{
		Model:        DefaultModel,
		Prompt:       "hello",
		Effort:       "high",
		ThinkingMode: "auto",
	})

	require.Equal(t, "hello", request.Prompt)
	require.Equal(t, DefaultModel, request.Model)
	require.Equal(t, "high", request.Effort)
	require.Equal(t, "auto", request.ThinkingMode)
	require.Equal(t, DefaultTimezone, request.Timezone)
	require.Equal(t, DefaultLocale, request.Locale)
	require.NotEmpty(t, request.ParentMessageUUID)
	require.Equal(t, humanUUID, request.TurnMessageUUIDs.HumanMessageUUID)
	require.Equal(t, assistantUUID, request.TurnMessageUUIDs.AssistantMessageUUID)
	require.Equal(t, "messages", request.RenderingMode)
	require.Equal(t, "auto", request.CreateConversationParams.ToolSearchMode)
	require.True(t, request.CreateConversationParams.EnabledImagine)
	require.True(t, json.Valid(request.Tools))
	require.Contains(t, string(request.Tools), `"web_search_v0"`)
}

func TestNormalizeCompletionOptionsRejectsUnknownEffort(t *testing.T) {
	options := normalizeCompletionOptions(CompletionOptions{Effort: "turbo", ThinkingMode: "disabled"})
	require.Equal(t, "medium", options.Effort)
	require.Equal(t, "auto", options.ThinkingMode)
	require.Equal(t, DefaultModel, options.Model)
}

func TestBuildCompletionRequestDisablesWebToolsForClientToolBridge(t *testing.T) {
	request, _, _ := buildCompletionRequest(CompletionOptions{
		Prompt:          "use the client tool",
		DisableWebTools: true,
	})
	require.JSONEq(t, `[]`, string(request.Tools))
}
