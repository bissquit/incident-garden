-- Rollback: restore view without 'scheduled' in exclusions
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
