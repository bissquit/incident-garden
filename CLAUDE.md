# CLAUDE.md — IncidentGarden

> Open-source self-hosted status page service. Alternative to Atlassian Statuspage, Cachet, Instatus.

---

> **MANDATORY: Keep This File Updated**
>
> This file is the source of truth for AI assistants working on this codebase.
> **You MUST update CLAUDE.md when your changes affect:**
> - Database schema (new tables, columns, constraints, views)
> - API endpoints (new routes, changed request/response formats)
> - Domain types (new structs in `internal/domain/`)
> - Module interfaces (new methods in `repository.go`, `service.go`)
> - Business rules or architectural decisions
> - Project status (completed features, new limitations)
>
> **Before completing any task, verify:**
> 1. Does CODEMAP reflect current file structure and interfaces?
> 2. Does Database Schema match actual migrations?
> 3. Do API Endpoints match `api/openapi/openapi.yaml`?
> 4. Does STATUS & TODO reflect what was just implemented?
>
> Outdated documentation causes cascading errors in future AI-assisted work.

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

### Key Architectural Decisions

**M:N Services ↔ Groups:**
- Service can belong to multiple groups simultaneously
- Junction table: `service_group_members(service_id, group_id)`
- API uses `group_ids: []string` instead of `group_id: *string`

**Events with Groups:**
- Events can be created by selecting groups (auto-expands to services)
- `event_groups` stores which groups were selected
- `event_services` stores flattened list of affected services
- `event_service_changes` tracks all composition changes (audit trail)

**Soft Delete:**
- Services and groups use `archived_at` instead of hard delete
- Archived items hidden from lists by default (`include_archived=true` to show)
- Cannot archive service/group with active (non-resolved) events
- Archived items remain visible in historical events

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

### Database Schema (Key Tables)

```
services
├── id, name, slug, description, status, order
├── created_at, updated_at, archived_at (soft delete)
└── NO group_id column (M:N via junction)

service_groups
├── id, name, slug, description, order
├── created_at, updated_at, archived_at (soft delete)

service_group_members (M:N junction)
├── service_id FK → services
└── group_id FK → service_groups

events
├── id, title, type, status, severity, description
├── started_at, resolved_at, scheduled_start_at, scheduled_end_at
├── notify_subscribers, template_id, created_by
└── created_at, updated_at

event_services (M:N junction — services with status)
├── event_id FK → events
├── service_id FK → services
├── status VARCHAR(50) NOT NULL — service status in event context
└── updated_at TIMESTAMP NOT NULL

event_groups (M:N junction — selected groups)
├── event_id FK → events
└── group_id FK → service_groups

event_service_changes (audit trail)
├── id, event_id, batch_id (nullable, groups operations)
├── action ('added'|'removed')
├── service_id (nullable), group_id (nullable)
├── reason, created_by, created_at

v_service_effective_status (VIEW — computed effective status)
├── service_id, status (stored), effective_status (computed)
├── has_active_events BOOLEAN
└── Uses worst-case priority: major_outage > partial_outage > degraded > maintenance > operational
```

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
├── handler.go             → CRUD /services, /groups + /restore endpoints
├── service.go             → CreateService, UpdateService, DeleteService (soft), RestoreService
├── service_test.go        → Unit tests
├── repository.go          → Interface with M:N methods + soft delete
└── postgres/repository.go → SQL with archived_at filtering

Key interfaces:
- SetServiceGroups(ctx, serviceID, groupIDs []string)
- GetServiceGroups(ctx, serviceID) → []string
- GetGroupServices(ctx, groupID) → []string  // Used by events module
- SetGroupServices(ctx, groupID, serviceIDs []string)
- ArchiveService/RestoreService
- GetActiveEventCountForService(ctx, serviceID) → int
- GetNonArchivedServiceCountForGroup(ctx, groupID) → int
- GetEffectiveStatus(ctx, serviceID) → (ServiceStatus, hasActiveEvents, error)
- GetServiceBySlugWithEffectiveStatus(ctx, slug) → *ServiceWithEffectiveStatus
- ListServicesWithEffectiveStatus(ctx, filter) → []ServiceWithEffectiveStatus
- SetServiceTags(ctx, serviceID, tags) / GetServiceTags(ctx, serviceID) → []ServiceTag

Dependencies: domain.Service, domain.ServiceGroup, domain.ServiceWithEffectiveStatus, pkg/postgres
```

### Module: events

```
internal/events/
├── handler.go             → CRUD /events + /services, /changes endpoints
├── service.go             → CreateEvent (with group expansion), AddServicesToEvent, RemoveServicesFromEvent
├── service_test.go        → Unit tests
├── repository.go          → Interface with groups + audit methods
├── resolver.go            → Interface: GroupServiceResolver (implemented by catalog.Service)
├── template_renderer.go   → Go template execution
├── errors.go              → ErrEventNotFound, ErrInvalidTransition...
└── postgres/repository.go → SQL for events, groups, changes

