-- Add subscribe_to_all_services column to notification_channels
ALTER TABLE notification_channels
ADD COLUMN subscribe_to_all_services BOOLEAN NOT NULL DEFAULT false;

-- Update constraint to include 'mattermost'
ALTER TABLE notification_channels
DROP CONSTRAINT check_channel_type;

ALTER TABLE notification_channels
ADD CONSTRAINT check_channel_type CHECK (type IN ('email', 'telegram', 'mattermost'));

-- Create channel_subscriptions table (channel-level subscriptions to services)
CREATE TABLE channel_subscriptions (
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel_id, service_id)
);

CREATE INDEX idx_channel_subscriptions_service ON channel_subscriptions(service_id);

-- Create event_subscribers table (channels subscribed to a specific event)
CREATE TABLE event_subscribers (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, channel_id)
);

CREATE INDEX idx_event_subscribers_channel ON event_subscribers(channel_id);

-- Migrate data from old subscription model to new channel-level model
-- For each user with a subscription, copy their service subscriptions to all their channels
INSERT INTO channel_subscriptions (channel_id, service_id, created_at)
SELECT nc.id, ss.service_id, NOW()
FROM subscription_services ss
JOIN subscriptions s ON s.id = ss.subscription_id
JOIN notification_channels nc ON nc.user_id = s.user_id
ON CONFLICT DO NOTHING;

-- For users with subscriptions but no subscription_services, set subscribe_to_all_services = true
UPDATE notification_channels nc
SET subscribe_to_all_services = true
WHERE EXISTS (
    SELECT 1 FROM subscriptions s
    WHERE s.user_id = nc.user_id
    AND NOT EXISTS (
        SELECT 1 FROM subscription_services ss
        WHERE ss.subscription_id = s.id
    )
);

-- Drop old tables
DROP TABLE subscription_services;
DROP TABLE subscriptions;
