-- Rollback: Restore 1:N relationship between services and groups

-- Restore the group_id column
ALTER TABLE services ADD COLUMN group_id UUID REFERENCES service_groups(id) ON DELETE SET NULL;

-- Create index for the restored column
CREATE INDEX idx_services_group_id ON services(group_id);

-- Drop the junction table
DROP TABLE service_group_members;
