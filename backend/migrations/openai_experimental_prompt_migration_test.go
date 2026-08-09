package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenAIExperimentalPromptMigrationIsFailClosed(t *testing.T) {
	content, err := FS.ReadFile("221_openai_experimental_prompt.sql")
	require.NoError(t, err)
	sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))

	require.Contains(t, sql, "openai_experimental_prompt_enabled boolean not null default false")
	require.Contains(t, sql, "openai_experimental_prompt_unlocked boolean not null default false")
	require.Contains(t, sql, "feature_key varchar(80) not null default ''")
}
