-- Recreate old tables
CREATE TABLE subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_subscriptions_user_id ON subscriptions(user_id);

CREATE TABLE subscription_services (
    subscription_id UUID NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    PRIMARY KEY (subscription_id, service_id)
);

CREATE INDEX idx_subscription_services_service_id ON subscription_services(service_id);

-- Best effort data restoration:
-- Create subscriptions for users who have channels with subscriptions
INSERT INTO subscriptions (user_id)
SELECT DISTINCT nc.user_id
FROM notification_channels nc
WHERE nc.subscribe_to_all_services = true
   OR EXISTS (SELECT 1 FROM channel_subscriptions cs WHERE cs.channel_id = nc.id);

-- Copy channel_subscriptions to subscription_services (using first channel per user)
INSERT INTO subscription_services (subscription_id, service_id)
SELECT DISTINCT ON (s.id, cs.service_id) s.id, cs.service_id
FROM subscriptions s
JOIN notification_channels nc ON nc.user_id = s.user_id
JOIN channel_subscriptions cs ON cs.channel_id = nc.id;

-- Drop new tables
DROP TABLE event_subscribers;
DROP TABLE channel_subscriptions;

-- Remove subscribe_to_all_services column
ALTER TABLE notification_channels
DROP COLUMN subscribe_to_all_services;

-- Restore original constraint (without mattermost)
ALTER TABLE notification_channels
DROP CONSTRAINT check_channel_type;

ALTER TABLE notification_channels
ADD CONSTRAINT check_channel_type CHECK (type IN ('email', 'telegram'));
