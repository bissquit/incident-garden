# CLAUDE.md — IncidentGarden

> Open-source self-hosted status page service. Alternative to Atlassian Statuspage, Cachet, Instatus.

> **MANDATORY: Keep This File Updated when changes affect:** database schema, API endpoints, domain types, module interfaces, business rules, or project status. Before completing any task, verify CODEMAP, schema, API endpoints, and STATUS sections are current.

---

## 1. PROJECT CONTEXT

- **Core:** Service status display + incident/maintenance management + notifications
- **Architecture:** Modular monolith (Go), REST API-first
- **Modules:** `identity` (auth/RBAC) → `catalog` (services/groups) → `events` (incidents/maintenance) → `notifications` (channels/dispatch)

### Tech Stack

- **Go 1.25**, chi (router), pgx (PostgreSQL 16), koanf (config), slog (logging)
- prometheus/client_golang, go-playground/validator, golang-migrate
- Docker, testcontainers-go, GitHub Actions

### Domain Concepts

| Concept          | Values                                                                     |
|------------------|----------------------------------------------------------------------------|
| Service statuses | `operational`, `degraded`, `partial_outage`, `major_outage`, `maintenance` |
| Incident flow    | `investigating` → `identified` → `monitoring` → `resolved`                 |
| Maintenance flow | `scheduled` → `in_progress` → `completed`                                  |
| Severity         | `minor`, `major`, `critical`                                               |
| Roles            | `user` → `operator` → `admin`                                              |
| Channel types    | `email`, `telegram`, `mattermost`                                          |

### Key Architectural Decisions

**M:N Services ↔ Groups:** Service belongs to multiple groups via `service_group_members` junction. API uses `group_ids: []string`.

**Events with Groups:** Groups auto-expand to services at creation time. `event_groups` stores selected groups, `event_services` stores flattened services, `event_service_changes` tracks audit trail.

**Soft Delete:** Services/groups use `archived_at`. Hidden from lists by default (`include_archived=true` to show). Cannot archive with active events. Archived items remain in historical events.

---

## 2. CODEMAP

Each module follows the pattern: `handler.go` (HTTP) → `service.go` (logic) → `repository.go` (interface) → `postgres/repository.go` (SQL)

