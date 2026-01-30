-- Добавляем archived_at для services
ALTER TABLE services ADD COLUMN archived_at TIMESTAMP NULL;

-- Добавляем archived_at для service_groups
ALTER TABLE service_groups ADD COLUMN archived_at TIMESTAMP NULL;

-- Частичный индекс для быстрого поиска активных сервисов
CREATE INDEX idx_services_active ON services(id) WHERE archived_at IS NULL;

-- Частичный индекс для быстрого поиска активных групп
CREATE INDEX idx_service_groups_active ON service_groups(id) WHERE archived_at IS NULL;

-- Индекс для поиска архивированных (для админки)
CREATE INDEX idx_services_archived_at ON services(archived_at) WHERE archived_at IS NOT NULL;
CREATE INDEX idx_service_groups_archived_at ON service_groups(archived_at) WHERE archived_at IS NOT NULL;
