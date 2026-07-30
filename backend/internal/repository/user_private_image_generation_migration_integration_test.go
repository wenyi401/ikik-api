//go:build integration

package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"ikik-api/migrations"
)

func TestMigration204BackfillsOnlyOpenAIAndGrokUserPrivateGroups(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	suffix := time.Now().UnixNano()

	var userID int64
	err := tx.QueryRowContext(ctx, `
		INSERT INTO users (email, password_hash, status)
		VALUES ($1, 'migration-204-test-hash', 'active')
		RETURNING id`, fmt.Sprintf("migration-204-%d@example.com", suffix)).Scan(&userID)
	require.NoError(t, err)

	testCases := []struct {
		name             string
		platform         string
		scope            string
		wantEnabled      bool
		wantInvalidation bool
	}{
		{name: "private-openai", platform: "openai", scope: "user_private", wantEnabled: true, wantInvalidation: true},
		{name: "private-grok", platform: "grok", scope: "user_private", wantEnabled: true, wantInvalidation: true},
		{name: "public-openai", platform: "openai", scope: "public", wantEnabled: false, wantInvalidation: false},
		{name: "private-anthropic", platform: "anthropic", scope: "user_private", wantEnabled: false, wantInvalidation: false},
	}

	type fixture struct {
		groupID  int64
		cacheKey string
	}
	fixtures := make([]fixture, len(testCases))
	for i, testCase := range testCases {
		var ownerUserID any
		if testCase.scope == "user_private" {
			ownerUserID = userID
		}

		groupName := fmt.Sprintf("migration-204-%s-%d", testCase.name, suffix)
		err = tx.QueryRowContext(ctx, `
			INSERT INTO groups (name, platform, scope, owner_user_id, allow_image_generation)
			VALUES ($1, $2, $3, $4, false)
			RETURNING id`, groupName, testCase.platform, testCase.scope, ownerUserID).Scan(&fixtures[i].groupID)
		require.NoError(t, err)

		rawKey := fmt.Sprintf("sk-migration-204-%d-%d", suffix, i)
		_, err = tx.ExecContext(ctx, `
			INSERT INTO api_keys (user_id, key, name, group_id, status)
			VALUES ($1, $2, $3, $4, 'active')`, userID, rawKey, testCase.name, fixtures[i].groupID)
		require.NoError(t, err)

		digest := sha256.Sum256([]byte(rawKey))
		fixtures[i].cacheKey = hex.EncodeToString(digest[:])
		_, err = tx.ExecContext(ctx,
			"DELETE FROM auth_cache_invalidation_outbox WHERE cache_key = $1", fixtures[i].cacheKey)
		require.NoError(t, err)
	}

	migrationSQL, err := migrations.FS.ReadFile("204_enable_user_private_image_generation.sql")
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)

	for i, testCase := range testCases {
		var enabled bool
		err = tx.QueryRowContext(ctx,
			"SELECT allow_image_generation FROM groups WHERE id = $1", fixtures[i].groupID).Scan(&enabled)
		require.NoError(t, err)
		require.Equal(t, testCase.wantEnabled, enabled, testCase.name)

		var invalidations int
		err = tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM auth_cache_invalidation_outbox WHERE cache_key = $1", fixtures[i].cacheKey).Scan(&invalidations)
		require.NoError(t, err)
		if testCase.wantInvalidation {
			require.Equal(t, 1, invalidations, testCase.name)
		} else {
			require.Zero(t, invalidations, testCase.name)
		}
	}
}
