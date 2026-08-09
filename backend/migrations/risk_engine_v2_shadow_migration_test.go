package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRiskEngineV2MigrationIsShadowOnlyAndHasNoPunishmentMutation(t *testing.T) {
	content, err := FS.ReadFile("218_risk_engine_v2_shadow.sql")
	require.NoError(t, err)
	sql := strings.ToLower(strings.Join(strings.Fields(string(content)), " "))

	require.Contains(t, sql, "create table if not exists risk_engine_v2_observations")
	require.Contains(t, sql, "create table if not exists risk_engine_v2_incidents")
	require.Contains(t, sql, "check (mode = 'shadow')")
	require.Contains(t, sql, "unique (request_id, input_hash, policy_version)")
	require.Contains(t, sql, "incident_fingerprint varchar(64) not null unique")
	require.NotContains(t, sql, "update users")
	require.NotContains(t, sql, "update api_keys")
	require.NotContains(t, sql, "user_blocked_groups")
	require.NotContains(t, sql, "content_moderation_user_group_penalties")
}
