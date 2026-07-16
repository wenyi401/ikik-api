package service

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCodexVersionConstantsConsistency(t *testing.T) {
	require.Equal(t, codexCLIVersion, openAICodexProbeVersion)
	require.Contains(t, codexCLIUserAgent, "codex_cli_rs/"+codexCLIVersion)
	require.True(t, strings.Contains(codexCLIUserAgent, codexCLIVersion))
}
