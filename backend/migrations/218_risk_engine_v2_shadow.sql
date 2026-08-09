CREATE TABLE IF NOT EXISTS risk_engine_v2_observations (
    id                     BIGSERIAL PRIMARY KEY,
    request_id             VARCHAR(128) NOT NULL,
    user_id                BIGINT REFERENCES users(id) ON DELETE SET NULL,
    group_id               BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    conversation_id_hash   VARCHAR(64) NOT NULL DEFAULT '',
    input_hash             VARCHAR(64) NOT NULL,
    incident_fingerprint   VARCHAR(64) NOT NULL,
    candidate              JSONB NOT NULL DEFAULT '{}'::jsonb,
    adjudication           JSONB NOT NULL DEFAULT '{}'::jsonb,
    recommendation         VARCHAR(32) NOT NULL,
    would_protect          BOOLEAN NOT NULL DEFAULT FALSE,
    would_strike           BOOLEAN NOT NULL DEFAULT FALSE,
    reason_code            VARCHAR(96) NOT NULL DEFAULT '',
    policy_version         INTEGER NOT NULL,
    adjudicator_model      VARCHAR(128) NOT NULL DEFAULT '',
    mode                   VARCHAR(16) NOT NULL DEFAULT 'shadow',
    review_status          VARCHAR(24) NOT NULL DEFAULT 'unreviewed',
    review_label           JSONB,
    reviewed_by            BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at            TIMESTAMPTZ,
    review_note            VARCHAR(500) NOT NULL DEFAULT '',
    observed_at            TIMESTAMPTZ NOT NULL,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT risk_engine_v2_observation_input_hash_check
        CHECK (input_hash ~ '^[0-9a-f]{64}$'),
    CONSTRAINT risk_engine_v2_observation_incident_hash_check
        CHECK (incident_fingerprint ~ '^[0-9a-f]{64}$'),
    CONSTRAINT risk_engine_v2_observation_mode_check
        CHECK (mode = 'shadow'),
    CONSTRAINT risk_engine_v2_observation_review_status_check
        CHECK (review_status IN ('unreviewed', 'confirmed', 'false_positive', 'inconclusive')),
    CONSTRAINT risk_engine_v2_observation_idempotency
        UNIQUE (request_id, input_hash, policy_version)
);

CREATE TABLE IF NOT EXISTS risk_engine_v2_incidents (
    id                     BIGSERIAL PRIMARY KEY,
    incident_fingerprint   VARCHAR(64) NOT NULL UNIQUE,
    user_id                BIGINT REFERENCES users(id) ON DELETE SET NULL,
    group_id               BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    category               VARCHAR(64) NOT NULL,
    observation_count      INTEGER NOT NULL DEFAULT 1,
    first_observation_id   BIGINT NOT NULL REFERENCES risk_engine_v2_observations(id) ON DELETE CASCADE,
    latest_observation_id  BIGINT NOT NULL REFERENCES risk_engine_v2_observations(id) ON DELETE CASCADE,
    first_seen_at          TIMESTAMPTZ NOT NULL,
    last_seen_at           TIMESTAMPTZ NOT NULL,
    review_status          VARCHAR(24) NOT NULL DEFAULT 'unreviewed',
    created_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT risk_engine_v2_incident_hash_check
        CHECK (incident_fingerprint ~ '^[0-9a-f]{64}$'),
    CONSTRAINT risk_engine_v2_incident_category_check
        CHECK (category IN ('cheat_automation', 'auth_reverse_engineering',
            'exploit_reverse_engineering', 'credential_theft', 'safety_bypass',
            'account_automation', 'cyber_abuse')),
    CONSTRAINT risk_engine_v2_incident_count_check CHECK (observation_count >= 1),
    CONSTRAINT risk_engine_v2_incident_review_status_check
        CHECK (review_status IN ('unreviewed', 'confirmed', 'false_positive', 'inconclusive'))
);

CREATE INDEX IF NOT EXISTS idx_risk_engine_v2_observations_created
    ON risk_engine_v2_observations(created_at DESC, id DESC);
CREATE INDEX IF NOT EXISTS idx_risk_engine_v2_observations_review
    ON risk_engine_v2_observations(review_status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_engine_v2_observations_category
    ON risk_engine_v2_observations((adjudication->>'category'), created_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_engine_v2_observations_user
    ON risk_engine_v2_observations(user_id, created_at DESC) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_risk_engine_v2_incidents_subject
    ON risk_engine_v2_incidents(user_id, group_id, category, last_seen_at DESC);
CREATE INDEX IF NOT EXISTS idx_risk_engine_v2_incidents_review
    ON risk_engine_v2_incidents(review_status, last_seen_at DESC);
