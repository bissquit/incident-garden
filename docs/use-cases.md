ГРУППА A: Жизненный цикл инцидента

A1. Создание инцидента с указанием статусов сервисов

Подготовка

- Создать сервис "api-gateway" (status: operational)
- Создать сервис "auth-service" (status: operational)

Действие

Создать инцидент:
```json
{
    "title": "API Performance Issues",
    "type": "incident",
    "status": "investigating",
    "severity": "major",    
    "description": "Response times increased",
    "affected_services": [
        {"service_id": "<api-gateway-id>", "status": "partial_outage"},
        {"service_id": "<auth-service-id>", "status": "degraded"}
    ]
}
```
Ожидаемый результат

1. Инцидент создан, возвращается 201
2. GET /services/api-gateway:
    - effective_status = "partial_outage"
    - has_active_events = true
3. GET /services/auth-service:
    - effective_status = "degraded"
    - has_active_events = true
4. GET /events/<id>:
    - service_ids содержит оба ID
    - status = "investigating"

--- 
A2. Создание инцидента с группой сервисов

Подготовка

- Создать группу "backend-services"
- Создать сервисы "database", "cache", "queue" в этой группе (все operational)

Действие

Создать инцидент:
```json
{
    "title": "Backend Infrastructure Issue",
    "type": "incident",
    "status": "investigating",
    "severity": "critical",
    "description": "Multiple backend services affected",
    "affected_groups": [
        {"group_id": "<backend-services-id>", "status": "major_outage"}
    ]
}
```

Ожидаемый результат

1. Инцидент создан
2. ВСЕ сервисы группы получают effective_status = "major_outage":
    - GET /services/database → effective_status = "major_outage"
    - GET /services/cache → effective_status = "major_outage"
    - GET /services/queue → effective_status = "major_outage"
3. GET /events/<id> содержит все три service_ids

  --- 
A3. Создание инцидента: приоритет явного сервиса над группой

Подготовка

- Группа "backend" с сервисами: database, cache
- Сервис database также указан явно

Действие
```json
{
    "title": "Mixed selection test",
    "type": "incident",
    "status": "investigating",
    "severity": "major",
    "description": "Test",
    "affected_services": [
        {"service_id": "<database-id>", "status": "degraded"}
    ],
    "affected_groups": [
        {"group_id": "<backend-id>", "status": "major_outage"}
    ]
}
```

Ожидаемый результат

1. database: effective_status = "degraded" (явный приоритет)
2. cache: effective_status = "major_outage" (из группы)

--- 
A4. Обновление инцидента: изменение статусов сервисов

Подготовка

- Активный инцидент с сервисами api-gateway (partial_outage), auth-service (degraded)

Действие
POST /events/<id>/updates
```json
{
    "status": "identified",
    "message": "Root cause found",
    "service_updates": [
        {"service_id": "<api-gateway-id>", "status": "degraded"}
    ] 
}
```

Ожидаемый результат

1. Update создан (201)
2. Инцидент: status = "identified"
3. api-gateway: effective_status = "degraded" (изменился)
4. auth-service: effective_status = "degraded" (не менялся)

--- 
A5. Обновление инцидента: добавление новых сервисов

Подготовка

- Активный инцидент с api-gateway
- Сервис "database" существует, operational

Действие

POST /events/<id>/updates
```json
{
    "status": "identified",
    "message": "Database also affected",
    "add_services": [
        {"service_id": "<database-id>", "status": "major_outage"}
    ],
    "reason": "Investigation revealed database impact"
}
```

Ожидаемый результат

1. database добавлен в инцидент
2. database: effective_status = "major_outage"
3. GET /events/<id>/changes содержит запись о добавлении database

--- 
A6. Обновление инцидента: добавление группы

Подготовка

- Активный инцидент
- Группа "frontend" с сервисами website, mobile-app (оба operational)

Действие

