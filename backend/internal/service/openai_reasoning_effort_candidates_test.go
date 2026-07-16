package service

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractOpenAIReasoningEffortFromBodyModelCandidates(t *testing.T) {
	body := []byte(`{"model":"gpt-5.6-sol","input":"hello"}`)

	got := extractOpenAIReasoningEffortFromBody(body, "gpt-5.6-sol", "gpt-5.6-sol-max")

	require.NotNil(t, got)
	require.Equal(t, "max", *got)
}

func TestExtractOpenAIReasoningEffortUsesMappedModelForExplicitMax(t *testing.T) {
	body := []byte(`{"model":"sol","reasoning":{"effort":"max"},"input":"hello"}`)

	got := extractOpenAIReasoningEffortFromBody(body, "gpt-5.6-sol", "sol")

	require.NotNil(t, got)
	require.Equal(t, "max", *got)
}
