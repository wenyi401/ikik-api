ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS input_source VARCHAR(64) NOT NULL DEFAULT 'unknown_input';

ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS client_ip VARCHAR(64) NOT NULL DEFAULT '';

ALTER TABLE content_moderation_logs
    ADD COLUMN IF NOT EXISTS user_agent TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_content_moderation_logs_input_source_created_at
    ON content_moderation_logs(input_source, created_at DESC);
