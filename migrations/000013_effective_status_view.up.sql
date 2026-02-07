-- Function to calculate status priority (higher = worse)
CREATE OR REPLACE FUNCTION service_status_priority(status VARCHAR)
RETURNS INTEGER AS $$
BEGIN
    RETURN CASE status
        WHEN 'major_outage' THEN 4
        WHEN 'partial_outage' THEN 3
        WHEN 'degraded' THEN 2
        WHEN 'maintenance' THEN 1
        ELSE 0
    END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- View for effective service status (worst-case from active events)
CREATE OR REPLACE VIEW v_service_effective_status AS
SELECT
    s.id,
    s.slug,
    s.status as stored_status,
    COALESCE(
        (SELECT es.status
         FROM event_services es
         JOIN events e ON es.event_id = e.id
         WHERE es.service_id = s.id
           AND e.status NOT IN ('resolved', 'completed')
         ORDER BY service_status_priority(es.status) DESC
         LIMIT 1),
        s.status
    ) as effective_status,
    EXISTS (
        SELECT 1
        FROM event_services es
        JOIN events e ON es.event_id = e.id
        WHERE es.service_id = s.id
          AND e.status NOT IN ('resolved', 'completed')
    ) as has_active_events
FROM services s;
