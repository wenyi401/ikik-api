package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentModerationGroupPenaltiesMigrationDefinesProgressiveIdempotentBlocks(t *testing.T) {
	content, err := FS.ReadFile("213_content_moderation_group_penalties.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS content_moderation_user_group_penalties")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS content_moderation_user_group_penalty_events")
	require.Contains(t, sql, "UNIQUE (user_id, group_id, request_id)")
	require.Contains(t, sql, "24 hours")
	require.Contains(t, sql, "36 hours")
	require.Contains(t, sql, "strike_count >= 1 AND strike_count <= 3")
	require.Contains(t, sql, "trg_content_moderation_group_penalty_auth_cache_invalidation")
	require.NotContains(t, sql, "INSERT INTO user_blocked_groups")
}
