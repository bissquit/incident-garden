# Notification Implementation Plan

> План реализации системы уведомлений для IncidentGarden.
> Связанный документ: [notification-architecture.md](./notification-architecture.md)

---

## Обзор

**Цель:** Реализовать полнофункциональную систему уведомлений о событиях (incidents, maintenance).

**Принципы:**
- Каждый этап — отдельный PR
- Каждый PR самодостаточен и не ломает существующий функционал
- Инкрементальная доставка ценности

---

## Текущее состояние

| Компонент | Статус | Файлы |
|-----------|--------|-------|
| Domain структуры | ✅ Готово | `internal/domain/notification.go`, `subscription.go` |
| Repository интерфейс | ✅ Готово | `internal/notifications/repository.go` |
| PostgreSQL реализация | ✅ Готово | `internal/notifications/postgres/repository.go` |
| HTTP Handler | ✅ Готово | `internal/notifications/handler.go` |
| Service | ✅ Готово | `internal/notifications/service.go` |
| Dispatcher | ✅ Готово | `internal/notifications/dispatcher.go` |
| Email Sender | 🔴 STUB | `internal/notifications/email/sender.go` |
| Telegram Sender | 🔴 STUB | `internal/notifications/telegram/sender.go` |
| Mattermost Sender | 🔴 Нет | — |
| Конфигурация | 🔴 Нет | — |
| Events интеграция | 🔴 Нет | — |
| Тесты | 🔴 Нет | — |

---

## Этапы реализации

### Этап 1: Миграция БД — новая модель подписок

**Цель:** Перейти от модели "подписка на уровне user" к "подписка на уровне channel".

**Изменения в БД:**
- Добавить `subscribe_to_all_services BOOLEAN` в `notification_channels`
- Добавить `mattermost` в constraint типов каналов
- Создать `channel_subscriptions(channel_id, service_id)`
- Создать `event_subscribers(event_id, channel_id)`
- Миграция данных из `subscriptions` → `channel_subscriptions`
- Удалить старые таблицы `subscriptions`, `subscription_services`

**Файлы:**
- `migrations/NNNNNN_notification_subscriptions_refactor.up.sql`
- `migrations/NNNNNN_notification_subscriptions_refactor.down.sql`
- `internal/notifications/repository.go` — обновить интерфейс
- `internal/notifications/postgres/repository.go` — обновить реализацию
- `internal/domain/notification.go` — добавить `SubscribeToAllServices`
- `internal/domain/subscription.go` — удалить или переработать

**Статус:** ⬜ Не начато

---

### Этап 2: Конфигурация notifications

**Цель:** Добавить конфигурацию для Email, Telegram, retry в config.

**Изменения:**
- Добавить `NotificationsConfig` в `internal/config/config.go`
- Обновить `internal/app/app.go` — передавать config в senders
- Обновить документацию `docs/deployment.md`

**Параметры:**
```
NOTIFICATIONS_ENABLED
NOTIFICATIONS_EMAIL_ENABLED, SMTP_HOST, SMTP_PORT, SMTP_USER, SMTP_PASSWORD, FROM_ADDRESS, BATCH_SIZE
NOTIFICATIONS_TELEGRAM_ENABLED, BOT_TOKEN, RATE_LIMIT
NOTIFICATIONS_RETRY_MAX_ATTEMPTS, INITIAL_BACKOFF, MAX_BACKOFF, BACKOFF_MULTIPLIER
```

**Статус:** ⬜ Не начато

---

### Этап 3: NotificationPayload + Renderer

**Цель:** Создать контракт данных и систему рендеринга шаблонов.

**Файлы:**
- `internal/notifications/payload.go` — структуры NotificationPayload, EventData, etc.
- `internal/notifications/renderer.go` — Renderer с Go templates
- `internal/notifications/templates/*.tmpl` — шаблоны для email, telegram, mattermost
- `internal/notifications/templates/embed.go` — embed templates

