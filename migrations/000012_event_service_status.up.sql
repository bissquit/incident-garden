-- Add status and updated_at columns to event_services
ALTER TABLE event_services
    ADD COLUMN status VARCHAR(50),
    ADD COLUMN updated_at TIMESTAMP;

-- Fill existing records based on event severity/type.
-- Note: incidents always have severity (enforced by business rules),
-- so ELSE branch only covers unexpected data inconsistencies.
UPDATE event_services es
SET
    status = CASE
        WHEN e.type = 'maintenance' THEN 'maintenance'
        WHEN e.severity = 'critical' THEN 'major_outage'
        WHEN e.severity = 'major' THEN 'partial_outage'
        WHEN e.severity = 'minor' THEN 'degraded'
        ELSE 'degraded'
    END,
    updated_at = COALESCE(e.started_at, e.created_at)
FROM events e
WHERE es.event_id = e.id;

-- Make columns NOT NULL with defaults
ALTER TABLE event_services
    ALTER COLUMN status SET NOT NULL,
    ALTER COLUMN updated_at SET NOT NULL,
    ALTER COLUMN status SET DEFAULT 'degraded',
    ALTER COLUMN updated_at SET DEFAULT NOW();

-- Add constraint for valid statuses
ALTER TABLE event_services
    ADD CONSTRAINT check_event_service_status
    CHECK (status IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance'));