POST /events/<id>/updates
```json
{
    "status": "identified",
    "message": "Frontend also impacted",
    "add_groups": [
        {"group_id": "<frontend-id>", "status": "degraded"}
    ] 
}
```

Ожидаемый результат

1. website и mobile-app добавлены в инцидент
2. Оба: effective_status = "degraded"
3. GET /events/<id> содержит group_id в списке

--- 
A7. Обновление инцидента: удаление сервиса

Подготовка

- Активный инцидент с сервисами: api-gateway, auth-service, database

Действие

POST /events/<id>/updates
```json
{
    "status": "monitoring",
    "message": "Auth service was not affected",
    "remove_service_ids": ["<auth-service-id>"],
    "reason": "Incorrectly added"
}
```

Ожидаемый результат

1. auth-service удалён из инцидента
2. auth-service: effective_status = "operational" (если нет других событий)
3. auth-service: has_active_events = false
4. GET /events/<id>/changes содержит запись об удалении

--- 
A8. Частичное восстановление: сервис operational, инцидент активен

Подготовка

- Активный инцидент с api-gateway (major_outage), database (major_outage)

Действие

POST /events/<id>/updates
```json
{
    "status": "monitoring",
    "message": "API Gateway recovered",
    "service_updates": [
        {"service_id": "<api-gateway-id>", "status": "operational"}
    ]
}
```

Ожидаемый результат

1. api-gateway: effective_status = "operational"
2. api-gateway ОСТАЁТСЯ в инциденте (service_ids содержит его)
3. database: effective_status = "major_outage"
4. Инцидент: status = "monitoring" (всё ещё активен)

--- 
A9. Закрытие инцидента

Подготовка

- Активный инцидент с api-gateway (degraded), database (partial_outage)

Действие

POST /events/<id>/updates
```json
{
    "status": "resolved",
    "message": "All services recovered"
}
```

Ожидаемый результат

1. Инцидент: status = "resolved", resolved_at заполнен
2. api-gateway:
    - stored_status = "operational"
    - effective_status = "operational"
    - has_active_events = false
3. database:
    - stored_status = "operational"
    - effective_status = "operational"
    - has_active_events = false

--- 
A10. Закрытие инцидента при наличии другого активного

Подготовка

- Инцидент A: api-gateway (major_outage) — активен
- Инцидент B: api-gateway (degraded) — активен

Действие

Закрыть инцидент A:
POST /events/<incident-A-id>/updates
```json
{
    "status": "resolved",
    "message": "Resolved"
}
```

Ожидаемый результат

1. Инцидент A: status = "resolved"
2. api-gateway:
    - effective_status = "degraded" (из инцидента B, worst-case)
    - has_active_events = true
3. stored_status НЕ изменился на operational (есть другой активный)

--- 
A11. Нельзя обновлять закрытый инцидент

Подготовка

- Закрытый инцидент (status = "resolved")

Действие

POST /events/<id>/updates
```json
{
    "status": "monitoring",
    "message": "Trying to reopen"
}
```

Ожидаемый результат

- HTTP 409 Conflict
- Сообщение: "cannot update resolved event"

--- 
ГРУППА B: Жизненный цикл maintenance

B1. Создание scheduled maintenance — статусы НЕ меняются

Подготовка

- Сервис database (operational)

Действие
```json
{
    "title": "Database Migration",
    "type": "maintenance",
    "status": "scheduled",
    "description": "Planned migration",
    "scheduled_start_at": "<будущая дата>",
    "scheduled_end_at": "<будущая дата + 4 часа>",
    "affected_services": [
        {"service_id": "<database-id>", "status": "maintenance"}
    ] 
}
```

Ожидаемый результат

1. Maintenance создан
2. database:
    - effective_status = "operational" (НЕ maintenance!)
    - has_active_events = false (scheduled не считается active)

--- 
B2. Начало maintenance (scheduled → in_progress)

Подготовка

- Scheduled maintenance с database

Действие

POST /events/<id>/updates
```json
{
    "status": "in_progress",
    "message": "Maintenance started"
}
```

