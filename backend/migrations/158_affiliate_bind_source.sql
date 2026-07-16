ALTER TABLE user_affiliates
    ADD COLUMN IF NOT EXISTS invite_bind_source VARCHAR(20) NULL;

COMMENT ON COLUMN user_affiliates.invite_bind_source IS '邀请关系绑定来源：registration=注册绑定，admin=管理员手动绑定；历史旧数据无法可靠反推时为 NULL';

CREATE INDEX IF NOT EXISTS idx_user_affiliates_inviter_source
    ON user_affiliates (inviter_id, invite_bind_source)
    WHERE inviter_id IS NOT NULL;
