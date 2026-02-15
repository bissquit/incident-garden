 -- Notification queue for reliable delivery with retry

CREATE TABLE notification_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    message_type VARCHAR(20) NOT NULL,
    payload JSONB NOT NULL,

    -- Status and retry
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    max_attempts INT NOT NULL DEFAULT 3,
    next_attempt_at TIMESTAMP NOT NULL DEFAULT NOW(),
    last_error TEXT,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMP,

    CONSTRAINT check_queue_status CHECK (status IN ('pending', 'processing', 'sent', 'failed')),
    CONSTRAINT check_queue_message_type CHECK (message_type IN ('initial', 'update', 'resolved', 'completed', 'cancelled'))
);

-- Index for efficient pending item retrieval
CREATE INDEX idx_notification_queue_pending
    ON notification_queue(next_attempt_at)
    WHERE status = 'pending';

-- Index for event-based queries
CREATE INDEX idx_notification_queue_event
    ON notification_queue(event_id);

-- Index for channel-based queries
CREATE INDEX idx_notification_queue_channel
    ON notification_queue(channel_id);

-- Index for status-based queries
CREATE INDEX idx_notification_queue_status
    ON notification_queue(status);