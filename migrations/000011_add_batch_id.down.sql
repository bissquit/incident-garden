-- Remove batch_id column from event_service_changes
DROP INDEX IF EXISTS idx_event_service_changes_batch_id;
ALTER TABLE event_service_changes DROP COLUMN IF EXISTS batch_id;
