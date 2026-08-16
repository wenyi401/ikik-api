ALTER TABLE pet_user_preferences
    ADD COLUMN IF NOT EXISTS assistant_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL;

COMMENT ON COLUMN pet_user_preferences.assistant_group_id IS
    'User-selected group for billed pet assistant model requests';
