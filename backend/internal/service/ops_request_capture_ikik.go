package service

import (
	"encoding/json"
	"strings"
)

const opsMaxStoredRequestBodyBytes = 256 * 1024

// PrepareOpsRequestBodyForQueue sanitizes request JSON before it enters the
// asynchronous ops queue, preventing the queue from retaining large raw bodies.
func PrepareOpsRequestBodyForQueue(raw []byte) (requestBodyJSON *string, truncated bool, requestBodyBytes *int) {
	if len(raw) == 0 {
		return nil, false, nil
	}
	originalSize := len(raw)

	var decoded any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return nil, false, &originalSize
	}
	decoded = sanitizeOpsRequestJSONStrings(decoded)
	normalized, err := json.Marshal(decoded)
	if err != nil {
		return nil, false, &originalSize
	}

	stored, truncated, _ := sanitizeAndTrimJSONPayload(normalized, opsMaxStoredRequestBodyBytes)
	if stored == "" {
		return nil, truncated, &originalSize
	}
	if truncated {
		stored = renameOpsTruncationMarker(stored)
	}
	return &stored, truncated, &originalSize
}

func sanitizeOpsRequestJSONStrings(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			cleanKey := strings.ReplaceAll(key, "\x00", "")
			out[cleanKey] = sanitizeOpsRequestJSONStrings(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = sanitizeOpsRequestJSONStrings(item)
		}
		return out
	case string:
		return strings.ReplaceAll(typed, "\x00", "")
	default:
		return value
	}
}

func renameOpsTruncationMarker(stored string) string {
	var payload map[string]any
	if err := json.Unmarshal([]byte(stored), &payload); err != nil {
		return stored
	}
	if marker, ok := payload["payload_truncated"]; ok {
		delete(payload, "payload_truncated")
		payload["request_body_truncated"] = marker
	}
	encoded, err := json.Marshal(payload)
	if err != nil || len(encoded) > opsMaxStoredRequestBodyBytes {
		return `{"request_body_truncated":true}`
	}
	return string(encoded)
}
