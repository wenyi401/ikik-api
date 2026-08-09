package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUserShareCardCustomizationMigrationAddsValidatedProfileFields(t *testing.T) {
	content, err := FS.ReadFile("217_user_share_card_customization.sql")
	require.NoError(t, err)
	sql := strings.Join(strings.Fields(string(content)), " ")

	require.Contains(t, sql, "share_card_text varchar(80) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "share_card_text_color varchar(7) NOT NULL DEFAULT '#08775c'")
	require.Contains(t, sql, "CHECK (share_card_text_color ~ '^#[0-9a-fA-F]{6}$')")
}
