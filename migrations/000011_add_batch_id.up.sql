-- Add batch_id column to event_service_changes for grouping related changes
ALTER TABLE event_service_changes ADD COLUMN batch_id UUID;

-- Index for efficient queries by batch
CREATE INDEX idx_event_service_changes_batch_id ON event_service_changes(batch_id);
