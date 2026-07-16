-- 允许用户平台维度配额记录 Kiro。
--
-- 部分部署分支没有 user_platform_quotas 表；全新库迁移时需要兼容缺表场景。
-- 如果该表存在，则扩展平台 check 以允许 platform='kiro'。

DO $$
BEGIN
  IF to_regclass('public.user_platform_quotas') IS NOT NULL THEN
    ALTER TABLE user_platform_quotas
      DROP CONSTRAINT IF EXISTS user_platform_quotas_platform_check;

    ALTER TABLE user_platform_quotas
      ADD CONSTRAINT user_platform_quotas_platform_check
      CHECK (platform IN ('anthropic', 'openai', 'gemini', 'antigravity', 'kiro'));
  END IF;
END $$;