Ожидаемый результат

1. Maintenance: status = "in_progress"
2. database:
    - effective_status = "maintenance"
    - has_active_events = true

--- 
B3. Завершение maintenance

Подготовка

- Maintenance in_progress с database (maintenance)

Действие

POST /events/<id>/updates
```json
{
    "status": "completed",
    "message": "Migration completed"
}
```

Ожидаемый результат

1. Maintenance: status = "completed", resolved_at заполнен
2. database:
    - stored_status = "operational"
    - effective_status = "operational"
    - has_active_events = false

--- 
B4. Scheduled maintenance + активный инцидент

Подготовка

- Активный инцидент: api-gateway (major_outage)
- Создать scheduled maintenance с api-gateway

Действие

Создать scheduled maintenance

Ожидаемый результат

1. api-gateway: effective_status = "major_outage" (инцидент, НЕ scheduled)
2. Перевести maintenance в in_progress
3. api-gateway: effective_status = "major_outage" (worst-case: 4 > 1)

--- 
ГРУППА C: Множественные события (worst-case)

C1. Два инцидента на одном сервисе — worst-case

Подготовка

- Сервис api-gateway (operational)

Действие

1. Создать инцидент A: api-gateway = degraded
2. Создать инцидент B: api-gateway = major_outage

Ожидаемый результат

- api-gateway: effective_status = "major_outage" (worst-case)

--- 
C2. Закрытие одного из инцидентов — пересчёт worst-case

Подготовка

- Инцидент A: api-gateway (degraded) — активен
- Инцидент B: api-gateway (major_outage) — активен

Действие

Закрыть инцидент B

Ожидаемый результат

1. api-gateway: effective_status = "degraded" (из инцидента A)
2. has_active_events = true

Закрыть инцидент A

Ожидаемый результат

1. api-gateway: effective_status = "operational"
2. has_active_events = false

--- 
C3. Приоритет статусов (проверка порядка)

Подготовка

- Сервис api-gateway

Действие

Создать инциденты с разными статусами:
1. Инцидент: api-gateway = degraded (priority 2)
2. Инцидент: api-gateway = partial_outage (priority 3)
3. Инцидент: api-gateway = maintenance (priority 1)

Ожидаемый результат

- effective_status = "partial_outage" (максимальный приоритет 3)

Закрыть инцидент с partial_outage:
- effective_status = "degraded" (следующий: priority 2)

--- 
C4. in_progress maintenance + инцидент

Подготовка

- Maintenance in_progress: database = maintenance
- Инцидент: database = degraded

Ожидаемый результат

- database: effective_status = "degraded" (priority 2 > 1)

--- 
ГРУППА D: Ручное управление статусами

D1. Ручное изменение статуса без событий

Подготовка

- Сервис api-gateway (operational), нет активных событий

Действие

PATCH /services/api-gateway
```json
{
    "name": "API Gateway",
    "slug": "api-gateway",
    "status": "degraded",
    "reason": "Load testing in progress"
}
```

Ожидаемый результат

1. stored_status = "degraded"
2. effective_status = "degraded"
3. has_active_events = false
4. GET /services/api-gateway/status-log содержит запись:
    - source_type = "manual"
    - old_status = "operational"
    - new_status = "degraded"

  --- 
D2. Ручное изменение при активном инциденте — не влияет на effective

Подготовка

- Активный инцидент: api-gateway = major_outage

Действие

PATCH /services/api-gateway
```json
{
    "name": "API Gateway",
    "slug": "api-gateway",
    "status": "degraded"
}
```

Ожидаемый результат

1. stored_status = "degraded"
2. effective_status = "major_outage" (определяется событием)
3. has_active_events = true

--- 
D3. Ручной статус "теряется" при закрытии инцидента

Подготовка

1. Сервис api-gateway: stored_status = "degraded" (ручное)
2. Создать инцидент: api-gateway = major_outage

Действие

Закрыть инцидент

Ожидаемый результат

