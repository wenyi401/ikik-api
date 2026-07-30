-- Prompt-only adaptive audit state. This is deliberately separate from
-- content moderation so one policy cannot accidentally enforce the other.

CREATE TABLE IF NOT EXISTS prompt_audit_user_profiles (
    user_id                 BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    total_requests          BIGINT NOT NULL DEFAULT 0,
    remote_audits           BIGINT NOT NULL DEFAULT 0,
    flagged_requests        BIGINT NOT NULL DEFAULT 0,
    risk_score              NUMERIC(7, 3) NOT NULL DEFAULT 0,
    risk_level              VARCHAR(32) NOT NULL DEFAULT 'new',
    blocked                 BOOLEAN NOT NULL DEFAULT FALSE,
    current_sample_rate     INT NOT NULL DEFAULT 100,
    last_category           VARCHAR(128) NOT NULL DEFAULT '',
    last_hit_at             TIMESTAMPTZ,
    last_audited_at         TIMESTAMPTZ,
    blocked_at              TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT prompt_audit_profile_risk_score_check CHECK (risk_score >= 0 AND risk_score <= 100),
    CONSTRAINT prompt_audit_profile_sample_rate_check CHECK (current_sample_rate >= 0 AND current_sample_rate <= 100),
    CONSTRAINT prompt_audit_profile_risk_level_check CHECK (risk_level IN ('new', 'normal', 'trusted', 'watch', 'high', 'critical'))
);

CREATE TABLE IF NOT EXISTS prompt_audit_fingerprints (
    prompt_hash             VARCHAR(64) PRIMARY KEY,
    category                VARCHAR(128) NOT NULL DEFAULT 'jailbreak',
    confidence              NUMERIC(6, 5) NOT NULL DEFAULT 1,
    source_event_id         BIGINT REFERENCES prompt_audit_events(id) ON DELETE SET NULL,
    hit_count               BIGINT NOT NULL DEFAULT 0,
    last_hit_at             TIMESTAMPTZ,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT prompt_audit_fingerprint_hash_check CHECK (prompt_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT prompt_audit_fingerprint_confidence_check CHECK (confidence >= 0 AND confidence <= 1)
);

CREATE INDEX IF NOT EXISTS idx_prompt_audit_profiles_level
    ON prompt_audit_user_profiles(risk_level, blocked, risk_score DESC);
CREATE INDEX IF NOT EXISTS idx_prompt_audit_profiles_last_hit
    ON prompt_audit_user_profiles(last_hit_at DESC NULLS LAST);
