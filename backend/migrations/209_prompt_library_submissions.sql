CREATE TABLE IF NOT EXISTS prompt_library_submissions (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(200) NOT NULL,
    description VARCHAR(500) NOT NULL DEFAULT '',
    content TEXT NOT NULL,
    type VARCHAR(16) NOT NULL DEFAULT 'TEXT',
    category VARCHAR(32) NOT NULL DEFAULT 'other',
    media_url TEXT NOT NULL DEFAULT '',
    status VARCHAR(16) NOT NULL DEFAULT 'pending',
    review_note VARCHAR(500) NOT NULL DEFAULT '',
    reviewed_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    reviewed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT prompt_library_submissions_type_check
        CHECK (type IN ('TEXT', 'STRUCTURED', 'IMAGE', 'VIDEO', 'AUDIO')),
    CONSTRAINT prompt_library_submissions_category_check
        CHECK (category IN ('coding', 'writing', 'business', 'creative', 'education', 'workflow', 'productivity', 'other')),
    CONSTRAINT prompt_library_submissions_status_check
        CHECK (status IN ('pending', 'approved', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_prompt_library_submissions_status_created
    ON prompt_library_submissions(status, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_prompt_library_submissions_user_created
    ON prompt_library_submissions(user_id, created_at DESC);
