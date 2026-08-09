package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildGeminiAIStudioModelActionURL(t *testing.T) {
	const base = "https://generativelanguage.googleapis.com"

	got, err := buildGeminiAIStudioModelActionURL(base+"/", " gemini-2.5-flash ", "streamGenerateContent", true)
	require.NoError(t, err)
	require.Equal(t, base+"/v1beta/models/gemini-2.5-flash:streamGenerateContent?alt=sse", got)

	for _, model := range []string{"../../x/y", "..", ".", "gemini/../../x", "gemini?x=y", "gemini%2f..", "gemini pro", "gemini\x00pro"} {
		t.Run(model, func(t *testing.T) {
			_, err := buildGeminiAIStudioModelActionURL(base, model, "generateContent", false)
			require.Error(t, err)
			require.False(t, IsSafeGeminiModelPathSegment(model))
		})
	}

	_, err = buildGeminiAIStudioModelActionURL(base, "gemini-2.5-pro", "deleteModel", false)
	require.Error(t, err)
}