**Типы сообщений:**
- `initial` — событие создано
- `update` — апдейт события
- `resolved` — инцидент закрыт
- `completed` — maintenance завершён
- `cancelled` — scheduled отменён

**Статус:** ⬜ Не начато

---

### Этап 4: Email Sender (реальный)

**Цель:** Реализовать отправку email через SMTP.

**Изменения:**
- `internal/notifications/email/sender.go` — SMTP реализация
- BCC batching (по 50 получателей)
- Обработка ошибок SMTP

**Тестирование:**
- Unit тесты с mock SMTP
- Опционально: интеграционные тесты с MailHog в Docker

**Статус:** ⬜ Не начато

---

### Этап 5: Telegram Sender (реальный)

**Цель:** Реализовать отправку сообщений через Telegram Bot API.

**Изменения:**
- `internal/notifications/telegram/sender.go` — HTTP клиент для Bot API
- Rate limiting (25 msg/sec)
- Parse mode: Markdown

**API:**
```
POST https://api.telegram.org/bot<TOKEN>/sendMessage
{
    "chat_id": "123456789",
    "text": "message",
    "parse_mode": "Markdown"
}
```

**Тестирование:**
- Unit тесты с mock HTTP
- Опционально: интеграционные тесты с реальным ботом (test environment)

**Статус:** ⬜ Не начато

---

### Этап 6: Mattermost Sender

**Цель:** Добавить поддержку Mattermost webhooks.

**Изменения:**
- `internal/notifications/mattermost/sender.go` — HTTP POST на webhook
- `internal/domain/notification.go` — добавить `ChannelTypeMattermost`
- Обновить app.go — инициализация sender

**API:**
```
POST <webhook_url>
{
    "text": "message",
    "username": "StatusPage"
}
```

**Тестирование:**
- Unit тесты с mock HTTP

**Статус:** ⬜ Не начато

---

### Этап 7: Email верификация

**Цель:** Реализовать верификацию email через код подтверждения.

**Изменения:**
- Добавить таблицу `channel_verification_codes(channel_id, code, expires_at)`
- `internal/notifications/service.go` — генерация и проверка кода
- `internal/notifications/handler.go` — обновить `/verify` endpoint
- Отправка кода при создании email канала

**Flow:**
1. POST /me/channels {type: email} → генерация 6-значного кода
2. Отправка email с кодом
3. POST /me/channels/{id}/verify {code: "123456"} → проверка и установка is_verified

**Статус:** ⬜ Не начато

---

### Этап 8: API подписок (новая модель)

**Цель:** Обновить API для работы с подписками на уровне канала.

**Изменения:**
- `internal/notifications/handler.go` — новые endpoints
- `internal/notifications/service.go` — новая логика
- `api/openapi/openapi.yaml` — обновить схемы

**Endpoints:**
```
GET  /api/v1/me/subscriptions           — матрица подписок (все каналы + сервисы)
PUT  /api/v1/me/channels/{id}/subscriptions — установить подписки для канала
```

**UI поддержка:**
- `subscribe_to_all_services: bool`
- `service_ids: []string`
- Группировка сервисов (только для отображения)

**Статус:** ⬜ Не начато

---

### Этап 9: Event subscribers + интеграция

**Цель:** Интегрировать notifications с events module.

**Изменения:**
- `internal/notifications/service.go`:
  - `OnEventCreated(event, serviceIDs)`
  - `OnEventUpdated(event, update, changes)`
  - `OnEventResolved(event)`
  - `OnEventCancelled(event)`
  - `FindSubscribersForServices(serviceIDs)`
  - `SaveEventSubscribers(eventID, channelIDs)`
- `internal/events/service.go`:
  - Добавить `notifier` интерфейс
  - Вызывать notifier при CRUD событий
