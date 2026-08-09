package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCarpoolSubscriptionsPermanentMigrationRepairsActiveMembers(t *testing.T) {
	content, err := FS.ReadFile("215_carpool_subscriptions_permanent.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "g.scope = 'user_carpool'")
	require.Contains(t, sql, "default_validity_days = 36500")
	require.Contains(t, sql, "expires_at = TIMESTAMPTZ '2099-12-31 23:59:59+00'")
	require.Contains(t, sql, "m.status = 'active'")
	require.Contains(t, sql, "p.status <> 'closed'")
}
