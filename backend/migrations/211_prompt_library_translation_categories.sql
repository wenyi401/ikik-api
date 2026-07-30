ALTER TABLE prompt_library_translations
    ADD COLUMN IF NOT EXISTS category VARCHAR(32) NOT NULL DEFAULT 'productivity';

CREATE INDEX IF NOT EXISTS idx_prompt_library_translations_category
    ON prompt_library_translations(category);
