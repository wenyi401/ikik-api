package kiro

const DefaultTestModelID = "claude-sonnet-4-5-20250929"

// Model describes a Kiro model in OpenAI-compatible /models shape.
type Model struct {
	ID          string `json:"id"`
	Object      string `json:"object"`
	Created     int64  `json:"created,omitempty"`
	OwnedBy     string `json:"owned_by"`
	DisplayName string `json:"display_name,omitempty"`
}

var defaultModels = []Model{
	{ID: "claude-opus-4-8", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.8"},
	{ID: "claude-opus-4-8-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.8 Thinking"},
	{ID: "claude-opus-4-7", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.7"},
	{ID: "claude-opus-4-7-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.7 Thinking"},
	{ID: "claude-opus-4-6", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.6"},
	{ID: "claude-opus-4-6-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.6 Thinking"},
	{ID: "claude-sonnet-4-6", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Sonnet 4.6"},
	{ID: "claude-sonnet-4-6-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Sonnet 4.6 Thinking"},
	{ID: "claude-opus-4-5-20251101", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.5"},
	{ID: "claude-opus-4-5-20251101-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Opus 4.5 Thinking"},
	{ID: "claude-sonnet-4-5-20250929", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Sonnet 4.5"},
	{ID: "claude-sonnet-4-5-20250929-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Sonnet 4.5 Thinking"},
	{ID: "claude-haiku-4-5-20251001", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Haiku 4.5"},
	{ID: "claude-haiku-4-5-20251001-thinking", Object: "model", OwnedBy: "kiro", DisplayName: "Claude Haiku 4.5 Thinking"},
}

var defaultModelMapping = map[string]string{
	"claude-opus-4-8":                     "claude-opus-4.8",
	"claude-opus-4-8-thinking":            "claude-opus-4.8",
	"claude-opus-4-7":                     "claude-opus-4.7",
	"claude-opus-4-7-thinking":            "claude-opus-4.7",
	"claude-opus-4-6":                     "claude-opus-4.6",
	"claude-opus-4-6-thinking":            "claude-opus-4.6",
	"claude-sonnet-4-6":                   "claude-sonnet-4.6",
	"claude-sonnet-4-6-thinking":          "claude-sonnet-4.6",
	"claude-opus-4-5-20251101":            "claude-opus-4.5",
	"claude-opus-4-5-20251101-thinking":   "claude-opus-4.5",
	"claude-sonnet-4-5-20250929":          "claude-sonnet-4.5",
	"claude-sonnet-4-5-20250929-thinking": "claude-sonnet-4.5",
	"claude-haiku-4-5-20251001":           "claude-haiku-4.5",
	"claude-haiku-4-5-20251001-thinking":  "claude-haiku-4.5",
}

func DefaultModels() []Model {
	out := make([]Model, len(defaultModels))
	copy(out, defaultModels)
	return out
}

func DefaultModelIDs() []string {
	models := DefaultModels()
	ids := make([]string, 0, len(models))
	for _, model := range models {
		ids = append(ids, model.ID)
	}
	return ids
}

func DefaultModelMapping() map[string]string {
	mapping := make(map[string]string, len(defaultModelMapping))
	for from, to := range defaultModelMapping {
		mapping[from] = to
	}
	return mapping
}