```
api/openapi/openapi.yaml           # API contract (source of truth for endpoints)
migrations/                        # golang-migrate SQL migrations (000001–000020)
deployments/prometheus/            # alerts.yaml, servicemonitor.yaml
docs/deployment.md                 # ENV vars, K8s config, Prometheus setup

internal/
├── app/app.go                     # DI wiring, router setup, server lifecycle
├── config/config.go               # koanf-based config from ENV
│
├── domain/                        # Entities, enums, domain errors (no infra)
│   ├── user.go                    # User, Role, RefreshToken
│   ├── service.go                 # Service, ServiceGroup, ServiceWithEffectiveStatus, ServiceTag, ServiceStatusLogEntry
│   ├── event.go                   # Event, EventUpdate, EventService, EventServiceChange, AffectedService, AffectedGroup
│   ├── notification.go            # NotificationChannel, ChannelType
│   └── template.go               # EventTemplate, TemplateData (macros: ServiceName, StartedAt, etc.)
│
├── identity/                      # Auth (register/login/refresh/logout), JWT, RBAC
│   ├── handler.go                 # POST /auth/register, /login, /refresh, /logout; GET /me
│   ├── service.go                 # CreateUser, Authenticate, RefreshTokens
│   ├── authenticator.go           # Authenticator interface
│   ├── repository.go              # UserRepository, TokenRepository interfaces
│   ├── jwt/authenticator.go       # JWT implementation
│   └── postgres/repository.go
│   # Middleware: RequireAuth, RequireRole — used by all protected routes
│   # Creates default email channel on registration via notifications.Service
│
├── catalog/                       # CRUD services/groups, M:N membership, soft delete, tags
│   ├── handler.go                 # CRUD /services, /groups, /restore, /tags, /{slug}/events
│   ├── service.go                 # Business rules (archive checks, status updates)
│   ├── repository.go              # M:N, soft delete, effective status, status log, validation
│   ├── postgres/repository.go     # SQL with archived_at filtering
│   └── service_test.go
│   # Exposes interfaces for events module: GroupServiceResolver, CatalogServiceUpdater
│
├── events/                        # Incidents/maintenance lifecycle, composition changes
│   ├── handler.go                 # CRUD /events, /updates, /changes, /templates
│   ├── service.go                 # CreateEvent, AddUpdate (orchestrates status + services + audit)
│   ├── resolver.go                # GroupServiceResolver, CatalogServiceUpdater, EventNotifier interfaces
│   ├── repository.go              # Events, groups, services, changes — with Tx variants
│   ├── template_renderer.go       # Go template execution for notifications
│   ├── errors.go                  # ErrEventNotFound, ErrInvalidTransition, etc.
│   ├── postgres/repository.go
│   └── service_test.go
│   # Depends on: catalog.Service (resolver), notifications.Notifier (EventNotifier)
│
├── notifications/                 # Channels, verification, subscriptions, dispatch
│   ├── handler.go                 # CRUD /me/channels, /verify, /resend-code, /subscriptions, /config
│   ├── service.go                 # Channel CRUD, verification, subscriptions, channel type checks
│   ├── notifier.go                # Implements EventNotifier: queues notifications on event lifecycle
│   ├── dispatcher.go              # Finds subscribers, sends via queue
│   ├── worker.go                  # Background queue processor with exponential backoff retry
│   ├── renderer.go                # Template rendering for notification messages
│   ├── payload.go                 # NotificationPayload, EventData, EventChanges
│   ├── queue.go                   # QueueItem, QueueStatus types
│   ├── sender.go                  # Sender interface (Send, Type)
│   ├── metrics.go                 # Prometheus: queue size, send duration
│   ├── errors.go                  # ErrChannelNotFound, ErrVerificationFailed, etc.
│   ├── repository.go              # Channels, subscriptions, event subscribers, queue ops
│   ├── postgres/repository.go
│   ├── email/sender.go            # SMTP sender
│   ├── telegram/sender.go         # Telegram Bot API sender
│   ├── mattermost/sender.go       # Mattermost webhook sender
│   └── templates/                 # Embedded .tmpl files (email/telegram/mattermost × initial/update/resolved/completed/cancelled)
│
├── pkg/                           # Shared infra (no business logic)
│   ├── httputil/                  # response.go, middleware.go, errors.go, logging.go, metrics.go
│   ├── postgres/postgres.go       # Connect with retry + exponential backoff
│   ├── metrics/                   # Prometheus collectors (HTTP, DB pool, Go runtime)
│   └── ctxlog/ctxlog.go           # Context-aware slog with request_id
│
├── testutil/                      # Test infrastructure
│   ├── client.go                  # HTTP test client with auth helpers
│   ├── container.go               # testcontainers-go PostgreSQL setup
│   ├── fixtures.go                # Test data builders
│   └── openapi_validator.go       # Response validation against OpenAPI spec
│
└── version/version.go             # Build info (injected at compile time)

tests/integration/                 # Integration tests (testcontainers, //go:build integration)
├── main_test.go                   # TestMain, DB setup
├── helpers_test.go                # createTestService, createTestGroup, createTestIncident, etc.
├── mocks_test.go                  # Mock senders for notification tests
├── auth_test.go, rbac_test.go     # Identity module
├── catalog_service_test.go        # Service CRUD
├── catalog_group_test.go          # Group CRUD and membership
├── catalog_archive_test.go        # Soft delete, restore
├── catalog_status_test.go         # Effective status, status log
├── catalog_service_events_test.go # GET /services/{slug}/events
├── events_lifecycle_test.go       # Event creation, status transitions
├── events_composition_test.go     # Add/remove services, updates
├── events_maintenance_test.go     # Maintenance lifecycle
├── events_delete_test.go          # Event deletion, cascade
├── events_public_test.go          # Public endpoints
├── notifications_channels_test.go # Channel CRUD
├── notifications_default_channel_test.go  # Default email channel on registration
├── notifications_subscriptions_test.go    # Subscriptions API
├── notifications_verification_test.go     # Verification flow
├── notifications_queue_test.go    # Queue operations, retry
├── notifications_dispatch_test.go # Dispatcher
├── notifications_events_test.go   # Event-notification integration
└── notifications_email_e2e_test.go # Email E2E with Mailpit
```

### Database Schema

**Core tables:** `services`, `service_groups` — both with soft delete (`archived_at`)

**Junctions:** `service_group_members` (M:N services↔groups), `event_services` (M:N with `status`), `event_groups`, `channel_subscriptions`, `event_subscribers`

**Events:** `events`, `event_service_changes` (audit trail with `batch_id`, `action`, `service_id`, `group_id`)

**Status tracking:** `service_status_log` (source_type: manual/event/webhook, links to event_id), `v_service_effective_status` (VIEW — worst-case priority across active events)

**Notifications:** `notification_channels` (type: email/telegram/mattermost, `is_default`, `is_verified`), `channel_verification_codes`, `notification_queue` (async delivery with retry: pending→processing→sent/failed)

---

## 3. WORKFLOW

### Algorithm for Any Task

