CREATE TABLE IF NOT EXISTS pet_assets (
    id UUID PRIMARY KEY,
    owner_user_id BIGINT REFERENCES users(id) ON DELETE CASCADE,
    pet_key VARCHAR(96) NOT NULL,
    display_name VARCHAR(120) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    sprite_version SMALLINT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    sha256 CHAR(64) NOT NULL,
    size_bytes BIGINT NOT NULL,
    width INTEGER NOT NULL,
    height INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pet_assets_sprite_version_check CHECK (sprite_version IN (1, 2)),
    CONSTRAINT pet_assets_dimensions_check CHECK (width = 1536 AND height IN (1872, 2288))
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_pet_assets_owner_key
    ON pet_assets(owner_user_id, pet_key) WHERE owner_user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_pet_assets_owner_created
    ON pet_assets(owner_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS pet_user_preferences (
    user_id BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    selected_asset_id UUID REFERENCES pet_assets(id) ON DELETE SET NULL,
    enabled BOOLEAN NOT NULL DEFAULT FALSE,
    size VARCHAR(16) NOT NULL DEFAULT 'medium',
    anchor VARCHAR(24) NOT NULL DEFAULT 'bottom-left',
    reduced_motion BOOLEAN NOT NULL DEFAULT FALSE,
    activity_reactions BOOLEAN NOT NULL DEFAULT TRUE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pet_user_preferences_size_check CHECK (size IN ('small', 'medium', 'large')),
    CONSTRAINT pet_user_preferences_anchor_check CHECK (anchor IN ('bottom-left', 'bottom-right'))
);

CREATE TABLE IF NOT EXISTS pet_knowledge_documents (
    id BIGSERIAL PRIMARY KEY,
    slug VARCHAR(120) NOT NULL UNIQUE,
    title VARCHAR(240) NOT NULL,
    content_md TEXT NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'draft',
    version INTEGER NOT NULL DEFAULT 1,
    updated_by BIGINT REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pet_knowledge_status_check CHECK (status IN ('draft', 'published', 'archived'))
);
CREATE INDEX IF NOT EXISTS idx_pet_knowledge_status_updated
    ON pet_knowledge_documents(status, updated_at DESC);

CREATE TABLE IF NOT EXISTS pet_conversations (
    id UUID PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(160) NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_pet_conversations_user_updated
    ON pet_conversations(user_id, updated_at DESC);

CREATE TABLE IF NOT EXISTS pet_messages (
    id BIGSERIAL PRIMARY KEY,
    conversation_id UUID NOT NULL REFERENCES pet_conversations(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL,
    content TEXT NOT NULL,
    citations JSONB NOT NULL DEFAULT '[]'::jsonb,
    refused BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT pet_messages_role_check CHECK (role IN ('user', 'assistant'))
);
CREATE INDEX IF NOT EXISTS idx_pet_messages_conversation_created
    ON pet_messages(conversation_id, created_at ASC);

COMMENT ON TABLE pet_assets IS 'Codex Pet compatible sprite metadata; binary assets live outside PostgreSQL';
COMMENT ON TABLE pet_knowledge_documents IS 'Administrator-approved IKIK-only support knowledge';
COMMENT ON TABLE pet_conversations IS 'User-scoped pet customer-service conversations';