Key interfaces:
- AssociateGroups(ctx, eventID, groupIDs)
- AddGroups(ctx, eventID, groupIDs)
- GetEventGroups(ctx, eventID) → []string
- GetEventServices(ctx, eventID) → []EventService
- AssociateServiceWithStatusTx(ctx, tx, eventID, serviceID, status)
- UpdateEventServiceStatusTx(ctx, tx, eventID, serviceID, status)
- CreateServiceChange(ctx, change)
- ListServiceChanges(ctx, eventID) → []EventServiceChange

Dependencies: domain.Event, domain.EventService, domain.AffectedService, domain.AffectedGroup, catalog.Service (as GroupServiceResolver), pkg/postgres
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
internal/domain/           → User, Service, ServiceGroup, Event, EventUpdate, EventServiceChange,
                             Template, Channel, Subscription,
                             ServiceWithEffectiveStatus, ServiceTag, EventService,
                             AffectedService, AffectedGroup (API input types)
internal/pkg/httputil/     → response.go (Success/Error), middleware.go
internal/pkg/postgres/     → Connect(cfg) → *pgxpool.Pool
internal/testutil/         → HTTP test client, testcontainers setup, fixtures, OpenAPI validator
internal/version/          → Build version info (injected at compile time)
```

### Dependency Flow

```
main.go → app.NewApp(cfg)
            ├── postgres.Connect()
            ├── identity:     Repository → Service → Handler + Middleware
            ├── catalog:      Repository → Service → Handler
            │                              ↓
            ├── events:       Repository → Service (resolver=catalogService) → Handler
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
8. **Update CLAUDE.md** if you changed schema, API, domain types, or interfaces

### OpenAPI Versioning

OpenAPI version (`info.version` in `api/openapi/openapi.yaml`) is **independent** from application version. Update it only when API contract changes.

**When to bump:**

| Change Type       | Example                                             | Version Bump              |
|-------------------|-----------------------------------------------------|---------------------------|
| Breaking change   | Remove endpoint, remove field, change response code | **MAJOR** (1.x.0 → 2.0.0) |
| New feature       | Add endpoint, add optional field, add enum value    | **MINOR** (1.3.0 → 1.4.0) |
| Fix/clarification | Fix description, fix example, no behavior change    | **PATCH** (1.3.0 → 1.3.1) |

**When NOT to bump:**
- Infrastructure changes (CORS, rate limiting, internal refactoring)
- Changes that don't affect request/response schemas
- Bug fixes in implementation (not contract)

**Checklist for API changes:**
1. Update schema in `api/openapi/openapi.yaml`
2. Bump `info.version` according to rules above
3. Mention in PR description: "OpenAPI: 1.3.0 → 1.4.0"

### Definition of Done (PR Checklist)

- [ ] Layer boundaries: handler has no business logic; service has no SQL
- [ ] Errors: no ignored errors; all wrapped with context
- [ ] Contract: OpenAPI updated if API changed; migrations if schema changed
- [ ] **OpenAPI version bumped** if contract changed (see OpenAPI Versioning)
- [ ] **CLAUDE.md updated** if changes affect: schema, API, domain types, interfaces, or business rules
- [ ] Tests: according to Test Matrix
- [ ] `make lint` passes
- [ ] `make test` / `make test-integration` passes
- [ ] `make build` passes

### Claude Interaction Modes

**`[DESIGN]`** — Before coding, discuss architecture
**`[REFACTOR]`** — Restructure existing code
**`[DEBUG]`** — Investigate issues
**`[REVIEW]`** — Code review

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

### Cross-Module Dependencies (Allowed)

```
events.Service depends on catalog.Service (via GroupServiceResolver interface)
```

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

**Error handling:**
```go
if err := db.Ping(ctx); err != nil {
return fmt.Errorf("ping database: %w", err)
}
```

**Empty slices for JSON:**
```go
items := make([]Item, 0)  // → [] not null
```

**Soft delete pattern:**
```go
// Repository
func (r *Repository) ArchiveService(ctx context.Context, id string) error {
query := `UPDATE services SET archived_at = NOW() WHERE id = $1 AND archived_at IS NULL`
// ...
}

// Service layer — check business rules before archive
func (s *Service) DeleteService(ctx context.Context, id string) error {
activeCount, _ := s.repo.GetActiveEventCountForService(ctx, id)
if activeCount > 0 {
return ErrServiceHasActiveEvents
}
return s.repo.ArchiveService(ctx, id)
}
```

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
| Soft delete logic        | —          | ✅ Required     |
| M:N relationships        | —          | ✅ Required     |

