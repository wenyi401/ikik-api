package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func codexConnectorToolNames(t *testing.T, reqBody map[string]any) []string {
	t.Helper()
	raw, ok := reqBody["tools"].([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(raw))
	for _, tool := range raw {
		names = append(names, codexToolName(tool))
	}
	return names
}

func TestIsCodexConnectorToolName(t *testing.T) {
	connector := []string{
		"codex_apps.github.get_repo",
		"codex_apps__github__get_repo",
		"codex_apps-github-get_repo",
		"CODEX_APPS.github",
		"codex_apps",
		"app/list",
		"mcpServerStatus/list",
		"list_connectors",
		"connectors/list",
		"codex_apps_github_get_repo",
	}
	for _, name := range connector {
		require.Truef(t, isCodexConnectorToolName(name), "expected %q to be a connector tool", name)
	}

	regular := []string{
		"",
		"shell",
		"apply_patch",
		"web_search",
		"get_weather",
		"codexapps",
		"my_codex_apps",
	}
	for _, name := range regular {
		require.Falsef(t, isCodexConnectorToolName(name), "expected %q not to be a connector tool", name)
	}
}

func TestStripCodexConnectorToolsRemovesConnectorKeepsRegular(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{"type": "function", "name": "apply_patch"},
			map[string]any{"type": "function", "name": "codex_apps.github.get_repo"},
			map[string]any{"type": "function", "function": map[string]any{"name": "codex_apps.gmail.search"}},
			map[string]any{"type": "function", "name": "shell"},
		},
	}

	removed := stripCodexConnectorTools(reqBody)
	require.ElementsMatch(t, []string{"codex_apps.github.get_repo", "codex_apps.gmail.search"}, removed)
	require.ElementsMatch(t, []string{"apply_patch", "shell"}, codexConnectorToolNames(t, reqBody))
}

func TestStripCodexConnectorToolsResetsToolChoicePinningRemovedTool(t *testing.T) {
	reqBody := map[string]any{
		"tools": []any{
			map[string]any{"type": "function", "name": "codex_apps.github.get_repo"},
			map[string]any{"type": "function", "name": "apply_patch"},
		},
		"tool_choice": map[string]any{"type": "function", "name": "codex_apps.github.get_repo"},
	}

	removed := stripCodexConnectorTools(reqBody)
	require.Equal(t, []string{"codex_apps.github.get_repo"}, removed)
	require.Equal(t, "auto", reqBody["tool_choice"])
}

func TestApplyCodexOAuthTransformBlockConnectorToolsOption(t *testing.T) {
	makeBody := func() map[string]any {
		return map[string]any{
			"model": "gpt-5.4",
			"tools": []any{
				map[string]any{"type": "function", "name": "codex_apps.github.get_repo"},
				map[string]any{"type": "function", "name": "apply_patch"},
			},
		}
	}

	bodyDefault := makeBody()
	applyCodexOAuthTransform(bodyDefault, false, false)
	require.ElementsMatch(t, []string{"codex_apps.github.get_repo", "apply_patch"}, codexConnectorToolNames(t, bodyDefault))

	bodyBlocked := makeBody()
	applyCodexOAuthTransformWithOptions(bodyBlocked, codexOAuthTransformOptions{BlockConnectorTools: true})
	require.ElementsMatch(t, []string{"apply_patch"}, codexConnectorToolNames(t, bodyBlocked))
}
