# CLAUDE.md — IncidentGarden

> Open-source self-hosted status page service. Alternative to Atlassian Statuspage, Cachet, Instatus.

---

## 1. PROJECT CONTEXT

### What This Is
- **Core:** Service status display + incident/maintenance management + notifications
- **Architecture:** Modular monolith (Go), REST API-first
- **Modules:** `identity` (auth/RBAC) → `catalog` (services/groups) → `events` (incidents/maintenance) → `notifications` (channels/dispatch)

### Tech Stack
- **Go 1.25**, chi (router), pgx (PostgreSQL 16), koanf (config), slog (logging)
- **Infra:** Docker, testcontainers-go, GitHub Actions
- **Validation:** go-playground/validator
- **Migrations:** golang-migrate

### Domain Concepts

**Service statuses:** `operational`, `degraded`, `partial_outage`, `major_outage`, `maintenance`

**Event types and transitions:**
```
incident:    investigating → identified → monitoring → resolved
maintenance: scheduled → in_progress → completed
```

**Severity (incidents only):** `minor`, `major`, `critical`

**Roles:** `user` → `operator` → `admin`

**Template macros:** `{{.ServiceName}}`, `{{.ServiceGroupName}}`, `{{.StartedAt}}`, `{{.ResolvedAt}}`, `{{.ScheduledStart}}`, `{{.ScheduledEnd}}`

---

## 2. CODEMAP

### Quick Navigation

| I need to...                 | Go to                                      |
|------------------------------|--------------------------------------------|
| Add/modify API endpoint      | `internal/<module>/handler.go`             |
| Add business rule/validation | `internal/<module>/service.go`             |
| Change database query        | `internal/<module>/postgres/repository.go` |
| Add new entity               | `internal/domain/<entity>.go`              |
| Add database migration       | `migrations/NNNNNN_name.up.sql`            |
| Add shared utility           | `internal/pkg/<package>/`                  |
| Add integration test         | `tests/integration/<module>_test.go`       |
| Change app wiring/DI         | `internal/app/app.go`                      |
| Modify configuration         | `internal/config/config.go`                |
| Update API contract          | `api/openapi/openapi.yaml`                 |

### Module: identity

```
internal/identity/
├── handler.go           → POST /auth/register, /login, /refresh, /logout; GET /me
├── service.go           → CreateUser, Authenticate, RefreshTokens
├── repository.go        → Interface: UserRepository, TokenRepository
├── authenticator.go     → Interface: Authenticator
├── jwt/authenticator.go → JWT implementation
└── postgres/repository.go

Middleware: RequireAuth(next), RequireRole(roles...)
Dependencies: domain.User, pkg/postgres, pkg/httputil
```

### Module: catalog

```
internal/catalog/
├── handler.go             → CRUD /services, /groups (public GET, admin POST/PATCH/DELETE)
├── service.go             → CreateService, UpdateService, ListServices, CreateGroup...
├── service_test.go        → Unit tests
├── repository.go          → Interface: ServiceRepository, GroupRepository
└── postgres/repository.go

Dependencies: domain.Service, domain.Group, pkg/postgres
```

### Module: events

```
internal/events/
├── handler.go             → CRUD /events, /updates, /templates; GET /status
├── service.go             → CreateEvent, AddUpdate, GetPublicStatus, RenderTemplate
├── service_test.go        → Unit tests
├── repository.go          → Interface: EventRepository, TemplateRepository
├── template_renderer.go   → Go template execution
├── errors.go              → ErrEventNotFound, ErrInvalidTransition...
└── postgres/repository.go

Dependencies: domain.Event, domain.Template, catalog.Service (read-only), pkg/postgres
```

### Module: notifications

```
internal/notifications/
├── handler.go             → CRUD /me/channels, /me/subscriptions
├── service.go             → CreateChannel, Subscribe, GetSubscribersForServices
├── repository.go          → Interface: ChannelRepository, SubscriptionRepository
├── dispatcher.go          → Dispatch(ctx, notification)
├── sender.go              → Interface: Sender
├── email/sender.go        → Email sender (STUB)
├── telegram/sender.go     → Telegram sender (STUB)
└── postgres/repository.go

⚠️ Senders are stubs, dispatcher not integrated with events yet
```

### Shared

```
internal/domain/           → User, Service, Group, Event, Template, Channel, Subscription (pure, no deps)
internal/pkg/httputil/     → response.go (Success/Error), middleware.go
internal/pkg/postgres/     → Connect(cfg) → *pgxpool.Pool
```

