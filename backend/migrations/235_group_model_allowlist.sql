-- 235: 新增 groups.model_allowlist 分组级模型白名单列（同时约束模型列表接口与请求准入）。
--
-- ikik 说明：上游 sub2api 在此迁移中将 models_list_config 重命名为 model_allowlist
-- 并升级其语义；ikik 分支同时保留两个特性：
--   * model_allowlist  —— 上游 v0.2.x 的分组级准入白名单（本迁移新建，默认 '{}' 不启用）；
--   * models_list_config —— ikik 自定义 /v1/models 展示列表（原列原数据，保持不动）。
-- 因此本迁移只新建 model_allowlist 列，绝不重命名/回填 models_list_config，
-- 升级后现有分组的展示列表行为不变，准入白名单默认关闭。
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM information_schema.columns
        WHERE table_schema = 'public'
          AND table_name = 'groups'
          AND column_name = 'model_allowlist'
    ) THEN
        ALTER TABLE groups ADD COLUMN model_allowlist JSONB NOT NULL DEFAULT '{}';
    END IF;
END
$$;

COMMENT ON COLUMN groups.model_allowlist IS
    'Group model allowlist: constrains both model listing responses and request admission';
