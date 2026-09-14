package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGroupModelAllowlistRepairMigration(t *testing.T) {
	content, err := FS.ReadFile("236_group_model_allowlist_repair.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")

	// ikik 双特性并存：model_allowlist 缺失时补建，但绝不重命名/回填 ikik 的
	// models_list_config 展示列表列。
	require.Contains(t, sql, "ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}'")
	require.NotContains(t, sql, "RENAME COLUMN models_list_config")
	require.NotContains(t, sql, "SET model_allowlist = models_list_config")

	// 235 用 table_schema = 'public' 判定列是否存在，而 ALTER TABLE 走的是 search_path；
	// 修复迁移必须用 regclass 解析，两者才不会在非 public schema 上分叉。
	require.Contains(t, sql, "attrelid = 'groups'::regclass")
}
