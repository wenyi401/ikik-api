-- 146_usage_cleanup_auto_retention.sql
-- Allow system-owned cleanup tasks for automatic usage log retention.

ALTER TABLE usage_cleanup_tasks
    ALTER COLUMN created_by DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS created_source VARCHAR(50) NOT NULL DEFAULT 'admin';

CREATE INDEX IF NOT EXISTS idx_usage_cleanup_tasks_created_source_created_at
    ON usage_cleanup_tasks(created_source, created_at DESC);
