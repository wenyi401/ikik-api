ALTER TABLE pet_user_preferences
    ADD COLUMN IF NOT EXISTS assistant_model VARCHAR(160) NOT NULL DEFAULT '';

COMMENT ON COLUMN pet_user_preferences.assistant_model IS
    'User-selected text model for the pet AI playground';
