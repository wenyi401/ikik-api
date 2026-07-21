package service

import (
	"strings"
)

func (a *Account) GetClaudeAccountUUID() string {
	if v := strings.TrimSpace(a.GetExtraString("account_uuid")); v != "" {
		return v
	}
	return strings.TrimSpace(a.GetCredential("account_uuid"))
}
func (a *Account) GetClaudeOrgUUID() string {
	if v := strings.TrimSpace(a.GetExtraString("org_uuid")); v != "" {
		return v
	}
	return strings.TrimSpace(a.GetCredential("org_uuid"))
}

// IsFreeModelOpenAICompatible reports whether the account was created by the
// free-model user flow. Those providers expose OpenAI-compatible chat endpoints,
// not the OpenAI Responses API.
func (a *Account) IsFreeModelOpenAICompatible() bool {
	if a == nil || a.Platform != PlatformOpenAI || a.Type != AccountTypeAPIKey {
		return false
	}
	return strings.TrimSpace(a.GetExtraString("free_model_provider")) != ""
}

const (
	legacyOpenAIEndpointCapabilitiesCredentialKey = "endpoint_capabilities"
)

func parseOpenAIEndpointCapabilitySet(raw any) map[string]bool {
	result := make(map[string]bool)
	add := func(value string) {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" {
			return
		}
		result[value] = true
	}

	switch capabilities := raw.(type) {
	case []any:
		for _, item := range capabilities {
			if value, ok := item.(string); ok {
				add(value)
			}
		}
	case []string:
		for _, value := range capabilities {
			add(value)
		}
	case string:
		for _, value := range strings.Split(capabilities, ",") {
			add(value)
		}
	case map[string]any:
		for key, value := range capabilities {
			if enabled, ok := value.(bool); ok && enabled {
				add(key)
			}
		}
	case map[string]bool:
		for key, enabled := range capabilities {
			if enabled {
				add(key)
			}
		}
	}

	return result
}
