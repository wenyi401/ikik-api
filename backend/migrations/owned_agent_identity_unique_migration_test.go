package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration205UsesTeamAndUserForOwnedAgentIdentityUniqueness(t *testing.T) {
	content, err := FS.ReadFile("205_fix_owned_openai_agent_identity_unique_notx.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "DROP INDEX CONCURRENTLY IF EXISTS idx_accounts_owned_openai_chatgpt_account_id_uniq")
	require.Contains(t, sql, "DROP INDEX CONCURRENTLY IF EXISTS idx_accounts_owned_openai_chatgpt_user_id_uniq")
	require.Contains(t, sql, "CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_owned_openai_agent_identity_uniq ON accounts ( owner_user_id, NULLIF(BTRIM(credentials->>'chatgpt_account_id'), ''), NULLIF(BTRIM(credentials->>'chatgpt_user_id'), '') )")
	require.Contains(t, sql, "LOWER(NULLIF(BTRIM(credentials->>'auth_mode'), '')) = 'agentidentity'")
	require.Contains(t, sql, "COALESCE(LOWER(NULLIF(BTRIM(credentials->>'auth_mode'), '')), '') <> 'agentidentity'")
	require.Contains(t, sql, "NULLIF(BTRIM(credentials->>'chatgpt_account_id'), '') IS NOT NULL AND NULLIF(BTRIM(credentials->>'chatgpt_user_id'), '') IS NULL AND NULLIF(BTRIM(credentials->>'email'), '') IS NULL")
}
