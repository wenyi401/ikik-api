package schema

import (
	"testing"

	"entgo.io/ent/entc/load"
	"github.com/stretchr/testify/require"
)

func TestDeveloperTokenSchemaAndPublicDefault(t *testing.T) {
	spec, err := (&load.Config{Path: "."}).Load()
	require.NoError(t, err)
	schemas := map[string]*load.Schema{}
	for _, loaded := range spec.Schemas {
		schemas[loaded.Name] = loaded
	}

	token := requireSchema(t, schemas, "DeveloperToken")
	requireSchemaFields(t, token,
		"user_id", "name", "token_prefix", "token_hash", "scopes", "status",
		"expires_at", "last_used_at", "last_used_ip", "deleted_at",
	)
	user := requireSchema(t, schemas, "User")
	field := requireSchemaField(t, user, "developer_api_enabled")
	require.True(t, field.Default)
	require.Equal(t, true, field.DefaultValue)
}
