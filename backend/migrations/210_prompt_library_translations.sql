CREATE TABLE IF NOT EXISTS prompt_library_translations (
    prompt_id TEXT NOT NULL,
    locale VARCHAR(16) NOT NULL,
    source_hash CHAR(64) NOT NULL,
    title TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (prompt_id, locale)
);

CREATE INDEX IF NOT EXISTS idx_prompt_library_translations_locale_updated
    ON prompt_library_translations(locale, updated_at DESC);
