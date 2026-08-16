package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAwesomeCodexPetCatalogMigration(t *testing.T) {
	content, err := FS.ReadFile("226_awesome_codex_pet_catalog.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "ADD COLUMN IF NOT EXISTS position_x DOUBLE PRECISION")
	require.Contains(t, sql, "ALTER COLUMN anchor SET DEFAULT 'bottom-right'")
	require.Contains(t, sql, "589cf957-d689-5829-9090-7d0140a24a26")
	require.Contains(t, sql, "chud-codex--jorge-cuevas90003")
	require.Equal(t, 193, strings.Count(sql, "https://raw.githubusercontent.com/legeling/awesome-codex-pet/main/pets/"))
}
