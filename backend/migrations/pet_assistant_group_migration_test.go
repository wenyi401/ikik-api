package migrations

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPetAssistantGroupMigration(t *testing.T) {
	content, err := FS.ReadFile("227_pet_assistant_group.sql")
	require.NoError(t, err)

	sql := string(content)
	require.Contains(t, sql, "assistant_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL")
}
