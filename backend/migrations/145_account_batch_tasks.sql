-- 145_account_batch_tasks.sql
-- Persistent async account batch tasks for long-running account operations.

CREATE TABLE IF NOT EXISTS account_batch_tasks (
    id BIGSERIAL PRIMARY KEY,
    scope VARCHAR(20) NOT NULL,
    operation VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    total INTEGER NOT NULL DEFAULT 0,
    processed INTEGER NOT NULL DEFAULT 0,
    success INTEGER NOT NULL DEFAULT 0,
    failed INTEGER NOT NULL DEFAULT 0,
    created_by BIGINT NOT NULL,
    owner_user_id BIGINT NULL,
    error_message TEXT NULL,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_batch_tasks_scope_check CHECK (scope IN ('admin', 'user')),
    CONSTRAINT account_batch_tasks_status_check CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'canceled')),
    CONSTRAINT account_batch_tasks_total_check CHECK (total >= 0),
    CONSTRAINT account_batch_tasks_processed_check CHECK (processed >= 0),
    CONSTRAINT account_batch_tasks_success_check CHECK (success >= 0),
    CONSTRAINT account_batch_tasks_failed_check CHECK (failed >= 0)
);

CREATE TABLE IF NOT EXISTS account_batch_task_items (
    id BIGSERIAL PRIMARY KEY,
    task_id BIGINT NOT NULL REFERENCES account_batch_tasks(id) ON DELETE CASCADE,
    account_id BIGINT NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    error_message TEXT NULL,
    result JSONB NOT NULL DEFAULT '{}'::jsonb,
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT account_batch_task_items_status_check CHECK (status IN ('pending', 'running', 'succeeded', 'failed', 'canceled'))
);

CREATE INDEX IF NOT EXISTS idx_account_batch_tasks_status_created_at
    ON account_batch_tasks(status, created_at ASC);

CREATE INDEX IF NOT EXISTS idx_account_batch_tasks_created_by_created_at
    ON account_batch_tasks(created_by, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_account_batch_tasks_owner_user_created_at
    ON account_batch_tasks(owner_user_id, created_at DESC)
    WHERE owner_user_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_account_batch_task_items_task_id_id
    ON account_batch_task_items(task_id, id ASC);

CREATE INDEX IF NOT EXISTS idx_account_batch_task_items_task_status
    ON account_batch_task_items(task_id, status);
