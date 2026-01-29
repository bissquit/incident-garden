-- Таблица связи событий с группами (какие группы были выбраны при создании/редактировании)
CREATE TABLE event_groups (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    group_id UUID NOT NULL REFERENCES service_groups(id) ON DELETE CASCADE,
    PRIMARY KEY (event_id, group_id)
);

CREATE INDEX idx_event_groups_group_id ON event_groups(group_id);

-- Таблица истории изменений состава события
CREATE TABLE event_service_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    action VARCHAR(20) NOT NULL,  -- 'added', 'removed'
    service_id UUID REFERENCES services(id) ON DELETE SET NULL,
    group_id UUID REFERENCES service_groups(id) ON DELETE SET NULL,
    reason TEXT,
    created_by UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    CONSTRAINT check_action CHECK (action IN ('added', 'removed')),
    CONSTRAINT check_target CHECK (service_id IS NOT NULL OR group_id IS NOT NULL)
);

CREATE INDEX idx_event_service_changes_event_id ON event_service_changes(event_id);
CREATE INDEX idx_event_service_changes_created_at ON event_service_changes(created_at DESC);
