ALTER TABLE notification_channels ADD COLUMN is_default BOOLEAN NOT NULL DEFAULT false;

-- Mark existing registration email channels as default
UPDATE notification_channels nc
SET is_default = true
WHERE nc.type = 'email'
  AND EXISTS (
    SELECT 1 FROM users u
    WHERE u.id = nc.user_id
      AND u.email = nc.target
  );
