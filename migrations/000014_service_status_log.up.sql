-- Service status audit log
CREATE TABLE service_status_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    old_status VARCHAR(50),
    new_status VARCHAR(50) NOT NULL,
    source_type VARCHAR(20) NOT NULL,
    event_id UUID REFERENCES events(id) ON DELETE SET NULL,
    reason TEXT,
    created_by UUID NOT NULL REFERENCES users(id),
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),

    CONSTRAINT check_source_type CHECK (source_type IN ('manual', 'event', 'webhook')),
    CONSTRAINT check_old_status CHECK (old_status IS NULL OR old_status IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance')),
    CONSTRAINT check_new_status CHECK (new_status IN ('operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance'))
);

CREATE INDEX idx_service_status_log_service_id ON service_status_log(service_id);
CREATE INDEX idx_service_status_log_event_id ON service_status_log(event_id);
CREATE INDEX idx_service_status_log_created_at ON service_status_log(created_at DESC);
CREATE INDEX idx_service_status_log_source_type ON service_status_log(source_type);
