package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRiskKnowledgeBaseMigrationIsVersionedAndShadowOnly(t *testing.T) {
	content, err := FS.ReadFile("220_risk_knowledge_base.sql")
	require.NoError(t, err)
	sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))

	require.Contains(t, sql, "create table if not exists risk_engine_knowledge_state")
	require.Contains(t, sql, "create table if not exists risk_engine_knowledge_entries")
	require.Contains(t, sql, "knowledge_version integer not null default 0")
	require.Contains(t, sql, "knowledge_match_ids bigint[] not null")
	require.Contains(t, sql, "'cheat_development'")
	require.Contains(t, sql, "'benign_research'")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "update api_keys")
	require.NotContains(t, sql, "user_blocked_groups")
	require.NotContains(t, sql, "content_moderation_user_group_penalties")
}
