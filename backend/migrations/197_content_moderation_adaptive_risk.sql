-- Adaptive content moderation user risk profiles and idempotent request events.

CREATE TABLE IF NOT EXISTS content_moderation_user_risk_profiles (
    user_id                 BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_requests          BIGINT NOT NULL DEFAULT 0,
    audited_requests        BIGINT NOT NULL DEFAULT 0,
    flagged_requests        BIGINT NOT NULL DEFAULT 0,
    risk_score              NUMERIC(7, 3) NOT NULL DEFAULT 0,
    risk_level              VARCHAR(32) NOT NULL DEFAULT 'new',
    manual_level            VARCHAR(32) NOT NULL DEFAULT 'auto',
    current_sample_rate     INT NOT NULL DEFAULT 100,
    last_category           VARCHAR(128) NOT NULL DEFAULT '',
    last_score_delta        NUMERIC(7, 3) NOT NULL DEFAULT 0,
    last_hit_at             TIMESTAMPTZ,
    last_audited_at         TIMESTAMPTZ,
    last_notified_at        TIMESTAMPTZ,
    score_updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_moderation_risk_score_check CHECK (risk_score >= 0 AND risk_score <= 100),
    CONSTRAINT content_moderation_sample_rate_check CHECK (current_sample_rate >= 0 AND current_sample_rate <= 100),
    CONSTRAINT content_moderation_risk_level_check CHECK (risk_level IN ('new', 'normal', 'trusted', 'watch', 'high', 'critical')),
    CONSTRAINT content_moderation_manual_level_check CHECK (manual_level IN ('auto', 'trusted', 'watch', 'high', 'critical'))
);

CREATE TABLE IF NOT EXISTS content_moderation_request_events (
    id                      BIGSERIAL PRIMARY KEY,
    request_id              VARCHAR(160) NOT NULL,
    user_id                 BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    api_key_id              BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    audited                 BOOLEAN NOT NULL DEFAULT FALSE,
    flagged                 BOOLEAN NOT NULL DEFAULT FALSE,
    severity                VARCHAR(16) NOT NULL DEFAULT 'none',
    category                VARCHAR(128) NOT NULL DEFAULT '',
    score_delta             NUMERIC(7, 3) NOT NULL DEFAULT 0,
    sample_rate             INT NOT NULL DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT content_moderation_request_event_unique UNIQUE (user_id, request_id)
);

CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_profiles_level_score
    ON content_moderation_user_risk_profiles(risk_level, risk_score DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_risk_profiles_last_hit
    ON content_moderation_user_risk_profiles(last_hit_at DESC NULLS LAST);
CREATE INDEX IF NOT EXISTS idx_content_moderation_request_events_created_at
    ON content_moderation_request_events(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_content_moderation_request_events_user_created_at
    ON content_moderation_request_events(user_id, created_at DESC);
