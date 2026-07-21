package service

import "strings"

var codexConnectorToolPrefixes = []string{"codex_apps"}

var codexConnectorToolExactNames = map[string]struct{}{
	"app_list":             {},
	"apps_list":            {},
	"mcpserverstatus_list": {},
	"list_connectors":      {},
	"connectors_list":      {},
}

func normalizeCodexConnectorToolName(name string) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	if normalized == "" {
		return ""
	}
	return strings.NewReplacer(".", "_", "-", "_", "/", "_").Replace(normalized)
}

func isCodexConnectorToolName(name string) bool {
	normalized := normalizeCodexConnectorToolName(name)
	if normalized == "" {
		return false
	}
	if _, ok := codexConnectorToolExactNames[normalized]; ok {
		return true
	}
	for _, prefix := range codexConnectorToolPrefixes {
		if normalized == prefix || strings.HasPrefix(normalized, prefix+"_") {
			return true
		}
	}
	return false
}

func codexToolName(tool any) string {
	toolMap, ok := tool.(map[string]any)
	if !ok {
		return ""
	}
	if name, ok := toolMap["name"].(string); ok && strings.TrimSpace(name) != "" {
		return name
	}
	if function, ok := toolMap["function"].(map[string]any); ok {
		name, _ := function["name"].(string)
		return name
	}
	return ""
}

func stripCodexConnectorTools(reqBody map[string]any) []string {
	tools, ok := reqBody["tools"].([]any)
	if !ok {
		return nil
	}

	removed := make([]string, 0)
	kept := make([]any, 0, len(tools))
	for _, tool := range tools {
		name := codexToolName(tool)
		if name != "" && isCodexConnectorToolName(name) {
			removed = append(removed, name)
			continue
		}
		kept = append(kept, tool)
	}
	if len(removed) == 0 {
		return nil
	}

	reqBody["tools"] = kept
	resetCodexToolChoiceIfRemoved(reqBody, removed)
	return removed
}

func resetCodexToolChoiceIfRemoved(reqBody map[string]any, removed []string) {
	choice, ok := reqBody["tool_choice"].(map[string]any)
	if !ok {
		return
	}
	name, _ := choice["name"].(string)
	if strings.TrimSpace(name) == "" {
		if function, ok := choice["function"].(map[string]any); ok {
			name, _ = function["name"].(string)
		}
	}
	for _, removedName := range removed {
		if name == removedName {
			reqBody["tool_choice"] = "auto"
			return
		}
	}
}

func (s *OpenAIGatewayService) codexBlockConnectorTools() bool {
	return s != nil && s.cfg != nil && s.cfg.Gateway.CodexBlockConnectorTools
}
