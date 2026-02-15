-- Table for email verification codes
CREATE TABLE channel_verification_codes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    code VARCHAR(6) NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    attempts INT NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT check_code_format CHECK (code ~ '^[0-9]{6}$')
);

-- Only one code per channel (old codes are deleted before creating new ones in repository)
CREATE UNIQUE INDEX idx_verification_codes_channel ON channel_verification_codes(channel_id);
CREATE INDEX idx_verification_codes_expires ON channel_verification_codes(expires_at);
