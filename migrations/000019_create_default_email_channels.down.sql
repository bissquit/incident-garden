-- Remove auto-created channels
-- Note: This will remove channels that were created by migration
-- but won't affect channels created manually by users after migration

-- We cannot reliably distinguish auto-created from manually created,
-- so down migration does nothing (safe approach)
-- Channels can be deleted manually if needed

SELECT 1; -- no-op, migration intentionally empty
