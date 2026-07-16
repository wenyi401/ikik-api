CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_owned_openai_chatgpt_account_id_uniq
    ON accounts (owner_user_id, NULLIF(BTRIM(credentials->>'chatgpt_account_id'), ''))
    WHERE deleted_at IS NULL
      AND owner_user_id IS NOT NULL
      AND platform = 'openai'
      AND type = 'oauth'
      AND NULLIF(BTRIM(credentials->>'chatgpt_account_id'), '') IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_owned_openai_chatgpt_user_id_uniq
    ON accounts (owner_user_id, NULLIF(BTRIM(credentials->>'chatgpt_user_id'), ''))
    WHERE deleted_at IS NULL
      AND owner_user_id IS NOT NULL
      AND platform = 'openai'
      AND type = 'oauth'
      AND NULLIF(BTRIM(credentials->>'chatgpt_user_id'), '') IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_owned_anthropic_org_account_uniq
    ON accounts (
        owner_user_id,
        LOWER(COALESCE(NULLIF(BTRIM(extra->>'org_uuid'), ''), NULLIF(BTRIM(credentials->>'org_uuid'), ''))),
        LOWER(COALESCE(NULLIF(BTRIM(extra->>'account_uuid'), ''), NULLIF(BTRIM(credentials->>'account_uuid'), '')))
    )
    WHERE deleted_at IS NULL
      AND owner_user_id IS NOT NULL
      AND platform = 'anthropic'
      AND type = 'oauth'
      AND COALESCE(NULLIF(BTRIM(extra->>'org_uuid'), ''), NULLIF(BTRIM(credentials->>'org_uuid'), '')) IS NOT NULL
      AND COALESCE(NULLIF(BTRIM(extra->>'account_uuid'), ''), NULLIF(BTRIM(credentials->>'account_uuid'), '')) IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_owned_gemini_project_uniq
    ON accounts (
        owner_user_id,
        LOWER(COALESCE(NULLIF(BTRIM(credentials->>'oauth_type'), ''), 'code_assist')),
        LOWER(NULLIF(BTRIM(credentials->>'project_id'), ''))
    )
    WHERE deleted_at IS NULL
      AND owner_user_id IS NOT NULL
      AND platform = 'gemini'
      AND type = 'oauth'
      AND NULLIF(BTRIM(credentials->>'project_id'), '') IS NOT NULL;

CREATE UNIQUE INDEX CONCURRENTLY IF NOT EXISTS idx_accounts_owned_antigravity_project_uniq
    ON accounts (owner_user_id, LOWER(NULLIF(BTRIM(credentials->>'project_id'), '')))
    WHERE deleted_at IS NULL
      AND owner_user_id IS NOT NULL
      AND platform = 'antigravity'
      AND type = 'oauth'
      AND NULLIF(BTRIM(credentials->>'project_id'), '') IS NOT NULL;
