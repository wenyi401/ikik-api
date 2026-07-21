package config

import "strings"

const (
	OpenAIImagesResponsesReasoningEffortLow     = "low"
	OpenAIImagesResponsesReasoningEffortMedium  = "medium"
	OpenAIImagesResponsesReasoningEffortHigh    = "high"
	OpenAIImagesResponsesReasoningEffortXHigh   = "xhigh"
	OpenAIImagesResponsesReasoningEffortDefault = OpenAIImagesResponsesReasoningEffortMedium
)

func IsValidOpenAIImagesResponsesReasoningEffort(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case OpenAIImagesResponsesReasoningEffortLow,
		OpenAIImagesResponsesReasoningEffortMedium,
		OpenAIImagesResponsesReasoningEffortHigh,
		OpenAIImagesResponsesReasoningEffortXHigh:
		return true
	default:
		return false
	}
}

func NormalizeOpenAIImagesResponsesReasoningEffort(raw string) string {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if IsValidOpenAIImagesResponsesReasoningEffort(normalized) {
		return normalized
	}
	return OpenAIImagesResponsesReasoningEffortDefault
}
