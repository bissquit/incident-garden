-- Migration: Service to Groups M:N relationship
-- Creates junction table for many-to-many relationship between services and groups

-- Create junction table for M:N relationship between services and groups
CREATE TABLE service_group_members (
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES service_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (service_id, group_id)
);

-- Index for efficient queries by group_id
CREATE INDEX idx_service_group_members_group_id ON service_group_members(group_id);

-- Migrate existing data from services.group_id to junction table
INSERT INTO service_group_members (service_id, group_id)
SELECT id, group_id FROM services WHERE group_id IS NOT NULL;

-- Drop the old foreign key column
ALTER TABLE services DROP COLUMN group_id;
