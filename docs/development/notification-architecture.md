# Notification Architecture (already implemented)

> Архитектурный документ системы уведомлений для IncidentGarden.
> Версия: 1.0 | Дата: 2024-01

---

## Содержание

1. [Концепция](#1-концепция)
2. [Ключевые решения](#2-ключевые-решения)
3. [Каналы уведомлений](#3-каналы-уведомлений)
4. [Модель подписок](#4-модель-подписок)
5. [Триггеры уведомлений](#5-триггеры-уведомлений)
6. [Схема данных](#6-схема-данных)
7. [Контракт NotificationPayload](#7-контракт-notificationpayload)
8. [Шаблоны сообщений](#8-шаблоны-сообщений)
9. [Архитектура компонентов](#9-архитектура-компонентов)
10. [Конфигурация](#10-конфигурация)
11. [API Endpoints](#11-api-endpoints)
12. [UI/UX](#12-uiux)
13. [Отложенный функционал](#13-отложенный-функционал)

---

## 1. Концепция

### Основной принцип

**Уведомления отправляются только о событиях (incidents, maintenance), не об изменениях статусов сервисов.**

### Обоснование

| Автоматические статусы | События |
|------------------------|---------|
| Шум от alertmanager (flapping) | Осознанная коммуникация оператора |
| Нет контекста ("сервис упал" — и что?) | Полный контекст: причина, прогресс, ETA |
| Много ложных срабатываний | Оператор фильтрует и публикует релевантное |
| Девальвация канала (пользователь заглушит) | Редкие, ценные уведомления |

### Что это означает

- **Подписка на сервис** = "уведомлять о событиях, затрагивающих этот сервис"
- **Событие** = incident или maintenance, созданное оператором
- Изменение статуса сервиса без события (например, через alertmanager webhook) **не генерирует уведомлений**

---

## 2. Ключевые решения

### 2.1. Архитектура: монолит

**Решение:** Уведомления остаются частью основного приложения, не выносятся в отдельный сервис.

**Причины:**
- Текущий масштаб не требует микросервисов (даже 3000 подписчиков × 100 сервисов = сотни сообщений/минуту)
- Shared state: подписки, события, шаблоны в одной БД
- Транзакционная консистентность: создание события + отправка должны быть атомарны
- Упрощение деплоя и мониторинга

**Когда пересмотреть:**
- Уведомления блокируют основной API
- Нужна независимая масштабируемость
- Появились требования к отдельному SLA

### 2.2. Конфигурация: через ENV

**Решение:** SMTP credentials, Telegram bot token и другие инфраструктурные настройки — через ENV/config, не через UI админки.

**Причины:**
- Это инфраструктурная конфигурация, меняется редко (раз в год)
- Секреты должны быть в K8s Secrets / Vault, не в БД
- Разделение: DevOps настраивает инфраструктуру, операторы работают в UI
- Уже есть koanf, добавить поля — минимальные усилия

### 2.3. Подписчики события: фиксируются при создании

**Решение:** При создании события определяется список подписчиков (на основе затронутых сервисов). Этот список сохраняется в `event_subscribers` и используется для всех последующих уведомлений.

**Причины:**
- Пользователь, получивший initial notification, должен получить все updates до закрытия
- Исключение сервиса из события не должно "отключать" уже вовлечённых подписчиков
- Технически проще: не пересчитывать подписчиков при каждом update

**При добавлении сервисов:** новые подписчики добавляются в `event_subscribers` и получают тот же update, что и остальные.

### 2.4. Без агрегации updates

**Решение:** Один update события = одно уведомление. Агрегация нескольких updates за короткий период не производится.

**Причины:**
- Реальный сценарий: оператор пишет update, через минуты/часы — следующий
- Агрегация добавляет сложность без реальной ценности
- Если нужен debounce — это защита от double-click (секунды), не агрегация контента

### 2.5. По умолчанию пользователь не подписан ни на что

**Решение:** Новый пользователь не имеет подписок. Чтобы получать уведомления, нужно явно добавить канал и выбрать сервисы.

**Причины:**
- При 100+ сервисах нет логики "на что подписывать по умолчанию"
- Публичная страница статусов доступна всем без подписки
- Уведомления нужны не всем — только заинтересованным (команды разработки, продакты)
- Нет спама, нет вопросов "почему мне это пришло"
- Осознанный выбор: кто хочет — сам настроит

### 2.6. notify_subscribers по умолчанию true

**Решение:** При создании события галочка "Уведомить подписчиков" включена по умолчанию.

**Причины:**
- Лучше уведомить случайно, чем не уведомить о важном
- Явное отключение требует осознанного действия оператора
- Снижает вероятность "забыл поставить галочку"

### 2.7. Отмена scheduled maintenance

**Решение:** `DELETE /events/{id}` для scheduled событий отправляет "cancelled" уведомление, затем удаляет событие.

**Причины:**
- Пользователи должны знать, что плановые работы отменены
- Не нужен отдельный статус "cancelled" — событие просто удаляется
- Единообразие: один endpoint для удаления всех типов событий

---

## 3. Каналы уведомлений

### Поддерживаемые каналы

| Канал | Target | Верификация | Batching |
|-------|--------|-------------|----------|
| Email | email адрес | Код (6 цифр) на почту | BCC по 50 |
| Telegram | chat_id | Тестовое сообщение | Rate limit 25/sec |
| Mattermost | webhook URL | Тестовый POST | Без ограничений |

### Email

**Настройка (DevOps):**
- SMTP host, port, credentials в ENV
- From address в ENV

**Пользовательский flow:**
1. Ввод email в настройках
2. Система отправляет 6-значный код
3. Ввод кода → канал верифицирован

**Отправка:**
- Один email на событие, получатели в BCC
- Батчи по 50 (лимит большинства SMTP серверов)
- При N получателях = ceil(N/50) писем

### Telegram

**Настройка (DevOps):**
- Создать бота через @BotFather
- Добавить `TELEGRAM_BOT_TOKEN` в ENV

**Пользовательский flow (MVP):**
1. Пользователь пишет `/start` боту @YourStatusBot
2. Узнаёт свой chat_id у @userinfobot
3. Вводит chat_id в настройках
4. Система отправляет тестовое сообщение → если дошло, канал верифицирован

**Будущее улучшение:** Webhook для автоматического получения chat_id (пользователь кликает ссылку, chat_id определяется автоматически).

**Отправка:**
- HTTP POST на `https://api.telegram.org/bot<TOKEN>/sendMessage`
- Rate limit: 25 msg/sec (ниже лимита Telegram 30/sec)
- Parse mode: Markdown

### Mattermost

**Настройка (DevOps):** Нет глобальной конфигурации.

**Пользовательский flow:**
1. Создать Incoming Webhook в Mattermost (Settings → Integrations)
2. Ввести webhook URL в настройках
3. Система отправляет тестовый POST → если 200 OK, канал верифицирован

**Отправка:**
- HTTP POST на webhook URL
- Payload: `{ "text": "message", "username": "StatusPage" }`
- Сообщения идут в канал, не личные (это особенность webhooks)

---

## 4. Модель подписок

### Принцип

**По умолчанию пользователь не подписан ни на что.**

Обоснование:
- Публичная страница статусов доступна всем без подписки
- При 100+ сервисах нет логики "на что подписывать по умолчанию"
- Уведомления получают только те, кому это реально нужно (команды разработки, продакты, руководители)
- Нет спама, нет вопросов "почему мне это пришло"
- Кто хочет — сам настроит

### Структура

```
Пользователь
└── Каналы (email, telegram, mattermost, ...)
    └── Подписки на сервисы
        - Пустой список = НЕ подписан ни на что
        - Явный список сервисов = подписан на них
        - Флаг "все сервисы" = подписан на всё (включая новые)
```

### UI: группировка сервисов

Сервисы отображаются сгруппированными для удобства, но подписки хранятся плоским списком.

```
▼ Backend                           [Email] [TG]
  ├─ [✓] API Gateway                  [✓]   [✓]
  ├─ [✓] Database                     [✓]   [ ]
  └─ [ ] Auth Service                 [ ]   [✓]

▼ Frontend                          [Email] [TG]
  ├─ [ ] Web App                      [ ]   [ ]
  └─ [ ] CDN                          [ ]   [ ]
```

Логика чекбоксов группы:
- Клик на группу → toggle всех сервисов в ней
- Все выбраны → ✓, часть → ▣ (indeterminate), ни одного → пусто

Это чисто UI-логика, в БД: `channel_subscriptions(channel_id, service_id)`.

### Новые сервисы

| Тип подписки | Новый сервис |
|--------------|--------------|
| Пустой список (не подписан) | Не влияет |
| Явный список сервисов | Не добавляется |
| Флаг "все сервисы" | Автоматически включён |

### Пример

| Канал | Подписки |
|-------|----------|
| Email (user@example.com) | API Gateway, Database |
| Telegram (123456789) | Все сервисы (флаг) |
| Mattermost (webhook) | Не подписан |

При инциденте с Database:
- Email получит уведомление (подписан на Database)
- Telegram получит уведомление (подписан на всё)
- Mattermost НЕ получит (не подписан)

---

## 5. Триггеры уведомлений

### Когда отправляются уведомления

| Событие в системе | Уведомление | Тип сообщения |
|-------------------|-------------|---------------|
| Event created (incident/maintenance) | ✅ Да | `initial` |
| Event update (status transition) | ✅ Да | `update` |
| Event update (message added) | ✅ Да | `update` |
| Services added to event | ✅ Да | `update` |
| Services removed from event | ✅ Да | `update` |
| Service status changed within event | ✅ Да | `update` |
| Event resolved | ✅ Да | `resolved` |
| Event completed (maintenance) | ✅ Да | `completed` |
| Scheduled maintenance deleted | ✅ Да | `cancelled` |
| Service status changed (без события) | ❌ Нет | — |

### Условие отправки

```
if event.NotifySubscribers == true {
    send notification
}
```

### Определение подписчиков

**При создании события:**
1. Получить список затронутых сервисов (service_ids)
2. Найти все каналы, подписанные хотя бы на один сервис:
   - Каналы с `subscribe_to_all_services = true`
   - Каналы с `channel_subscriptions.service_id IN (service_ids)`
3. Отфильтровать: только `is_enabled = true` и `is_verified = true`
4. Сохранить в `event_subscribers`

**SQL:**
```sql
SELECT DISTINCT nc.id
FROM notification_channels nc
LEFT JOIN channel_subscriptions cs ON cs.channel_id = nc.id
WHERE nc.is_enabled = true
  AND nc.is_verified = true
  AND (
      nc.subscribe_to_all_services = true
      OR cs.service_id = ANY($1::uuid[])  -- $1 = affected service_ids
  )
```

**При добавлении сервисов:**
1. Найти новых подписчиков (подписаны на добавленные сервисы, но не в `event_subscribers`)
2. Добавить в `event_subscribers`
3. Отправить им тот же update

**При исключении сервиса:** Подписчики остаются, продолжают получать уведомления.

---

## 6. Схема данных

### Изменения в notification_channels

```sql
-- Флаг "подписан на все сервисы"
ALTER TABLE notification_channels
    ADD COLUMN subscribe_to_all_services BOOLEAN NOT NULL DEFAULT false;
```

### Новые таблицы

```sql
-- Подписки канала на конкретные сервисы
-- Используется только если subscribe_to_all_services = false
CREATE TABLE channel_subscriptions (
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    service_id UUID NOT NULL REFERENCES services(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (channel_id, service_id)
);

CREATE INDEX idx_channel_subscriptions_service ON channel_subscriptions(service_id);

-- Подписчики конкретного события
-- Фиксируются при создании события, используются для всех уведомлений
CREATE TABLE event_subscribers (
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    PRIMARY KEY (event_id, channel_id)
);

CREATE INDEX idx_event_subscribers_channel ON event_subscribers(channel_id);

-- Очередь уведомлений (для retry)
CREATE TABLE notification_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL REFERENCES events(id) ON DELETE CASCADE,
    channel_id UUID NOT NULL REFERENCES notification_channels(id) ON DELETE CASCADE,
    message_type VARCHAR(20) NOT NULL,  -- initial, update, resolved, completed, cancelled
    payload JSONB NOT NULL,             -- NotificationPayload в JSON
    status VARCHAR(20) NOT NULL DEFAULT 'pending',  -- pending, sent, failed
    attempts INT NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMP,
    last_error TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW(),
    sent_at TIMESTAMP
);

CREATE INDEX idx_notification_queue_status ON notification_queue(status, next_attempt_at);
CREATE INDEX idx_notification_queue_event ON notification_queue(event_id);
```

### Изменения в существующих таблицах

```sql
-- Удаление старой модели подписок
DROP TABLE IF EXISTS subscription_services;
DROP TABLE IF EXISTS subscriptions;

-- notify_subscribers по умолчанию true
ALTER TABLE events ALTER COLUMN notify_subscribers SET DEFAULT true;
ALTER TABLE event_updates ALTER COLUMN notify_subscribers SET DEFAULT true;
```

### Итоговая схема

```
notification_channels (существует, расширяется)
├── id, user_id, type, target, is_enabled, is_verified
├── subscribe_to_all_services (новое, default false)
├── created_at, updated_at
│
├── channel_subscriptions (новая)
│   ├── channel_id FK → notification_channels
│   └── service_id FK → services
│   (используется только если subscribe_to_all_services = false)
│
└── event_subscribers (новая)
    ├── event_id FK → events
    └── channel_id FK → notification_channels

notification_queue (новая)
├── id, event_id, channel_id, message_type, payload
├── status, attempts, next_attempt_at, last_error
└── created_at, sent_at
```

### Логика подписок

```
Если subscribe_to_all_services = true:
  → Игнорировать channel_subscriptions
  → Подписан на все сервисы (включая новые)

Если subscribe_to_all_services = false:
  → Если channel_subscriptions пустой → НЕ подписан ни на что
  → Если channel_subscriptions не пустой → подписан на указанные сервисы
```

---

## 7. Контракт NotificationPayload

### Структура

```go
// internal/notifications/payload.go

type MessageType string

const (
    MessageTypeInitial   MessageType = "initial"
    MessageTypeUpdate    MessageType = "update"
    MessageTypeResolved  MessageType = "resolved"
    MessageTypeCompleted MessageType = "completed"
    MessageTypeCancelled MessageType = "cancelled"
)

type NotificationPayload struct {
    MessageType MessageType    `json:"message_type"`
    Event       EventData      `json:"event"`
    Changes     *EventChanges  `json:"changes,omitempty"`
    Resolution  *EventResolution `json:"resolution,omitempty"`
    EventURL    string         `json:"event_url,omitempty"`
    GeneratedAt time.Time      `json:"generated_at"`
}

type EventData struct {
    ID             string           `json:"id"`
    Title          string           `json:"title"`
    Type           string           `json:"type"`     // incident, maintenance
    Status         string           `json:"status"`
    Severity       string           `json:"severity"` // minor, major, critical (пусто для maintenance)
    Message        string           `json:"message"`
    Services       []AffectedService `json:"services"`
    Groups         []AffectedGroup   `json:"groups,omitempty"` // Группы для контекста
    CreatedAt      time.Time        `json:"created_at"`
    StartedAt      *time.Time       `json:"started_at,omitempty"`
    ScheduledStart *time.Time       `json:"scheduled_start,omitempty"`
    ScheduledEnd   *time.Time       `json:"scheduled_end,omitempty"`
}

type AffectedGroup struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

type AffectedService struct {
    ID     string `json:"id"`
    Name   string `json:"name"`
    Status string `json:"status"`
}

type EventChanges struct {
    StatusFrom      string                `json:"status_from,omitempty"`
    StatusTo        string                `json:"status_to,omitempty"`
    SeverityFrom    string                `json:"severity_from,omitempty"`
    SeverityTo      string                `json:"severity_to,omitempty"`
    ServicesAdded   []AffectedService     `json:"services_added,omitempty"`
    ServicesRemoved []AffectedService     `json:"services_removed,omitempty"`
    ServicesUpdated []ServiceStatusChange `json:"services_updated,omitempty"`
}

type ServiceStatusChange struct {
    ID         string `json:"id"`
    Name       string `json:"name"`
    StatusFrom string `json:"status_from"`
    StatusTo   string `json:"status_to"`
}

type EventResolution struct {
    ResolvedAt time.Time     `json:"resolved_at"`
    Duration   time.Duration `json:"duration"`
    Message    string        `json:"message"`
}
```

### Использование полей по типам сообщений

| Поле | initial | update | resolved | completed | cancelled |
|------|---------|--------|----------|-----------|-----------|
| MessageType | ✅ | ✅ | ✅ | ✅ | ✅ |
| Event.* | ✅ | ✅ | ✅ | ✅ | ✅ |
| Event.Severity | ✅¹ | ✅¹ | ✅¹ | — | — |
| Event.ScheduledStart/End | ✅² | ✅² | — | ✅² | ✅ |
| Changes.StatusFrom/To | — | ✅³ | ✅ | ✅ | — |
| Changes.Services* | — | ✅³ | — | — | — |
| Resolution.* | — | — | ✅ | ✅ | — |
| EventURL | ✅ | ✅ | ✅ | ✅ | — |

¹ Только для incident
² Только для maintenance
³ Заполняется если есть изменения

---

## 8. Шаблоны сообщений

### Подход

- Go templates с custom functions
- Отдельный шаблон для каждой комбинации: `{channel_type}_{message_type}.tmpl`
- Шаблоны встроены в бинарник через `embed`

### Пример: Email Initial

```
{{- if eq .Event.Type "incident" -}}
🔴 Incident: {{ .Event.Title }}
{{- else -}}
🔧 Scheduled Maintenance: {{ .Event.Title }}
{{- end }}

{{- if .Event.Services }}

Affected services:
{{- range .Event.Services }}
  • {{ .Name }} ({{ .Status }})
{{- end }}
{{- end }}

{{- if and (eq .Event.Type "incident") .Event.Severity }}
Severity: {{ .Event.Severity | title }}
{{- end }}
Status: {{ .Event.Status | title }}

{{- if .Event.ScheduledStart }}
Scheduled: {{ .Event.ScheduledStart | formatTime }} — {{ .Event.ScheduledEnd | formatTime }}
{{- else if .Event.StartedAt }}
Started: {{ .Event.StartedAt | formatTime }}
{{- end }}

{{ .Event.Message }}

---
View details: {{ .EventURL }}
```

### Пример: Email Update

```
📢 Update: {{ .Event.Title }}

{{- if and .Changes .Changes.StatusFrom }}

Status: {{ .Changes.StatusFrom | title }} → {{ .Changes.StatusTo | title }}
{{- end }}

{{- if and .Changes .Changes.ServicesAdded }}

Services added:
{{- range .Changes.ServicesAdded }}
  • {{ .Name }} ({{ .Status }})
{{- end }}
{{- end }}

{{- if and .Changes .Changes.ServicesRemoved }}

Services removed:
{{- range .Changes.ServicesRemoved }}
  • {{ .Name }}
{{- end }}
{{- end }}

{{- if and .Changes .Changes.ServicesUpdated }}

Service status changes:
{{- range .Changes.ServicesUpdated }}
  • {{ .Name }}: {{ .StatusFrom }} → {{ .StatusTo }}
{{- end }}
{{- end }}

{{- if .Event.Message }}

{{ .Event.Message }}
{{- end }}

---
View details: {{ .EventURL }}
```

### Пример: Email Resolved

```
🟢 Resolved: {{ .Event.Title }}

Duration: {{ .Resolution.Duration | formatDuration }}

{{- if .Resolution.Message }}

{{ .Resolution.Message }}
{{- end }}

---
View details: {{ .EventURL }}
```

### Пример: Email Cancelled

```
🚫 Cancelled: {{ .Event.Title }}

Originally scheduled: {{ .Event.ScheduledStart | formatTime }} — {{ .Event.ScheduledEnd | formatTime }}

This maintenance has been cancelled.
```

### Template Functions

```go
template.FuncMap{
    "title":          strings.Title,
    "formatTime":     func(t *time.Time) string { return t.Format("Jan 2, 2006 15:04 UTC") },
    "formatDuration": func(d time.Duration) string { ... },
}
```

---

## 9. Архитектура компонентов

### Диаграмма

```
┌─────────────────────────────────────────────────────────────────┐
│                         Events Module                           │
│                                                                 │
│  CreateEvent() ──────┐                                          │
│  AddUpdate() ────────┤                                          │
│  ResolveEvent() ─────┤                                          │
│  DeleteEvent() ──────┘                                          │
│                      │                                          │
└──────────────────────┼──────────────────────────────────────────┘
                       │ if event.NotifySubscribers
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                    Notification Service                         │
│                                                                 │
│  OnEventCreated(event, serviceIDs)                              │
│    → FindSubscribers(serviceIDs)                                │
│    → SaveEventSubscribers(eventID, subscribers)                 │
│    → BuildPayload(event, "initial")                             │
│    → Enqueue(payload, subscribers)                              │
│                                                                 │
│  OnEventUpdated(event, update, changes)                         │
│    → If services added: AddNewSubscribers()                     │
│    → BuildPayload(event, "update", changes)                     │
│    → Enqueue(payload, GetEventSubscribers())                    │
│                                                                 │
│  OnEventResolved(event)                                         │
│    → BuildPayload(event, "resolved")                            │
│    → Enqueue(payload, GetEventSubscribers())                    │
│                                                                 │
│  OnEventCancelled(event)                                        │
│    → BuildPayload(event, "cancelled")                           │
│    → Enqueue(payload, GetEventSubscribers())                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                       │
                       ▼
┌─────────────────────────────────────────────────────────────────┐
│                         Dispatcher                              │
│                    (background goroutine)                       │
│                                                                 │
│  Loop:                                                          │
│    1. Fetch pending from notification_queue                     │
│    2. Group by channel_type                                     │
│    3. Render template for each group                            │
│    4. Send via appropriate Sender                               │
│    5. Update status (sent / retry / failed)                     │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
                       │
       ┌───────────────┼───────────────┐
       ▼               ▼               ▼
┌───────────┐   ┌───────────┐   ┌───────────┐
│   Email   │   │ Telegram  │   │Mattermost │
│  Sender   │   │  Sender   │   │  Sender   │
│           │   │           │   │           │
│ SMTP/API  │   │ Bot API   │   │ Webhook   │
│ BCC batch │   │ Rate limit│   │           │
└───────────┘   └───────────┘   └───────────┘
```

### Компоненты

**NotificationService:**
- Точка входа для events module
- Управляет подписчиками события
- Формирует NotificationPayload
- Ставит в очередь

**Dispatcher:**
- Background goroutine
- Читает из `notification_queue`
- Рендерит шаблоны
- Вызывает Senders
- Обрабатывает retry

**Senders:**
- Интерфейс: `Send(ctx, to, subject, body) error`
- Реализации: EmailSender, TelegramSender, MattermostSender
- Каждый знает специфику своего канала

**Renderer:**
- Загружает и кэширует шаблоны
- Рендерит NotificationPayload в текст

### Retry логика

```go
type RetryConfig struct {
    MaxAttempts       int           // 3
    InitialBackoff    time.Duration // 1s
    MaxBackoff        time.Duration // 5m
    BackoffMultiplier float64       // 2.0
}
```

**Алгоритм:**
1. Попытка 1: немедленно
2. Попытка 2: через 1s
3. Попытка 3: через 2s
4. После 3 неудач: статус `failed`, требует ручного разбора

**Retryable ошибки:**
- HTTP 429 (Too Many Requests)
- HTTP 5xx
- Network errors

**Non-retryable:**
- HTTP 4xx (кроме 429)
- Invalid credentials
- Chat not found (Telegram)

---

## 10. Конфигурация

### ENV переменные

```bash
# Общие
NOTIFICATIONS_ENABLED=true

# Email
NOTIFICATIONS_EMAIL_ENABLED=true
NOTIFICATIONS_EMAIL_SMTP_HOST=smtp.example.com
NOTIFICATIONS_EMAIL_SMTP_PORT=587
NOTIFICATIONS_EMAIL_SMTP_USER=notifications@example.com
NOTIFICATIONS_EMAIL_SMTP_PASSWORD=secret
NOTIFICATIONS_EMAIL_FROM_ADDRESS="StatusPage <notifications@example.com>"
NOTIFICATIONS_EMAIL_BATCH_SIZE=50

# Telegram
NOTIFICATIONS_TELEGRAM_ENABLED=true
NOTIFICATIONS_TELEGRAM_BOT_TOKEN=123456:ABC-DEF
NOTIFICATIONS_TELEGRAM_RATE_LIMIT=25

# Retry
NOTIFICATIONS_RETRY_MAX_ATTEMPTS=3
NOTIFICATIONS_RETRY_INITIAL_BACKOFF=1s
NOTIFICATIONS_RETRY_MAX_BACKOFF=5m
NOTIFICATIONS_RETRY_BACKOFF_MULTIPLIER=2.0
```

### Config struct

```go
type NotificationsConfig struct {
    Enabled  bool          `koanf:"enabled"`
    Email    EmailConfig   `koanf:"email"`
    Telegram TelegramConfig `koanf:"telegram"`
    Retry    RetryConfig   `koanf:"retry"`
}

type EmailConfig struct {
    Enabled     bool   `koanf:"enabled"`
    SMTPHost    string `koanf:"smtp_host"`
    SMTPPort    int    `koanf:"smtp_port"`
    SMTPUser    string `koanf:"smtp_user"`
    SMTPPassword string `koanf:"smtp_password"`
    FromAddress string `koanf:"from_address"`
    BatchSize   int    `koanf:"batch_size"`
}

type TelegramConfig struct {
    Enabled   bool    `koanf:"enabled"`
    BotToken  string  `koanf:"bot_token"`
    RateLimit float64 `koanf:"rate_limit"`
}

type RetryConfig struct {
    MaxAttempts       int           `koanf:"max_attempts"`
    InitialBackoff    time.Duration `koanf:"initial_backoff"`
    MaxBackoff        time.Duration `koanf:"max_backoff"`
    BackoffMultiplier float64       `koanf:"backoff_multiplier"`
}
```

### Kubernetes

```yaml
env:
  - name: NOTIFICATIONS_ENABLED
    value: "true"
  - name: NOTIFICATIONS_EMAIL_SMTP_PASSWORD
    valueFrom:
      secretKeyRef:
        name: notification-secrets
        key: smtp-password
  - name: NOTIFICATIONS_TELEGRAM_BOT_TOKEN
    valueFrom:
      secretKeyRef:
        name: notification-secrets
        key: telegram-bot-token
```

---

## 11. API Endpoints

### Каналы

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/me/channels` | Список каналов пользователя | User |
| POST | `/api/v1/me/channels` | Создать канал | User |
| PATCH | `/api/v1/me/channels/{id}` | Обновить (is_enabled) | User |
| DELETE | `/api/v1/me/channels/{id}` | Удалить канал | User |
| POST | `/api/v1/me/channels/{id}/verify` | Подтвердить код (email) | User |

### Подписки

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| GET | `/api/v1/me/subscriptions` | Матрица подписок | User |
| PUT | `/api/v1/me/channels/{id}/subscriptions` | Установить сервисы для канала | User |

### События (изменение)

| Method | Path | Description | Auth |
|--------|------|-------------|------|
| DELETE | `/api/v1/events/{id}` | Для scheduled: "cancelled" + удаление | Admin |

### Примеры запросов

**Создание Email канала:**
```http
POST /api/v1/me/channels
{
    "type": "email",
    "target": "user@example.com"
}

Response 201:
{
    "data": {
        "id": "ch-123",
        "type": "email",
        "target": "user@example.com",
        "is_enabled": true,
        "is_verified": false
    }
}
```

**Верификация Email:**
```http
POST /api/v1/me/channels/ch-123/verify
{
    "code": "123456"
}

Response 200:
{
    "data": {
        "id": "ch-123",
        "is_verified": true
    }
}
```

**Получение матрицы подписок:**
```http
GET /api/v1/me/subscriptions

Response 200:
{
    "data": {
        "channels": [
            {
                "id": "ch-123",
                "type": "email",
                "target": "user@example.com",
                "is_verified": true,
                "subscribe_to_all_services": false,
                "subscribed_service_ids": ["svc-1", "svc-2"]
            },
            {
                "id": "ch-456",
                "type": "telegram",
                "target": "789012345",
                "is_verified": true,
                "subscribe_to_all_services": true,
                "subscribed_service_ids": []  // игнорируется когда subscribe_to_all_services = true
            },
            {
                "id": "ch-789",
                "type": "mattermost",
                "target": "https://mm.company.com/hooks/xxx",
                "is_verified": true,
                "subscribe_to_all_services": false,
                "subscribed_service_ids": []  // не подписан ни на что
            }
        ]
    }
}
```

**Установка подписок для канала:**
```http
PUT /api/v1/me/channels/ch-123/subscriptions
{
    "subscribe_to_all_services": false,
    "service_ids": ["svc-1", "svc-3"]
}

Response 200:
{
    "data": {
        "channel_id": "ch-123",
        "subscribe_to_all_services": false,
        "subscribed_service_ids": ["svc-1", "svc-3"]
    }
}
```

**Подписка на все сервисы:**
```http
PUT /api/v1/me/channels/ch-456/subscriptions
{
    "subscribe_to_all_services": true
}

Response 200:
{
    "data": {
        "channel_id": "ch-456",
        "subscribe_to_all_services": true,
        "subscribed_service_ids": []
    }
}
```

---

## 12. UI/UX

### Страница "Уведомления" (пользователь)

**Первое посещение (пустое состояние):**

```
┌─────────────────────────────────────────────────────────────────┐
│  Уведомления                                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │  📭 У вас пока нет настроенных уведомлений              │    │
│  │                                                         │    │
│  │  Добавьте канал (Email, Telegram) и выберите сервисы,   │    │
│  │  о событиях которых хотите получать уведомления.        │    │
│  │                                                         │    │
│  │  [+ Добавить канал]                                     │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**С настроенными каналами:**

```
┌─────────────────────────────────────────────────────────────────┐
│  Уведомления                                                    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Каналы доставки                                                │
│  ───────────────                                                │
│                                                                 │
│  ┌────────────────────────────────────────────────────────────┐ │
│  │ ✉️  user@example.com                    ✓ Verified  [Вкл]  │ │
│  │     Email                                          [✕]     │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │ 📱 123456789                            ✓ Verified  [Вкл]  │ │
│  │     Telegram                                       [✕]     │ │
│  ├────────────────────────────────────────────────────────────┤ │
│  │ 💬 https://mm.company.com/hooks/xxx     ✓ Verified  [Выкл] │ │
│  │     Mattermost                                     [✕]     │ │
│  └────────────────────────────────────────────────────────────┘ │
│                                                                 │
│  [+ Добавить канал]                                             │
│                                                                 │
│                                                                 │
│  Подписки на сервисы                                            │
│  ───────────────────                                            │
│                                                                 │
│  Выберите, о каких сервисах получать уведомления.               │
│                                                                 │
│                           ┌───────┬──────────┬────────────┐     │
│                           │ Email │ Telegram │ Mattermost │     │
│  ┌────────────────────────┼───────┼──────────┼────────────┤     │
│  │ [✓] Все сервисы        │  [ ]  │   [✓]    │    [ ]     │     │
│  │     (включая новые)    │       │          │            │     │
│  ├────────────────────────┼───────┼──────────┼────────────┤     │
│  │ ▼ Backend              │       │          │            │     │
│  │   ├─ API Gateway       │  [✓]  │   —      │    [ ]     │     │
│  │   ├─ Database          │  [✓]  │   —      │    [ ]     │     │
│  │   └─ Auth Service      │  [ ]  │   —      │    [ ]     │     │
│  │ ▼ Frontend             │       │          │            │     │
│  │   ├─ Web App           │  [ ]  │   —      │    [ ]     │     │
│  │   └─ CDN               │  [ ]  │   —      │    [ ]     │     │
│  └────────────────────────┴───────┴──────────┴────────────┘     │
│                                                                 │
│  — = "Все сервисы" включено, индивидуальный выбор не требуется  │
│                                                                 │
│  [Сохранить]                                                    │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Модалка "Добавить канал"

**Выбор типа:**
```
┌─────────────────────────────────────────────────────────────────┐
│  Добавить канал уведомлений                              [✕]    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Выберите тип:                                                  │
│                                                                 │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐           │
│  │    ✉️        │  │    📱        │  │    💬        │          │
│  │   Email      │  │  Telegram    │  │  Mattermost  │           │
│  └──────────────┘  └──────────────┘  └──────────────┘           │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Telegram (после выбора):**
```
┌─────────────────────────────────────────────────────────────────┐
│  Добавить Telegram                                       [✕]    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Chat ID: [_______________]                                     │
│                                                                 │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │ ℹ️  Как подключить:                                     │    │
│  │                                                         │    │
│  │ 1. Напишите /start боту @YourStatusBot                  │    │
│  │ 2. Узнайте свой Chat ID у @userinfobot                  │    │
│  │ 3. Введите Chat ID в поле выше                          │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│                                    [Отмена]  [Добавить]         │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

**Email верификация:**
```
┌─────────────────────────────────────────────────────────────────┐
│  Подтверждение email                                     [✕]    │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Мы отправили код на user@example.com                           │
│                                                                 │
│  Введите код: [ _ ] [ _ ] [ _ ] [ _ ] [ _ ] [ _ ]               │
│                                                                 │
│  Не получили? [Отправить повторно]                              │
│                                                                 │
│                                    [Отмена]  [Подтвердить]      │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

### Создание события (оператор)

```
┌─────────────────────────────────────────────────────────────────┐
│  Создать событие                                                │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  Тип:  (•) Incident    ( ) Maintenance                          │
│                                                                 │
│  Название: [________________________________]                   │
│                                                                 │
│  Severity: [Major ▼]                                            │
│                                                                 │
│  Затронутые сервисы:                                            │
│  [✓] API Gateway                                                │
│  [✓] Database                                                   │
│  [ ] Auth Service                                               │
│  [ ] Payment Gateway                                            │
│                                                                 │
│  Описание:                                                      │
│  ┌─────────────────────────────────────────────────────────┐    │
│  │                                                         │    │
│  │                                                         │    │
│  └─────────────────────────────────────────────────────────┘    │
│                                                                 │
│  [✓] Уведомить подписчиков              ← включено по умолчанию │
│                                                                 │
│                                    [Отмена]  [Создать]          │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## 13. Отложенный функционал

| Функционал | Причина откладывания | Приоритет |
|------------|---------------------|-----------|
| Telegram webhook (auto chat_id) | Улучшение UX, не блокер | Medium |
| Reminder для scheduled maintenance | Усложняет систему | Low |
| Mute конкретного события | Polish phase | Low |
| Notification preferences (severity filter) | Не нужно на старте | Low |
| Quiet hours | Решается на уровне телефона/клиента | Low |
| Подписка после создания события | Редкий edge case | Low |
| Ручная отправка (если забыл галочку) | notify=true по умолчанию решает | Low |
| UI админки для SMTP/Telegram настроек | Конфигурация через ENV достаточна | Low |

### Подготовка к будущим изменениям

**Mute события:**
- Добавить колонку `muted_at TIMESTAMP` в `event_subscribers`
- При отправке: `WHERE muted_at IS NULL`

**Telegram webhook:**
- Endpoint `/internal/telegram/webhook`
- При создании канала генерировать verification_token
- Ссылка: `t.me/Bot?start={token}`

**Notification preferences:**
- Добавить колонку `preferences JSONB` в `notification_channels`
- Фильтровать при формировании payload

---

## Changelog

| Версия | Дата | Изменения |
|--------|------|-----------|
| 1.0 | 2024-01 | Первоначальная версия |