### Dependency Flow

```
main.go → app.NewApp(cfg)
            ├── postgres.Connect()
            ├── identity:     Repository → Service → Handler + Middleware
            ├── catalog:      Repository → Service → Handler
            ├── events:       Repository → Service (+ TemplateRenderer) → Handler
            └── notifications: Repository → Service → Dispatcher → Handler
                                                        ├── email.Sender
                                                        └── telegram.Sender
            All Handlers → chi.Router → HTTP Server
```

---

## 3. WORKFLOW

### Algorithm for Any Task

1. **Clarify:** module, endpoint/schema change, roles, backward compatibility
2. **Contract first:** OpenAPI (`api/openapi/openapi.yaml`) or migration before code
3. **Boundaries:** what goes to handler/service/repository/domain/pkg
4. **Top-down:** handler → service → repository → migrations
5. **Errors:** wrap with context (`fmt.Errorf("...: %w", err)`)
6. **Tests:** unit for logic, integration for DB paths
7. **Validate:** `make lint && make test && make build`

### Definition of Done (PR Checklist)

- [ ] Layer boundaries: handler has no business logic; service has no SQL
- [ ] Errors: no ignored errors; all wrapped with context
- [ ] Contract: OpenAPI updated if API changed; migrations if schema changed
- [ ] Tests: according to Test Matrix
- [ ] `make lint` passes
- [ ] `make test` / `make test-integration` passes
- [ ] `make build` passes

### Claude Interaction Modes

**`[DESIGN]`** — Before coding, discuss architecture
```
[DESIGN] Feature X in module Y.
- Requirement: ...
- Affected endpoints: ...
- Constraints: ...
```

**`[REFACTOR]`** — Restructure existing code
```
[REFACTOR] Target: reduce coupling in X.
- Current problem: ...
- Target state: ...
```

**`[DEBUG]`** — Investigate issues
```
[DEBUG] Error X when doing Y.
- Steps: ...
- Expected/Actual: ...
- Logs: ...
```

**`[REVIEW]`** — Code review
```
[REVIEW] Check for: boundaries, errors, tests, OpenAPI, lints.
- Diff/Link: ...
```

---

## 4. ARCHITECTURE

### Layer Responsibilities

| Layer      | File            | Does                                         | Does NOT            |
|------------|-----------------|----------------------------------------------|---------------------|
| Handler    | `handler.go`    | HTTP I/O, auth check, validation, error→HTTP | Business logic, SQL |
| Service    | `service.go`    | Use-cases, business rules, orchestration     | SQL, HTTP concerns  |
| Repository | `repository.go` | Interface definition                         | Implementation      |
| Repo Impl  | `postgres/*.go` | SQL/pgx data access                          | Business decisions  |
| Domain     | `domain/*.go`   | Entities, domain errors                      | Infrastructure      |
| Pkg        | `pkg/*`         | Shared infra utilities                       | Business logic      |

### Principles

1. **Simplicity > Flexibility** — no abstractions "just in case"
2. **10/20 Rule** — >20% complexity for <10% value → postpone
3. **API-first** — contract before implementation
4. **No circular deps** between modules

### Patterns (DO)

- Thin handlers (HTTP only)
- Use-case services (all logic in service.go)
- Repository interfaces in module, impl in `postgres/`
- Domain errors in `errors.go` + `errors.Is/As` mapping
- Response helpers from `pkg/httputil/response.go`

### Anti-patterns (DON'T)

- ORM (GORM) → use pgx
- God-objects → single responsibility
- Ignored errors → always check and wrap
- Hardcoded config → ENV/koanf
- Business logic in handlers
- Circular module dependencies
- Features without tests
- Skipping linters

---

## 5. CODE STYLE

### Must Have

**Package comments:**
```go
// Package catalog provides service and group management.
package catalog
```

**Exported symbol comments:**
```go
// Service implements catalog use-cases.
type Service struct { ... }

// NewService creates a new catalog service instance.
func NewService(repo Repository) *Service { ... }
```

**Error handling:**
```go
// Always wrap with context
if err := db.Ping(ctx); err != nil {
    return fmt.Errorf("ping database: %w", err)
}

// Deferred close with error check
defer func() {
    if err := rows.Close(); err != nil {
        log.Error("close rows", "error", err)
    }
}()
```

**Context first:**
```go
func (s *Service) GetUser(ctx context.Context, id int64) (*User, error)
```

