DROP TABLE IF EXISTS password_reset_tokens;

ALTER TABLE users
    DROP COLUMN IF EXISTS must_change_password,
    DROP COLUMN IF EXISTS is_active;
