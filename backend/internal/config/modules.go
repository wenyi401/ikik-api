package config

import "fmt"

// normalizeModulesSubtree preserves dotted module IDs as complete keys.
func normalizeModulesSubtree(raw any) (map[string]map[string]any, error) {
	result := make(map[string]map[string]any)
	if raw == nil {
		return result, nil
	}
	top, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("modules: expected a mapping, got %T", raw)
	}
	for key, value := range top {
		if value == nil {
			result[key] = map[string]any{}
			continue
		}
		entry, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("modules.%s: expected a mapping, got %T", key, value)
		}
		result[key] = entry
	}
	return result, nil
}
