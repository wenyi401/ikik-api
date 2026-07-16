package geminicli

import "testing"

func TestDefaultModels_IncludesImageModels(t *testing.T) {
	t.Parallel()

	byID := make(map[string]Model, len(DefaultModels))
	for _, model := range DefaultModels {
		byID[model.ID] = model
	}

	required := []string{
		"gemini-3.5-flash",
		"gemini-2.5-flash-image",
		"gemini-3.1-flash-image",
	}

	for _, id := range required {
		model, ok := byID[id]
		if !ok {
			t.Fatalf("expected curated Gemini image model %q to exist", id)
		}
		if model.DisplayName == "" {
			t.Fatalf("expected curated Gemini image model %q to have a display name", id)
		}
	}
}
