ALTER TABLE event_services DROP CONSTRAINT IF EXISTS check_event_service_status;
ALTER TABLE event_services DROP COLUMN IF EXISTS updated_at;
ALTER TABLE event_services DROP COLUMN IF EXISTS status;
