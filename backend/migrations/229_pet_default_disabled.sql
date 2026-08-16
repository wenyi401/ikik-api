ALTER TABLE pet_user_preferences
    ALTER COLUMN enabled SET DEFAULT FALSE;

COMMENT ON COLUMN pet_user_preferences.enabled IS
    'Whether the global pet widget is enabled; new preferences default to disabled';