1. stored_status = "operational" (сброшен, НЕ degraded)
2. effective_status = "operational"

Это ожидаемое поведение. Если нужен degraded после инцидента — установить вручную.
 
--- 
ГРУППА E: Страница сервиса

E1. Получение событий сервиса

Подготовка

- Сервис api-gateway
- 2 активных инцидента
- 3 закрытых инцидента

Действие

GET /services/api-gateway/events

Ожидаемый результат

1. Возвращает все 5 событий
2. Активные идут первыми
3. Внутри групп — сортировка по дате (новые первыми)

  --- 
E2. Фильтр событий: только активные

Подготовка

- Как E1

Действие

GET /services/api-gateway/events?status=active

Ожидаемый результат

- Только 2 активных инцидента
- total = 2

--- 
E3. Фильтр событий: только закрытые

Подготовка

- Как E1

Действие

GET /services/api-gateway/events?status=resolved

Ожидаемый результат

- Только 3 закрытых инцидента
- total = 3

  --- 
E4. Пагинация событий

Подготовка

- Сервис с 10 событиями

Действие

GET /services/api-gateway/events?limit=3&offset=0
GET /services/api-gateway/events?limit=3&offset=3

Ожидаемый результат

- Первый запрос: 3 события, total = 10
- Второй запрос: 3 других события, total = 10

--- 
E5. История статусов сервиса

Подготовка

- Сервис api-gateway
- Несколько изменений статуса (ручных и от событий)

Действие

GET /services/api-gateway/status-log

Ожидаемый результат (требует operator+ роль)

1. Список изменений с полями:
    - old_status, new_status
    - source_type (manual/event/webhook)
    - event_id (если source_type = event)
    - reason
    - created_by, created_at
2. Сортировка: новые первыми

--- 
E6. История статусов — требует авторизации

Действие (без токена)

GET /services/api-gateway/status-log

Ожидаемый результат

- HTTP 401 Unauthorized

Действие (с user токеном)

GET /services/api-gateway/status-log
Authorization: Bearer <user-token>

Ожидаемый результат

- HTTP 403 Forbidden

--- 
ГРУППА F: События в прошлом

F1. Создание инцидента в прошлом (уже resolved)

Подготовка

- Сервис api-gateway (operational)

Действие
```json
{
    "title": "Past incident",
    "type": "incident",
    "status": "resolved",
    "severity": "minor",
    "description": "Already resolved",
    "started_at": "2024-01-10T10:00:00Z",
    "resolved_at": "2024-01-10T12:00:00Z",
    "affected_services": [
        {"service_id": "<api-gateway-id>", "status": "degraded"}
    ]
}
```

Ожидаемый результат

1. Инцидент создан со status = "resolved"
2. api-gateway:
    - effective_status = "operational" (событие уже закрыто)
    - has_active_events = false
3. Инцидент появляется в истории событий сервиса

--- 
F2. Инцидент в прошлом — не влияет на текущий статус

Подготовка

- Сервис api-gateway: stored_status = "degraded" (ручное)

Действие

Создать resolved инцидент в прошлом

Ожидаемый результат

- stored_status остаётся "degraded"
- effective_status = "degraded"
- Инцидент в прошлом НЕ сбрасывает на operational (он создан уже закрытым)

--- 
ГРУППА G: Удаление событий

G1. Нельзя удалить активное событие

Подготовка

- Активный инцидент (status = "investigating")

Действие

DELETE /events/<id>
Authorization: Bearer <admin-token>

Ожидаемый результат

- HTTP 409 Conflict
- Сообщение: "cannot delete active event: resolve it first"

  --- 
G2. Удаление закрытого события

Подготовка

- Закрытый инцидент (status = "resolved")
- Есть event_updates
- Есть записи в event_service_changes
- Есть записи в service_status_log

Действие

DELETE /events/<id>
Authorization: Bearer <admin-token>

Ожидаемый результат

