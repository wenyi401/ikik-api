-- 236: 收敛 groups.model_allowlist 列，修复 235 未落地却已被记账的实例。
--
-- 235 只在 model_allowlist 不存在时新建列，而迁移一旦写入 schema_migrations 就会按
-- 「文件名 + checksum」整份跳过，不再重跑。因此只要数据库在 235 之后回到了旧结构
-- （例如手工删列，或用旧结构备份做了部分恢复），应用依旧能正常启动，但每个关联
-- groups 的查询都会失败：pq: column groups.model_allowlist does not exist。
--
-- 本迁移可重放，用 regclass 解析表（跟随 search_path），不再假定 schema 为 public：
-- model_allowlist 不存在 -> 按默认值补建。
-- 注意（ikik）：models_list_config 是 ikik 的独立展示列表列，此迁移绝不重命名或
-- 回填它，只负责确保 model_allowlist 存在且为 NOT NULL DEFAULT '{}'。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'groups'::regclass
          AND attname = 'model_allowlist'
          AND NOT attisdropped
    ) THEN
        ALTER TABLE groups ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}';
    END IF;
END
$$;
