DROP INDEX idx_service_groups_archived_at;
DROP INDEX idx_services_archived_at;
DROP INDEX idx_service_groups_active;
DROP INDEX idx_services_active;

ALTER TABLE service_groups DROP COLUMN archived_at;
ALTER TABLE services DROP COLUMN archived_at;
