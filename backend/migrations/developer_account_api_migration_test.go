package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeveloperAccountAPIMigrationUsesHashedScopedTokens(t *testing.T) {
	content, err := FS.ReadFile("214_developer_account_api.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS developer_api_enabled BOOLEAN NOT NULL DEFAULT FALSE")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS developer_tokens")
	require.Contains(t, sql, "user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE")
	require.Contains(t, sql, "token_hash VARCHAR(64) NOT NULL")
	require.Contains(t, sql, "scopes JSONB NOT NULL DEFAULT '[]'::jsonb")
	require.Contains(t, sql, "CREATE UNIQUE INDEX IF NOT EXISTS developer_tokens_token_hash_key")
	require.NotContains(t, sql, "plaintext_token")
}