- `internal/notifications/repository.go`:
  - `CreateEventSubscribers(eventID, channelIDs)`
  - `GetEventSubscribers(eventID)`
  - `AddEventSubscribers(eventID, channelIDs)` — для добавления сервисов

**Интерфейс:**
```go
type EventNotifier interface {
    OnEventCreated(ctx, event, serviceIDs) error
    OnEventUpdated(ctx, event, update, changes) error
    OnEventResolved(ctx, event) error
    OnEventCancelled(ctx, event) error
}
```

**Статус:** ⬜ Не начато

---

### Этап 10: Notification queue + retry

**Цель:** Добавить очередь и механизм повторных попыток.

**Изменения:**
- Создать таблицу `notification_queue`
- `internal/notifications/queue.go` — логика очереди
- `internal/notifications/dispatcher.go` — использовать очередь
- Background goroutine для обработки очереди
- Exponential backoff для retry

**Параметры retry:**
- Max attempts: 3
- Initial backoff: 1s
- Max backoff: 5m
- Multiplier: 2.0

**Статусы в очереди:**
- `pending` — ожидает отправки
- `sent` — успешно отправлено
- `failed` — все попытки исчерпаны

**Статус:** ⬜ Не начато

---

### Этап 11: Интеграционные тесты

**Цель:** Покрыть notifications module интеграционными тестами.

**Файлы:**
- `tests/integration/notifications_channels_test.go` — CRUD каналов
- `tests/integration/notifications_subscriptions_test.go` — подписки
- `tests/integration/notifications_dispatch_test.go` — отправка уведомлений
- `tests/integration/notifications_events_test.go` — интеграция с events

**Покрытие:**
- Создание/обновление/удаление каналов
- Верификация email (с mock SMTP)
- Подписки на сервисы
- Отправка при создании события
- Отправка при обновлении события
- Отправка при закрытии события
- Отмена scheduled maintenance
- Retry логика
- Rate limiting (Telegram)

**Статус:** ⬜ Не начато

---

## Зависимости между этапами

```
Этап 1 (БД) ─────────────────────────────────┐
                                             │
Этап 2 (Config) ─────────────────────────────┼──→ Этап 4 (Email)
                                             │      │
Этап 3 (Payload) ────────────────────────────┤      ├──→ Этап 7 (Верификация)
                                             │      │
                                             ├──→ Этап 5 (Telegram)
                                             │
                                             ├──→ Этап 6 (Mattermost)
                                             │
Этап 1 + Этап 3 ─────────────────────────────┼──→ Этап 8 (API подписок)
                                             │
                                             └──→ Этап 9 (Events интеграция)
                                                       │
                                                       └──→ Этап 10 (Queue)
                                                              │
                                                              └──→ Этап 11 (Тесты)
```

**Критический путь:** 1 → 2 → 4 → 7, 1 → 3 → 9 → 10 → 11

**Можно делать параллельно:**
- Этапы 4, 5, 6 (senders) — после этапа 2
- Этапы 7 и 8 — независимы друг от друга

---

## Оценка трудозатрат

| Этап | Сложность | Примерный объём |
|------|-----------|-----------------|
| 1. БД миграция | Средняя | 2 миграции, 3 файла |
| 2. Конфигурация | Низкая | 2 файла |
| 3. Payload + Renderer | Средняя | 3 файла, 5 шаблонов |
| 4. Email Sender | Средняя | 1 файл, тесты |
| 5. Telegram Sender | Низкая | 1 файл, тесты |
| 6. Mattermost Sender | Низкая | 2 файла |
| 7. Email верификация | Средняя | 3 файла, миграция |
| 8. API подписок | Средняя | 3 файла, OpenAPI |
| 9. Events интеграция | Высокая | 4 файла |
| 10. Queue + Retry | Высокая | 3 файла, миграция |
| 11. Тесты | Высокая | 4 файла |

---

## Changelog

| Дата | Изменение |
|------|-----------|
| 2024-01 | Первоначальный план |