1. **Clarify:** module, endpoint/schema change, roles, backward compat
2. **Contract first:** OpenAPI or migration before code
3. **Boundaries:** handler/service/repository/domain/pkg (see Layer Responsibilities)
4. **Top-down:** handler → service → repository → migrations
5. **Errors:** wrap with context (`fmt.Errorf("...: %w", err)`)
6. **Tests:** unit for logic, integration for DB paths
7. **Validate:** `make lint && make test && make build`
8. **Update CLAUDE.md** if you changed schema, API, domain types, or interfaces

### OpenAPI Versioning

Version in `api/openapi/openapi.yaml` is **independent** from app version. Bump: **MAJOR** (breaking), **MINOR** (new feature), **PATCH** (fix/clarification). Don't bump for infra-only changes.

### Definition of Done (PR Checklist)

- [ ] Layer boundaries respected (no business logic in handlers, no SQL in services)
- [ ] Errors wrapped with context
- [ ] OpenAPI updated + version bumped if API changed
- [ ] CLAUDE.md updated if schema/API/domain/interfaces/rules changed
- [ ] Tests per Test Matrix
- [ ] `make lint && make test && make build` pass

---

## 4. ARCHITECTURE

### Layer Responsibilities

| Layer      | File            | Does                                     | Does NOT            |
|------------|-----------------|------------------------------------------|---------------------|
| Handler    | `handler.go`    | HTTP I/O, auth, validation, error→HTTP   | Business logic, SQL |
| Service    | `service.go`    | Use-cases, business rules, orchestration | SQL, HTTP concerns  |
| Repository | `repository.go` | Interface definition                     | Implementation      |
| Repo Impl  | `postgres/*.go` | SQL/pgx data access                      | Business decisions  |
| Domain     | `domain/*.go`   | Entities, domain errors                  | Infrastructure      |
| Pkg        | `pkg/*`         | Shared infra utilities                   | Business logic      |

### Principles

1. **Simplicity > Flexibility** — no abstractions "just in case"
2. **10/20 Rule** — >20% complexity for <10% value → postpone
3. **API-first** — contract before implementation
4. **No circular deps** between modules

### Anti-patterns (DON'T)

ORM (use pgx) · God-objects · Ignored errors · Hardcoded config (use ENV/koanf) · Business logic in handlers · Circular module deps · Features without tests · Skipping linters

---

## 5. CODE STYLE

- **Errors:** always check and wrap with `fmt.Errorf("context: %w", err)`
- **Empty slices:** `make([]Item, 0)` for JSON `[]` not `null`
- **Soft delete:** archive in repository, business rule check in service layer
- **Linters:** `make lint` — zero tolerance, PR cannot merge with errors

---

## 6. TESTING

### Strategy & Matrix

```
Unit (70%) — pure logic, mocked deps | Integration (25%) — real Postgres (testcontainers) | E2E (5%)
```

| Change                   | Unit     | Integration    |
|--------------------------|----------|----------------|
| Repository SQL           | —        | Required       |
| Service business rules   | Required | If DB involved |
| Handler/validation/roles | —        | Required       |
| Soft delete / M:N        | —        | Required       |

### Commands

```bash
make test                # All
make test-unit           # Unit only
make test-integration    # Integration (testcontainers)
```

### Integration Test Conventions

- Build tag: `//go:build integration` + `package integration`
- Naming: `Test<Module>_<Entity>_<Action>_<Scenario>`
- `require` for setup (test stops on failure), `assert` for verification (shows all failures)
- Always decode and verify response data, not just HTTP status code
- Use helpers from `helpers_test.go`: `createTestService`, `createTestGroup`, `createTestIncident`, `resolveEvent`, etc. with `t.Cleanup`
- Cover: happy path, validation errors (400), not found (404), conflict (409), auth/RBAC (401/403), edge cases
- New test files: `tests/integration/<module>_<domain>_test.go`

---

## 7. REFERENCE

### API Endpoints

**Ports:** `:8080` (API + health), `:9090` (Prometheus metrics)

**Infrastructure:** `GET /healthz`, `/readyz`, `/version`, `/metrics` (port 9090), `/api/openapi.yaml`, `/docs`

**Public (no auth):**
- `GET /api/v1/status`, `/status/history` — public status page
- `GET /api/v1/services?include_archived=bool`, `/services/{slug}` — services
- `GET /api/v1/services/{slug}/events?status=active|resolved&limit=N&offset=N` — service events (paginated)
- `GET /api/v1/groups?include_archived=bool`, `/groups/{slug}` — groups
- `GET /api/v1/events`, `/events/{id}`, `/events/{id}/updates`, `/events/{id}/changes` — events
- `GET /api/v1/notifications/config` — available channel types

