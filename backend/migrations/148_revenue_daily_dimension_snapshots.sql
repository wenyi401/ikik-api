-- Daily revenue aggregate snapshots for revenue analytics after usage_logs retention cleanup.
-- Stores aggregate metrics only; request-level share ledger still comes from account_share_settlement_entries.
CREATE TABLE IF NOT EXISTS revenue_daily_dimension_snapshots (
    bucket_date             DATE NOT NULL,
    user_id                 BIGINT NOT NULL DEFAULT 0,
    account_id              BIGINT NOT NULL DEFAULT 0,
    group_id                BIGINT NOT NULL DEFAULT 0,
    owner_user_id           BIGINT NOT NULL DEFAULT 0,
    model                   VARCHAR(100) NOT NULL DEFAULT '',
    requested_model         VARCHAR(100) NOT NULL DEFAULT '',
    total_requests          BIGINT NOT NULL DEFAULT 0,
    total_tokens            BIGINT NOT NULL DEFAULT 0,
    standard_cost           NUMERIC(20, 10) NOT NULL DEFAULT 0,
    consumed_revenue        NUMERIC(20, 10) NOT NULL DEFAULT 0,
    account_cost            NUMERIC(20, 10) NOT NULL DEFAULT 0,
    share_consumer_charge   NUMERIC(20, 10) NOT NULL DEFAULT 0,
    share_account_cost      NUMERIC(20, 10) NOT NULL DEFAULT 0,
    share_owner_credit      NUMERIC(20, 10) NOT NULL DEFAULT 0,
    share_platform_fee      NUMERIC(20, 10) NOT NULL DEFAULT 0,
    computed_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (
        bucket_date,
        user_id,
        account_id,
        group_id,
        owner_user_id,
        model,
        requested_model
    )
);

CREATE INDEX IF NOT EXISTS idx_revenue_daily_snapshots_bucket_date
    ON revenue_daily_dimension_snapshots (bucket_date);

CREATE INDEX IF NOT EXISTS idx_revenue_daily_snapshots_user_date
    ON revenue_daily_dimension_snapshots (user_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_revenue_daily_snapshots_account_date
    ON revenue_daily_dimension_snapshots (account_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_revenue_daily_snapshots_group_date
    ON revenue_daily_dimension_snapshots (group_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_revenue_daily_snapshots_owner_date
    ON revenue_daily_dimension_snapshots (owner_user_id, bucket_date);

CREATE INDEX IF NOT EXISTS idx_revenue_daily_snapshots_model_date
    ON revenue_daily_dimension_snapshots (model, bucket_date);
