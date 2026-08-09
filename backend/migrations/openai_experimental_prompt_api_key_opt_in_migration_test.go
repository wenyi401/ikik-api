package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIExperimentalPromptAPIKeyOptInMigrationIsFailClosed(t *testing.T) {
	content, err := FS.ReadFile("222_openai_experimental_prompt_api_key_opt_in.sql")
	require.NoError(t, err)
	sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))

	require.Contains(t, sql, "openai_experimental_prompt_enabled boolean not null default false")
}