**Authenticated:**
- `POST /api/v1/auth/register`, `/login`, `/refresh`, `/logout`; `GET /api/v1/me`
- `GET|POST /api/v1/me/channels`; `PATCH|DELETE /api/v1/me/channels/{id}`
- `POST /api/v1/me/channels/{id}/verify`, `/resend-code`
- `GET /api/v1/me/subscriptions`; `PUT /api/v1/me/channels/{id}/subscriptions`

**Operator+:**
- `POST /api/v1/events` — create (accepts `affected_services` + `affected_groups` with explicit statuses)
- `POST /api/v1/events/{id}/updates` — status update + manage services (`service_updates`, `add_services`, `add_groups`, `remove_service_ids`)
- `GET /api/v1/services/{slug}/status-log?limit=N&offset=N`

**Admin:**
- `POST|PATCH|DELETE /api/v1/services/{slug}`, `POST /services/{slug}/restore`
- `GET|PUT /api/v1/services/{slug}/tags`
- `POST|PATCH|DELETE /api/v1/groups/{slug}`, `POST /groups/{slug}/restore`
- `POST|GET /api/v1/templates`, `GET|DELETE /api/v1/templates/{slug}`, `POST /templates/{slug}/preview`
- `DELETE /api/v1/events/{id}` — only resolved/completed (409 for active)

### Response Contract

```json
{ "data": { ... } }                                    // Success
{ "error": { "message": "...", "details": "..." } }    // Error
```

### Key Business Rules

**Effective Status:**
- Operator specifies status per service/group: `{service_id, status}`, `{group_id, status}`
- Explicit service status overrides group-derived (priority: service > group)
- Effective = worst-case from ACTIVE events. Priority: `major_outage` > `partial_outage` > `degraded` > `maintenance` > `operational`
- Scheduled maintenance does NOT affect effective status until `in_progress`
- Computed via `v_service_effective_status` view; no active events → stored status

**Event Resolution:**
- On resolved/completed: services with no other active events → stored status set to `operational`
- Services with other active events → unchanged (effective status from remaining events)
- Manual status changes during active events are overwritten by this behavior

**Event Composition (via POST /events/{id}/updates):**
- All service management through updates endpoint
- Changes recorded in `event_service_changes` with `batch_id`, `reason`, `created_by`
- Cannot update resolved events (409). Validates all IDs exist before transaction
- Non-existent IDs → 400: "affected service/group not found: \<id\>". Archived = non-existent

**Event Deletion (admin only):**
- Only resolved/completed (409 for active). CASCADE: event_services, groups, updates, changes
- Status log entries referencing event deleted. Service statuses NOT changed

**Service Status Audit Log:**
- Every status change recorded in `service_status_log` (manual/event/webhook source)
- `GET /services/{slug}/status-log` (operator+), paginated

**Default Email Channel:**
- Auto-created on registration (verified, `is_default=true`). Cannot be deleted (409)
- Skipped if `NOTIFICATIONS_EMAIL_ENABLED=false`. Duplicate email per user → 409

**Channel Types:**
- Disabled types rejected with 400 (`ErrChannelTypeDisabled`). Mattermost always available
- Verification failures → 422 with user-friendly message (telegram: /start needed or bot blocked; mattermost: check webhook URL)

### Enums

```
roles:            user, operator, admin
channel_types:    email, telegram, mattermost
service_status:   operational, degraded, partial_outage, major_outage, maintenance
event_type:       incident, maintenance
event_status:     investigating, identified, monitoring, resolved (incident)
                  scheduled, in_progress, completed (maintenance)
severity:         minor, major, critical
change_action:    added, removed
status_log_source: manual, event, webhook
message_type:     initial, update, resolved, completed, cancelled
queue_status:     pending, processing, sent, failed
```

### Test Users

```
admin@example.com / admin123 / admin  |  operator@example.com / admin123 / operator  |  user@example.com / user123 / user
```

### Commands

```bash
make docker-up / dev                    # Start PostgreSQL / Run app
make lint / test / test-integration     # Quality
make migrate-up / migrate-down / migrate-create NAME=xxx  # DB
make build / docker-build               # Build
```

---

## 8. STATUS & TODO

### Done

All core modules implemented: identity (auth/RBAC), catalog (services/groups, M:N, soft delete, effective status, tags, status log), events (incidents/maintenance lifecycle, composition editing, audit trail, templates), notifications (email/telegram/mattermost senders, verification, subscriptions, event integration, async queue with retry). Cloud-native: Prometheus metrics, structured logging, graceful shutdown, deployment guide.

### Known Limitations

- No Helm chart (see `docs/deployment.md` for K8s examples)
- No pagination (except service events and status log)
- No bulk operations, email batching, telegram rate limiting
- No graceful degradation for senders, no rate limiting, no transient DB error retry

### Configuration

See [docs/deployment.md](./docs/deployment.md) for environment variables, K8s config, Prometheus setup.
