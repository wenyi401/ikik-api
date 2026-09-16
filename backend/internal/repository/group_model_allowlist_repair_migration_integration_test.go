//go:build integration

package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/require"

	dbmigrations "ikik-api/migrations"
)

const groupModelAllowlistRepairMigration = "236_group_model_allowlist_repair.sql"

// 236 是可重放的修复迁移（issue #6780）：235 只负责新建 model_allowlist，一旦被
// 记账就不会重跑，数据库若回到旧结构（手工删列、按旧结构部分恢复）应用仍能启动，
// 但关联 groups 的查询会报 column groups.model_allowlist does not exist。
//
// ikik 双特性并存（与 235 的注释一致）：model_allowlist 是上游的分组级准入白名单，
// models_list_config 是 ikik 的 /v1/models 展示列表——**本修复只补建缺失的新列，
// 绝不重命名也绝不回填 models_list_config**。本用例固化这一点。
func TestMigration236RecreatesMissingColumnWithoutTouchingDisplayList(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	_, err := tx.ExecContext(ctx, "ALTER TABLE groups DROP COLUMN model_allowlist")
	require.NoError(t, err)

	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO groups (name, platform, rate_multiplier, status, models_list_config)
VALUES ('migration-236-rename', 'anthropic', 1, 'active', '{"enabled":true,"models":["claude-sonnet-5"]}'::jsonb)
RETURNING id
`).Scan(&groupID))

	applyGroupModelAllowlistRepair(ctx, t, tx)

	// 新列按默认值补建（不承接展示列表的数据），展示列表列保持原样。
	var allowlist, displayList string
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT model_allowlist::text, models_list_config::text FROM groups WHERE id = $1", groupID).
		Scan(&allowlist, &displayList))
	require.JSONEq(t, `{}`, allowlist)
	require.JSONEq(t, `{"enabled":true,"models":["claude-sonnet-5"]}`, displayList)
	requireModelAllowlistColumnShape(ctx, t, tx)

	// 可重放：重复执行不报错也不改变结果。
	applyGroupModelAllowlistRepair(ctx, t, tx)
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT model_allowlist::text, models_list_config::text FROM groups WHERE id = $1", groupID).
		Scan(&allowlist, &displayList))
	require.JSONEq(t, `{}`, allowlist)
	require.JSONEq(t, `{"enabled":true,"models":["claude-sonnet-5"]}`, displayList)
}

// 与上一用例配套：两列同时存在时，修复迁移不得把展示列表(models_list_config)
// 的内容回填进准入白名单(model_allowlist)——回填会让历史展示配置意外变成
// 「准入白名单」把请求挡在门外。官方在 236 里做重命名+回填，ikik 明确不做。
func TestMigration236DoesNotBackfillFromModelsListConfig(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	// ikik 的 models_list_config 是基线列（并非上游 235 重命名出来的），直接使用。

	// 空白的准入白名单：不得被展示列表的内容回填，保持 '{}'（不启用准入）。
	var staleID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO groups (name, platform, rate_multiplier, status, model_allowlist, models_list_config)
VALUES ('migration-236-backfill', 'anthropic', 1, 'active', '{}'::jsonb, '{"enabled":true,"models":["legacy-model"]}'::jsonb)
RETURNING id
`).Scan(&staleID))

	// 已有准入配置：同样不受展示列表影响。
	var currentID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO groups (name, platform, rate_multiplier, status, model_allowlist, models_list_config)
VALUES ('migration-236-keep', 'anthropic', 1, 'active', '{"enabled":true,"models":["current-model"]}'::jsonb, '{"enabled":true,"models":["legacy-model"]}'::jsonb)
RETURNING id
`).Scan(&currentID))

	applyGroupModelAllowlistRepair(ctx, t, tx)

	var backfilled, kept, displayList string
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT model_allowlist::text, models_list_config::text FROM groups WHERE id = $1", staleID).
		Scan(&backfilled, &displayList))
	require.JSONEq(t, `{}`, backfilled)
	require.JSONEq(t, `{"enabled":true,"models":["legacy-model"]}`, displayList)
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT model_allowlist::text FROM groups WHERE id = $1", currentID).Scan(&kept))
	require.JSONEq(t, `{"enabled":true,"models":["current-model"]}`, kept)
}

func TestMigration236RecreatesMissingModelAllowlistColumn(t *testing.T) {
	tx := testTx(t)
	ctx := context.Background()

	_, err := tx.ExecContext(ctx, "ALTER TABLE groups DROP COLUMN model_allowlist")
	require.NoError(t, err)

	var groupID int64
	require.NoError(t, tx.QueryRowContext(ctx, `
INSERT INTO groups (name, platform, rate_multiplier, status)
VALUES ('migration-236-recreate', 'anthropic', 1, 'active')
RETURNING id
`).Scan(&groupID))

	applyGroupModelAllowlistRepair(ctx, t, tx)

	var allowlist string
	require.NoError(t, tx.QueryRowContext(ctx,
		"SELECT model_allowlist::text FROM groups WHERE id = $1", groupID).Scan(&allowlist))
	require.JSONEq(t, `{}`, allowlist)
	requireModelAllowlistColumnShape(ctx, t, tx)
}

func applyGroupModelAllowlistRepair(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()

	migrationSQL, err := dbmigrations.FS.ReadFile(groupModelAllowlistRepairMigration)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, string(migrationSQL))
	require.NoError(t, err)
}

func requireModelAllowlistColumnShape(ctx context.Context, t *testing.T, tx *sql.Tx) {
	t.Helper()

	var isNullable, columnDefault string
	require.NoError(t, tx.QueryRowContext(ctx, `
SELECT is_nullable, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_name = 'groups' AND column_name = 'model_allowlist'
`).Scan(&isNullable, &columnDefault))
	require.Equal(t, "NO", isNullable)
	require.Contains(t, columnDefault, "'{}'::jsonb")
}