### Commands

```bash
make test               # All
make test-unit          # Unit only
make test-integration   # Integration (testcontainers)
```

### Test environment

```shell
docker compose -f deployments/docker/docker-compose-postgres.yml up -d
# ... work ...
docker compose -f deployments/docker/docker-compose-postgres.yml down
docker volume rm docker_postgres_data
```

---

## 7. REFERENCE

### API Endpoints

**Public:**
- `GET /healthz`, `/readyz` — health checks
- `GET /api/v1/status`, `/status/history` — public status
- `GET /api/v1/services?include_archived=bool`, `/services/{slug}` — services
- `GET /api/v1/groups?include_archived=bool`, `/groups/{slug}` — groups
- `GET /api/v1/events`, `/events/{id}` — events (read-only)
- `GET /api/v1/events/{id}/updates` — event updates (read-only)
- `GET /api/v1/events/{id}/changes` — event service changes (read-only)

**Auth (any authenticated):**
- `POST /api/v1/auth/register`, `/login`, `/refresh`, `/logout`
- `GET /api/v1/me`
- `GET|POST|PATCH|DELETE /api/v1/me/channels`
- `GET|POST|DELETE /api/v1/me/subscriptions`

**Operator+:**
- `POST /api/v1/events` — create (accepts `affected_services` + `affected_groups` with explicit statuses)
- `POST /api/v1/events/{id}/updates` — add status updates
- `POST /api/v1/events/{id}/services` — add services/groups to event
- `DELETE /api/v1/events/{id}/services` — remove services from event

**Admin:**
- `DELETE /api/v1/events/{id}`
- `POST|GET|DELETE /api/v1/templates`
- `POST /api/v1/templates/{slug}/preview`
- `POST|PATCH|DELETE /api/v1/services`, `/groups` — soft delete on DELETE
- `POST /api/v1/services/{slug}/restore`, `/groups/{slug}/restore`

### API Response Contract

```json
{ "data": { ... } }                                    // Success
{ "error": { "message": "..." } }                      // Error
{ "error": { "message": "...", "details": "..." } }    // Validation
```

### Key Business Rules

**Soft Delete:**
- DELETE returns 409 if service/group has active events (status not resolved/completed)
- Archived items excluded from listings unless `include_archived=true`
- Archived items remain in historical event data

**Event Creation with Groups:**
- `group_ids` in request → system resolves to `service_ids` at creation time
- Both `group_ids` and expanded `service_ids` stored
- Duplicate services (in multiple groups or explicit) deduplicated

**Event Composition Changes:**
- Adding services/groups records to `event_service_changes`
- Removing services records to `event_service_changes`
- Full audit trail with `reason`, `created_by`, `created_at`

**Effective Status (Service Status in Events):**
- When creating events, operator specifies status for each service/group via `affected_services` and `affected_groups`
- Each `affected_service` contains `{service_id, status}`, each `affected_group` contains `{group_id, status}`
- Explicit service status overrides group-derived status (priority: service > group)
- Service's effective status = worst active event status (priority: `major_outage` > `partial_outage` > `degraded` > `maintenance` > `operational`)
- Computed via `v_service_effective_status` view, accessible via `GetEffectiveStatus()` and `ListServicesWithEffectiveStatus()`
- If no active events, effective status = stored service status

### Enums

```
roles:           user, operator, admin
channel_types:   email, telegram
service_status:  operational, degraded, partial_outage, major_outage, maintenance
event_type:      incident, maintenance
event_status:    investigating, identified, monitoring, resolved (incident)
                 scheduled, in_progress, completed (maintenance)
severity:        minor, major, critical
change_action:   added, removed
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

✅ **Done:**
- Infrastructure, Database, Identity, Catalog, Events, CI/CD
- M:N Services ↔ Groups relationship
- Events with group selection (auto-expand to services)
- Event composition editing with audit trail
- Soft delete for services and groups
- Service status tracking within events (affected_services/affected_groups with explicit statuses)
- Effective status computation (v_service_effective_status view)
- Integration tests (25+)

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
- Bulk operations

**Tech Debt:**
- No graceful degradation for senders
- No rate limiting

### Next Up

- [ ] Real Email sender (SMTP)
- [ ] Real Telegram sender
- [ ] Dispatcher ↔ Events integration
- [ ] Channel verification flow
- [ ] Notifications on event composition changes