1. HTTP 204 No Content
2. GET /events/<id> → 404 Not Found
3. event_services удалены
4. event_updates удалены
5. event_service_changes удалены
6. service_status_log записи с этим event_id удалены

--- 
G3. Удаление события — статусы сервисов не меняются

Подготовка

- Сервис api-gateway: effective_status = operational
- Закрытый инцидент где api-gateway был major_outage

Действие

Удалить инцидент

Ожидаемый результат

- api-gateway: effective_status = operational (не изменился)

--- 
G4. Удаление требует admin роль

Действие (с operator токеном)

DELETE /events/<id>
Authorization: Bearer <operator-token>

Ожидаемый результат

- HTTP 403 Forbidden

--- 
ГРУППА H: Аудит и логирование

H1. Лог при создании события

Действие

Создать инцидент с api-gateway

Ожидаемый результат

GET /services/api-gateway/status-log содержит:
- source_type = "event"
- event_id = <id инцидента>
- old_status = "operational"
- new_status = <статус из affected_services>

--- 
H2. Лог при обновлении статуса в событии

Действие

Обновить статус сервиса в инциденте

Ожидаемый результат

Новая запись в status-log с изменением статуса
 
--- 
H3. Лог при закрытии события

Действие

Закрыть инцидент

Ожидаемый результат

Запись в status-log:
- new_status = "operational"
- reason содержит информацию о закрытии события

--- 
H4. Event service changes при добавлении сервиса

Действие

Добавить сервис в инцидент через update

Ожидаемый результат

GET /events/<id>/changes содержит:
- action = "added"
- service_id = <добавленный сервис>
- reason = <указанная причина>

--- 
H5. Event service changes при удалении сервиса

Действие

Удалить сервис из инцидента через update

Ожидаемый результат

GET /events/<id>/changes содержит:
- action = "removed"
- service_id = <удалённый сервис>

--- 
ГРУППА I: Права доступа

I1. Создание события — требует operator+

Действие (без токена)

POST /events

Ожидаемый результат

- HTTP 401 Unauthorized

Действие (с user токеном)

POST /events
Authorization: Bearer <user-token>

Ожидаемый результат

- HTTP 403 Forbidden

Действие (с operator токеном)

Ожидаемый результат

- HTTP 201 Created

--- 
I2. Чтение событий — публичный доступ

Действие (без токена)

GET /event
GET /events/<id>
GET /events/<id>/updates  
GET /events/<id>/changes  
GET /services/<slug>/events

Ожидаемый результат

- Все возвращают HTTP 200

--- 
I3. Удаление события — только admin

Как G4.
 
--- 
ГРУППА J: Валидация

J1. Инцидент без severity — ошибка

Действие
```json
{
    "title": "Test",
    "type": "incident",
    "status": "investigating",
    "description": "Test"
    // severity отсутствует
}
```

Ожидаемый результат

- HTTP 400 Bad Request
- Ошибка о required severity

--- 
J2. Maintenance с severity — игнорируется или ошибка?

Действие
```json
{
    "title": "Test",
    "type": "maintenance",
    "status": "scheduled",
    "severity": "major",
    "description": "Test"
}
```

Ожидаемый результат

Уточнить: severity игнорируется или возвращает ошибку?
 
--- 
J3. Несуществующий service_id

Действие
```json
{
    "affected_services": [
        {"service_id": "00000000-0000-0000-0000-000000000000", "status": "degraded"}
    ]
}
```

Ожидаемый результат

- HTTP 400 или 404
- Сообщение о несуществующем сервисе

---
J4. Невалидный статус сервиса

Действие
```json
{
    "affected_services": [
        {"service_id": "<valid-id>", "status": "invalid_status"}
    ]
}
```

Ожидаемый результат

- HTTP 400 Bad Request
- Ошибка валидации статуса

---
J5. Невалидный переход статуса события

Действие

Инцидент в статусе "investigating", попытка перевести в "completed"

Ожидаемый результат

- HTTP 400 Bad Request
- completed не валиден для incident (только для maintenance)