**Empty slices for JSON:**
```go
items := make([]Item, 0)  // → [] not null
```

### Naming

- Exported: `PascalCase`, unexported: `camelCase`
- No stuttering: `user.Service` not `user.UserService`
- Standard: `ctx`, `err`, `i` (loop index only)

### Linters

```bash
make lint                    # Run before every commit
golangci-lint run --fix      # Auto-fix some issues
```

Zero tolerance — PR cannot merge with linter errors.

---

## 6. TESTING

### Strategy

```
Unit (70%)        — pure logic, mocked deps
Integration (25%) — service + real Postgres (testcontainers)
E2E (5%)          — full API scenarios
```

### Test Matrix

| Change                   | Unit       | Integration    |
|--------------------------|------------|----------------|
| Repository SQL           | —          | ✅ Required     |
| Service business rules   | ✅ Required | If DB involved |
| Handler/validation/roles | —          | ✅ Required     |
| OpenAPI changes          | —          | ✅ At least 1   |
| Migrations               | —          | ✅ Required     |
| Domain pure functions    | ✅ Required | —              |

### Commands

```bash
make test               # All
make test-unit          # Unit only
make test-integration   # Integration (testcontainers)
```

### Test Files

- Unit: `internal/<module>/service_test.go`
- Integration: `tests/integration/<module>_test.go`
- Utilities: `internal/testutil/` (client.go, container.go, fixtures.go)

### Test environment

Prepare test environment for each task. Start database instance:
```shell
docker compose -f deployments/docker/docker-compose-postgres.yml up -d
```

Don't forget to clean environment after task is done:
```shell
docker compose -f deployments/docker/docker-compose-postgres.yml down ;\
docker volume rm docker_postgres_data
```

---

## 7. REFERENCE

### API Endpoints

**Public:**
- `GET /healthz`, `/readyz` — health checks
- `GET /api/v1/status`, `/status/history` — public status
- `GET /api/v1/services`, `/services/{slug}` — services
- `GET /api/v1/groups`, `/groups/{slug}` — groups

**Auth (any authenticated):**
- `POST /api/v1/auth/register`, `/login`, `/refresh`, `/logout`
- `GET /api/v1/me`
- `GET|POST|PATCH|DELETE /api/v1/me/channels`
- `GET|POST|DELETE /api/v1/me/subscriptions`

**Operator+:**
- `POST /api/v1/events` — create
- `GET /api/v1/events`, `/events/{id}` — list/get
- `POST|GET /api/v1/events/{id}/updates` — updates

**Admin:**
- `DELETE /api/v1/events/{id}`
- `POST|GET|DELETE /api/v1/templates`
- `POST /api/v1/templates/{slug}/preview`
- `POST|PATCH|DELETE /api/v1/services`, `/groups`

### API Response Contract

```json
{ "data": { ... } }                                    // Success
{ "error": { "message": "..." } }                      // Error
{ "error": { "message": "...", "details": "..." } }    // Validation
```

### Enums

```
roles:           user, operator, admin
channel_types:   email, telegram
service_status:  operational, degraded, partial_outage, major_outage, maintenance
event_type:      incident, maintenance
event_status:    investigating, identified, monitoring, resolved (incident)
                 scheduled, in_progress, completed (maintenance)
severity:        minor, major, critical
```

### Test Users (from migrations)

```
admin@example.com    / admin123  / admin
operator@example.com / admin123  / operator
user@example.com     / user123   / user
```

### Commands

```bash
# Dev
make docker-up          # Start PostgreSQL
make dev                # Run app (hot-reload)

# Quality
make lint               # Linters
make test               # All tests
make test-integration   # Integration only

# DB
make migrate-up
make migrate-down
make migrate-create NAME=xxx

# Build
make build
make docker-build
```

---

## 8. STATUS & TODO

### Current State

✅ **Done:** Infrastructure, Database, Identity, Catalog, Events, CI/CD, Integration tests (20)
⚠️ **Partial:** Notifications (structure ready, senders are stubs)

### Known Limitations

**Notifications:**
- Email/Telegram senders are stubs
- Dispatcher not called when creating events
- No channel verification

**Missing:**
- Helm chart
- Prometheus metrics
- Pagination

**Tech Debt:**
- No graceful degradation for senders
- No rate limiting
- No audit log

### Next Up

- [ ] Real Email sender (SMTP)
- [ ] Real Telegram sender
- [ ] Dispatcher ↔ Events integration
- [ ] Channel verification flow
