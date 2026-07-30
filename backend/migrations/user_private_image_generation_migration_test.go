package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMigration204EnablesImageGenerationOnlyForOpenAIAndGrokUserPrivateGroups(t *testing.T) {
	content, err := FS.ReadFile("204_enable_user_private_image_generation.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "WITH updated_groups AS ( UPDATE groups SET allow_image_generation = true")
	require.Contains(t, sql, "WHERE scope = 'user_private' AND platform IN ('openai', 'grok') AND allow_image_generation = false RETURNING id ), affected_api_keys AS (")
	require.Contains(t, sql, "JOIN updated_groups AS g ON g.id = k.group_id")
	require.Contains(t, sql, "JOIN api_key_group_routes AS r ON r.api_key_id = k.id JOIN updated_groups AS g ON g.id = r.group_id")
	require.Contains(t, sql, "INSERT INTO auth_cache_invalidation_outbox (cache_key) SELECT DISTINCT encode(sha256(convert_to(key, 'UTF8')), 'hex') FROM affected_api_keys;")
}
