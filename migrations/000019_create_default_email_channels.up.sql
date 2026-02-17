-- Create default email channels for existing users
-- These channels use registration email and are pre-verified

INSERT INTO notification_channels (
    id,
    user_id,
    type,
    target,
    is_enabled,
    is_verified,
    subscribe_to_all_services,
    created_at,
    updated_at
)
SELECT
    gen_random_uuid(),
    u.id,
    'email',
    u.email,
    true,
    true,
    false,
    u.created_at,
    NOW()
FROM users u
WHERE NOT EXISTS (
    SELECT 1
    FROM notification_channels nc
    WHERE nc.user_id = u.id
      AND nc.type = 'email'
      AND nc.target = u.email
);
