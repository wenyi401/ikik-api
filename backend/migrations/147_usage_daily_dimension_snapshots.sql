-- Daily aggregate snapshots for historical usage lookups after usage_logs retention cleanup.
-- This table intentionally stores only aggregate metrics and filter dimensions, not request details.
CREATE TABLE IF NOT EXISTS usage_daily_dimension_snapshots (
    bucket_date             DATE NOT NULL,
    user_id                 BIGINT NOT NULL DEFAULT 0,
    api_key_id              BIGINT NOT NULL DEFAULT 0,
    account_id              BIGINT NOT NULL DEFAULT 0,
    group_id                BIGINT NOT NULL DEFAULT 0,
    model                   VARCHAR(100) NOT NULL DEFAULT '',
    requested_model         VARCHAR(100) NOT NULL DEFAULT '',
    upstream_model          VARCHAR(100) NOT NULL DEFAULT '',
    model_mapping_chain     VARCHAR(500) NOT NULL DEFAULT '',
    request_type            SMALLINT NOT NULL DEFAULT 0,
    stream_state            SMALLINT NOT NULL DEFAULT 0,
    billing_type            SMALLINT NOT NULL DEFAULT -1,
    billing_mode            VARCHAR(20) NOT NULL DEFAULT '',
    total_requests          BIGINT NOT NULL DEFAULT 0,
    input_tokens            BIGINT NOT NULL DEFAULT 0,
    output_tokens           BIGINT NOT NULL DEFAULT 0,
    cache_creation_tokens   BIGINT NOT NULL DEFAULT 0,
    cache_read_tokens       BIGINT NOT NULL DEFAULT 0,
    total_cost              NUMERIC(20, 10) NOT NULL DEFAULT 0,
    actual_cost             NUMERIC(20, 10) NOT NULL DEFAULT 0,
    account_cost            NUMERIC(20, 10) NOT NULL DEFAULT 0,
    total_duration_ms       BIGINT NOT NULL DEFAULT 0,
    computed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (
        bucket_date,
        user_id,
        api_key_id,
        account_id,
        group_id,
        model,
        requested_model,
        upstream_model,
        model_mapping_chain,
        request_type,
        stream_state,
        billing_type,
        billing_mode
    )
);

CREATE INDEX IF NOT EXISTS idx_usage_daily_snapshots_bucket_date
    ON usage_daily_dimension_snapshots (bucket_date);

CREATE INDEX IF NOT EXISTS idx_usage_daily_snapshots_user_date
    ON usage_daily_dimension_snapshots (user_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_usage_daily_snapshots_api_key_date
    ON usage_daily_dimension_snapshots (api_key_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_usage_daily_snapshots_account_date
    ON usage_daily_dimension_snapshots (account_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_usage_daily_snapshots_group_date
    ON usage_daily_dimension_snapshots (group_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_usage_daily_snapshots_model_date
    ON usage_daily_dimension_snapshots (model, bucket_date);
