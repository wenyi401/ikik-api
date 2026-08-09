package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDeveloperAPIPublicMigrationEnablesExistingAndFutureUsers(t *testing.T) {
	content, err := FS.ReadFile("219_developer_api_public.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "ALTER COLUMN developer_api_enabled SET DEFAULT TRUE")
	require.Contains(t, sql, "UPDATE users SET developer_api_enabled = TRUE WHERE developer_api_enabled = FALSE")
}
