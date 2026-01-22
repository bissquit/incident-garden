# CLAUDE.md — StatusPage Service

## 🎯 Project Goal

An open-source self-hosted status page service for displaying service states and managing incidents. An alternative to Atlassian Statuspage, Cachet, Instatus — but simple, lightweight, and cloud-native.

**Key Features:**
- Service status display (operational, degraded, partial_outage, major_outage, maintenance)
- Event management (incidents + scheduled maintenance) with timeline updates
- Event templates with Go template support
- Scheduled maintenance
- RBAC: user → operator → admin
- Notification subscriptions (Email, Telegram) with flexible channel configuration
- REST API first (web interface is a separate project)

---

## 📊 Current Project Status

**Last update:** 2026-01-21

### What's Implemented

| Component                | Status       | Description                                                   |
|--------------------------|--------------|---------------------------------------------------------------|
| **Infrastructure**       | ✅ Done       | Docker Compose, Makefile, configuration                       |
| **Database**             | ✅ Done       | 5 migrations, complete schema                                 |
| **Identity module**      | ✅ Done       | JWT auth, register/login/refresh/logout, RBAC                 |
| **Catalog module**       | ✅ Done       | Services, Groups, Tags CRUD                                   |
| **Events module**        | ✅ Done       | Events, Updates, Templates, public status                     |
| **Notifications module** | ✅ Structure  | Handler, Service, Repository, Dispatcher (senders are stubs)  |
| **CI/CD**                | ✅ Done       | GitHub Actions: lint, test, integration-test, build           |
| **Integration tests**    | ✅ Done       | 20 tests, testcontainers                                      |

### File Structure

```
incident-management/
├── cmd/statuspage/main.go           # Entry point
├── internal/
│   ├── app/app.go                   # DI, routing, lifecycle
│   ├── config/config.go             # Configuration (koanf)
│   ├── domain/                      # Business entities
│   │   ├── event.go
│   │   ├── notification.go
│   │   ├── service.go
│   │   ├── subscription.go
│   │   ├── template.go
│   │   └── user.go
│   ├── identity/                    # Auth module
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── authenticator.go
│   │   ├── jwt/authenticator.go
│   │   └── postgres/repository.go
│   ├── catalog/                     # Services, Groups, Tags
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── service_test.go
│   │   ├── repository.go
│   │   └── postgres/repository.go
│   ├── events/                      # Events, Updates, Templates
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── service_test.go
│   │   ├── repository.go
│   │   ├── template_renderer.go
│   │   ├── errors.go
│   │   └── postgres/repository.go
│   ├── notifications/               # Channels, Subscriptions, Dispatch
│   │   ├── handler.go
│   │   ├── service.go
│   │   ├── repository.go
│   │   ├── dispatcher.go
│   │   ├── sender.go
│   │   ├── errors.go
│   │   ├── email/sender.go
│   │   ├── telegram/sender.go
│   │   └── postgres/repository.go
│   ├── testutil/                    # Test utilities
│   │   ├── client.go
│   │   ├── container.go
│   │   └── fixtures.go
│   └── pkg/
│       ├── httputil/
│       │   ├── middleware.go
│       │   └── response.go
│       └── postgres/postgres.go
├── migrations/
│   ├── 000001_init.up.sql
│   ├── 000002_add_refresh_tokens.up.sql
│   ├── 000003_add_default_admin.up.sql
│   ├── 000004_add_default_user.up.sql
│   └── 000005_add_default_operator.up.sql
├── tests/integration/
│   ├── main_test.go
│   ├── auth_test.go
│   ├── catalog_test.go
│   ├── events_test.go
│   └── rbac_test.go
├── .github/workflows/
│   └── ci.yml                       # lint, test, integration-test, build
├── docker-compose.yml
├── Makefile
└── go.mod
```

### API Endpoints (implemented)

**Public (no authentication):**
- `GET /healthz`, `GET /readyz` — health checks
- `GET /api/v1/status` — current status
- `GET /api/v1/status/history` — event history
- `GET /api/v1/services`, `GET /api/v1/services/{slug}` — list/details of services
- `GET /api/v1/groups`, `GET /api/v1/groups/{slug}` — list/details of groups

**Auth (no role required):**
- `POST /api/v1/auth/register` — registration
- `POST /api/v1/auth/login` — login
- `POST /api/v1/auth/refresh` — token refresh
- `POST /api/v1/auth/logout` — logout
- `GET /api/v1/me` — current user
- `GET|POST|PATCH|DELETE /api/v1/me/channels` — notification channels
- `GET|POST|DELETE /api/v1/me/subscriptions` — subscriptions

**Operator+ (operator or admin role):**
- `POST /api/v1/events` — create event
- `GET /api/v1/events`, `GET /api/v1/events/{id}` — list/details of events
- `POST /api/v1/events/{id}/updates` — add update
- `GET /api/v1/events/{id}/updates` — list of updates

**Admin:**
- `DELETE /api/v1/events/{id}` — delete event
- `POST|GET|DELETE /api/v1/templates` — template management
- `POST /api/v1/templates/{slug}/preview` — template preview
- `POST|PATCH|DELETE /api/v1/services` — service management
- `POST|PATCH|DELETE /api/v1/groups` — group management

### API Response Format (contract)

```json
// Success
{
  "data": { ... }
}

// Error
{
  "error": {
    "message": "error description"
  }
}

// Validation Error
{
  "error": {
    "message": "validation error",
    "details": "field validation failed"
  }
}
```

### Test Users (created by migrations)

| Email                | Password  | Role     |
|----------------------|-----------|----------|
| admin@example.com    | admin123  | admin    |
| operator@example.com | admin123  | operator |
| user@example.com     | user123   | user     |

### Working Commands

```bash
# Run
make docker-up          # Start PostgreSQL
make dev                # Run application (hot-reload)

# Tests
make test               # All tests
make test-unit          # Unit tests
make test-integration   # Integration tests (testcontainers)
make lint               # Linters

# Migrations
make migrate-up         # Apply migrations
make migrate-down       # Rollback last migration
make migrate-create NAME=xxx  # Create new migration

# Build
make build              # Build binary
make docker-build       # Build Docker image
```

---

## 📖 Functional Requirements (User Stories)

### Services
- Contains a list of services for which statuses are generated
- Each service has:
    - Name, slug (unique identifier)
    - Status: `operational`, `degraded`, `partial_outage`, `major_outage`, `maintenance`
    - Description (optional)
    - Belongs to a service group (optional)
    - Sort order
    - **Tags (key-value)**: e.g., "owner: John Doe", "owner_email: john@mail.com"

### Service Groups
- A group contains:
    - Name, slug
    - Description
    - Sort order
    - List of included services (linked via service.group_id)

### Events — combines incidents and scheduled maintenance
- Each event has:
    - Title
    - **Type**: `incident` | `maintenance` (scheduled maintenance)
    - **Status** (depends on type):
        - For incident: `investigating` → `identified` → `monitoring` → `resolved`
        - For maintenance: `scheduled` → `in_progress` → `completed`
    - Severity: `minor`, `major`, `critical` (incidents only, required)
    - Description
    - **Timestamps**:
        - `created_at` — when the record was created
        - `started_at` — when it actually started (may be earlier than created_at)
        - `updated_at` — last update
        - `resolved_at` — completion time
        - `scheduled_start_at` — scheduled start (for maintenance)
        - `scheduled_end_at` — scheduled end (for maintenance)
    - **`notify_subscribers` flag** — whether to send notifications
    - **Template reference** (optional)
    - Link to services (many-to-many)

### Event Updates
- Messages (updates) can be added to each event
- Each update contains:
    - New event status
    - Message text
    - **`notify_subscribers` flag** — whether to send notification for this update
    - Author and creation time

### Event Templates
- Have:
    - **Unique slug** (human-readable: `planned-maintenance-aws`, `incident-database-outage`)
    - Type: `incident` | `maintenance`
    - Title template (title_template)
    - Body template (body_template)
- **Go template support with macros**:
    - `{{.ServiceName}}` — service name
    - `{{.ServiceGroupName}}` — group name
    - `{{.StartedAt}}` — start time
    - `{{.ResolvedAt}}` — completion time
    - `{{.ScheduledStart}}` — scheduled start
    - `{{.ScheduledEnd}}` — scheduled end
    - Extensible in the future

### Scheduled Maintenance
- These are events of type `maintenance` with status `scheduled`
- When creating, specify:
    - Name, description
    - Related services
    - `scheduled_start_at`, `scheduled_end_at`
- Completion: operator adds update with status `completed`
    - Time can be selected manually or use current time

### Users
- Fields:
    - Email (required, unique)
    - Password (hash)
    - First name, last name (optional)
    - Role: `user`, `operator`, `admin`
- By default, notifications are sent to email

### Notification Channels
- User can add channels:
    - Type: `email`, `telegram`
    - Target: email address or Telegram chat_id
    - `is_enabled` flag — whether to use it
    - `is_verified` flag — whether the channel is verified
- Individual channels or all channels can be enabled/disabled

### Subscriptions
- User subscribes to notifications
- Can subscribe to:
    - All services (subscription_services is empty)
    - Specific services (via subscription_services)

---

## 🗄 Database Schema (Reference)

```
┌─────────────────┐       ┌─────────────────────┐
│     users       │       │ notification_channels│
├─────────────────┤       ├─────────────────────┤
│ id (PK)         │──┐    │ id (PK)             │
│ email (unique)  │  │    │ user_id (FK)        │──┐
│ password_hash   │  │    │ type                │  │
│ first_name      │  │    │ target              │  │
│ last_name       │  │    │ is_enabled          │  │
│ role            │  │    │ is_verified         │  │
│ created_at      │  │    │ created_at          │  │
│ updated_at      │  │    └─────────────────────┘  │
└─────────────────┘  │                             │
         │           │    ┌─────────────────────┐  │
         │           └───>│   subscriptions     │<─┘
         │                ├─────────────────────┤
         │                │ id (PK)             │
         │                │ user_id (FK)        │
         │                │ created_at          │
         │                └─────────────────────┘
         │                         │
         │                         ▼
         │                ┌─────────────────────┐
         │                │subscription_services│
         │                ├─────────────────────┤
         │                │ subscription_id(FK) │
         │                │ service_id (FK)     │─────────────┐
         │                └─────────────────────┘             │
         │                                                    │
         │    ┌─────────────────┐      ┌──────────────────┐   │
         │    │ service_groups  │      │    services      │<──┘
         │    ├─────────────────┤      ├──────────────────┤
         │    │ id (PK)         │<─────│ id (PK)          │
         │    │ name            │      │ name             │
         │    │ slug (unique)   │      │ slug (unique)    │
         │    │ description     │      │ description      │
         │    │ order           │      │ status           │
         │    │ created_at      │      │ group_id (FK)    │
         │    │ updated_at      │      │ order            │
         │    └─────────────────┘      │ created_at       │
         │                             │ updated_at       │
         │                             └──────────────────┘
         │                                      │
         │                                      ▼
         │                             ┌──────────────────┐
         │                             │  service_tags    │
         │                             ├──────────────────┤
         │                             │ id (PK)          │
         │                             │ service_id (FK)  │
         │                             │ key              │
         │                             │ value            │
         │                             └──────────────────┘
         │
         │    ┌─────────────────────┐
         │    │  event_templates    │
         │    ├─────────────────────┤
         │    │ id (PK)             │
         │    │ slug (unique)       │
         │    │ type                │
         │    │ title_template      │
         │    │ body_template       │
         │    │ created_at          │
         │    │ updated_at          │
         │    └─────────────────────┘
         │              │
         │              ▼
         │    ┌─────────────────────┐      ┌──────────────────┐
         └───>│      events         │      │  event_services  │
              ├─────────────────────┤      ├──────────────────┤
              │ id (PK)             │<─────│ event_id (FK)    │
              │ title               │      │ service_id (FK)  │────> services
              │ type                │      └──────────────────┘
              │ status              │
              │ severity            │
              │ description         │
              │ started_at          │
              │ resolved_at         │
              │ scheduled_start_at  │
              │ scheduled_end_at    │
              │ notify_subscribers  │
              │ template_id (FK)    │────> event_templates
              │ created_by (FK)     │────> users
              │ created_at          │
              │ updated_at          │
              └─────────────────────┘
                        │
                        ▼
              ┌─────────────────────┐
              │   event_updates     │
              ├─────────────────────┤
              │ id (PK)             │
              │ event_id (FK)       │
              │ status              │
              │ message             │
              │ notify_subscribers  │
              │ created_by (FK)     │────> users
              │ created_at          │
              └─────────────────────┘

              ┌─────────────────────┐
              │   refresh_tokens    │
              ├─────────────────────┤
              │ id (PK)             │
              │ user_id (FK)        │────> users
              │ token (unique)      │
              │ expires_at          │
              │ created_at          │
              └─────────────────────┘
```

### Enums (CHECK constraints)
```sql
-- users.role
'user', 'operator', 'admin'

-- notification_channels.type
'email', 'telegram'

-- services.status
'operational', 'degraded', 'partial_outage', 'major_outage', 'maintenance'

-- events.type, event_templates.type
'incident', 'maintenance'

-- events.status (depends on type)
-- incident:    'investigating', 'identified', 'monitoring', 'resolved'
-- maintenance: 'scheduled', 'in_progress', 'completed'

-- events.severity (incidents only, required)
'minor', 'major', 'critical'
```

---

## 🏗 Architectural Principles

### Main Rules
1. **Simplicity > Flexibility** — don't add abstractions "just in case"
2. **10/20 Rule** — if a feature adds >20% complexity while providing <10% value → rethink or postpone
3. **Testability** — any component can be tested in isolation
4. **Cloud-native** — 12-factor app, stateless, configuration via ENV
5. **API-first** — contract is more important than implementation

### Architectural Style
- **Modular monolith** with clear bounded contexts separation
- Ready to split into microservices when necessary
- If splitting is needed → move services to separate repositories with OpenAPI contracts

### Bounded Contexts (modules)
```
┌───────────────────────────────────────────────────────────────┐
│                      StatusPage API                           │
├─────────────┬─────────────┬─────────────┬─────────────────────┤
│   Identity  │   Catalog   │   Events    │    Notifications    │
│   (auth,    │  (services, │  (events,   │   (channels,        │
│    rbac)    │   groups,   │  updates,   │    subscriptions,   │
│             │   tags)     │  templates) │    dispatch)        │
└─────────────┴─────────────┴─────────────┴─────────────────────┘
```

**Rule for splitting into microservices:** extract a module only if:
- It has a fundamentally different load pattern (notifications are asynchronous)
- Independent deployment is required
- Development team is scaling

---

## 🛠 Technology Stack

### Core
| Component   | Technology               | Rationale                           |
|-------------|--------------------------|-------------------------------------|
| Language    | Go 1.25                  | Performance, simple deployment      |
| HTTP Router | chi                      | Lightweight, idiomatic              |
| Validation  | go-playground/validator  | De-facto standard                   |
| Config      | koanf                    | 12-factor compatible                |
| Logging     | slog (stdlib)            | Standard library Go 1.21+           |

### Data
| Component  | Technology          | Rationale                        |
|------------|---------------------|----------------------------------|
| Database   | PostgreSQL 16       | Reliability, JSON support        |
| Migrations | golang-migrate      | Simplicity, CLI + library        |
| SQL        | pgx                 | High performance                 |

### Infrastructure
| Component       | Technology                  | Rationale                         |
|-----------------|-----------------------------|-----------------------------------|
| Containerization| Docker + multi-stage builds | Minimal image size                |
| Local dev       | Docker Compose              | Simple local development          |
| CI/CD           | GitHub Actions              | GitHub Flow integration           |
| Tests           | testcontainers-go           | Real database in tests            |

---

## 🧪 Testing Strategy

### Current Coverage
- **Unit tests:** catalog/service_test.go, events/service_test.go
- **Integration tests:** tests/integration/ (20 tests)
    - auth_test.go — registration, login, tokens
    - catalog_test.go — services and groups CRUD
    - events_test.go — incident and maintenance lifecycle
    - rbac_test.go — role and access verification

### Running Tests
```bash
make test               # All tests
make test-unit          # Unit tests
make test-integration   # Integration tests (with testcontainers)
```

### Test Pyramid
```
         /\
        /  \     E2E (5%) — full scenarios via API
       /────\
      /      \   Integration (25%) — service + real DB
     /────────\
    /          \ Unit (70%) — isolated functions
   /────────────\
```

---

## 📍 Development Roadmap

### Stage 0: Project Initialization ✅
**Goal:** ready project skeleton with development tools

**Tasks:**
- [x] `go mod init`
- [x] Directory structure
- [x] Makefile with commands
- [x] .golangci.yml
- [x] .gitignore
- [x] README.md

---

### Stage 1: Local Development Environment ✅
**Goal:** runnable application with database connection

**Tasks:**
- [x] docker-compose.yml (PostgreSQL 16)
- [x] internal/config — configuration loading
- [x] internal/pkg/postgres — database connection
- [x] cmd/statuspage/main.go — entry point
- [x] Health endpoints: GET /healthz, GET /readyz

---

### Stage 2: Domain and Migrations ✅
**Goal:** business entities and database structure defined

**Tasks:**
- [x] internal/domain — all domain structures
- [x] migrations/000001_init.up.sql — initial migration
- [x] migrations/000002-000005 — additional migrations
- [x] Makefile commands for migrations

---

### Stage 3: Catalog Module (Services, Groups, Tags) ✅
**Goal:** CRUD for services, groups, and tags

**Tasks:**
- [x] internal/catalog — handler, service, repository
- [x] CRUD for services with tags
- [x] CRUD for groups
- [x] Unit tests (service_test.go)

---

### Stage 4: Identity Module (Auth & RBAC) ✅
**Goal:** authentication and authorization

**Tasks:**
- [x] internal/identity — Authenticator interface
- [x] JWT implementation
- [x] Middleware for token verification
- [x] RBAC middleware (user, operator, admin)
- [x] Registration, login, refresh, logout

---

### Stage 5: Events Module (Events, Updates, Templates) ✅
**Goal:** event and template management

**Tasks:**
- [x] internal/events — handler, service, repository
- [x] Support for two types: incident, maintenance
- [x] Different statuses depending on type
- [x] CRUD for templates
- [x] Go template renderer with macros
- [x] Timeline updates for events
- [x] Public status endpoint (GET /api/v1/status)
- [x] Unit tests (service_test.go)

---

### Stage 6: Notifications Module (Channels, Subscriptions, Dispatch) ✅ (partial)
**Goal:** event notifications

**Tasks:**
- [x] internal/notifications — handler, service, repository, dispatcher
- [x] CRUD for user channels
- [x] CRUD for subscriptions
- [ ] Real Email sender implementation (SMTP)
- [ ] Real Telegram sender implementation
- [ ] Channel verification
- [ ] Dispatcher integration with events (call when notify_subscribers=true)

---

### Stage 7: CI/CD ✅
**Goal:** automated checks and build

**Tasks:**
- [x] .github/workflows/ci.yml — lint, test, integration-test, build
- [ ] .github/workflows/release.yml — Docker image build, push to registry
- [ ] Dockerfile (multi-stage)

---

### Stage 8: Helm Chart 🔜
**Goal:** Kubernetes deployment

**Tasks:**
- [ ] deployments/helm/statuspage/ — chart
- [ ] Configurable values (replicas, resources, ingress)
- [ ] Deployment README

---

### Stage 9 (future): OIDC/Keycloak Integration
**Goal:** SSO via external Identity Provider

**Tasks:**
- [ ] OIDC Authenticator implementation
- [ ] Configuration via ENV
- [ ] Role mapping from claims
- [ ] Keycloak setup documentation

---

## 🎯 Definition of Done

A feature is considered complete when:
- [x] Code is written and meets standards
- [x] Unit tests are written
- [x] Integration tests for critical paths
- [x] OpenAPI specification is updated
- [x] Linters pass without errors
- [x] CI passes

---

## 💬 How to Work with Claude

### When requesting a new feature:
1. Describe the business requirement
2. I'll propose a design and estimate complexity
3. Discuss trade-offs
4. Implement iteratively

### When discussing architecture:
1. I'll ask clarifying questions
2. Propose several options with pros/cons
3. Apply the "10/20 rule" to assess complexity

### When writing code:
1. First — interface/contract
2. Then — implementation
3. In parallel — tests
4. Finally — integration

### Flags for special modes:
- `[REVIEW]` — please review my code
- `[REFACTOR]` — need to refactor existing code
- `[DEBUG]` — help find an issue
- `[DESIGN]` — discuss architecture before code

---

## ⚠️ Known Limitations and TODO

### Notifications Module
- Email sender and Telegram sender are stubs, don't send actual messages
- No dispatcher integration with events (not called when creating event/update)
- No channel verification

### Missing
- OpenAPI specification (api/openapi/openapi.yaml)
- Dockerfile
- Helm chart
- Prometheus metrics
- Pagination in lists

### Technical Debt
- No graceful degradation when notification senders are unavailable
- No rate limiting
- No audit log

---

## 📝 Go Code Style & Linter Requirements

### golangci-lint Configuration
The project uses `.golangci.yml` with strict linting rules. **ALWAYS** follow these requirements when writing Go code:

#### 1. Package Comments (Required)
Every package MUST have a package-level comment:
```go
// Package version contains build version information.
package version

// Package catalog provides service and group management functionality.
package catalog
```

#### 2. Exported Symbols Comments (Required)
ALL exported types, functions, constants, and variables MUST have comments:
```go
// User represents a system user with authentication credentials.
type User struct {
    ID    int64
    Email string
}

// NewService creates a new catalog service instance.
func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}

// Version is the current application version.
// This value is updated automatically by Release Please.
var Version = "0.0.0"

// MaxRetries defines the maximum number of retry attempts.
const MaxRetries = 3
```

#### 3. Error Handling (Required)
- NEVER ignore errors
- Always check and wrap errors with context
```go
// ✅ Good
if err := db.Ping(ctx); err != nil {
    return fmt.Errorf("ping database: %w", err)
}

// ❌ Bad
_ = db.Ping(ctx)
```

#### 4. Context Usage
- Always pass `context.Context` as first parameter
- Use `context.Background()` only at top level
```go
// ✅ Good
func (s *Service) GetUser(ctx context.Context, id int64) (*User, error)

// ❌ Bad
func (s *Service) GetUser(id int64) (*User, error)
```

#### 5. Naming Conventions
- Use `camelCase` for unexported, `PascalCase` for exported
- Avoid stuttering: `user.UserService` → `user.Service`
- Use meaningful names: `ctx` (context), `err` (error), `i` (index only in loops)

#### 6. Common Linter Errors to Avoid

**revive: package-comments**
```go
// ❌ Missing package comment
package mypackage

// ✅ With package comment
// Package mypackage provides functionality for X.
package mypackage
```

**revive: exported**
```go
// ❌ Exported without comment
var Version = "0.0.0"

// ✅ Exported with comment
// Version is the current application version.
var Version = "0.0.0"
```

**errcheck: unchecked error**
```go
// ❌ Ignored error
rows.Close()

// ✅ Checked error
defer func() {
    if err := rows.Close(); err != nil {
        log.Error("close rows", "error", err)
    }
}()
```

**staticcheck: unused**
```go
// ❌ Unused variable
func foo() {
    x := 1
    return
}

// ✅ Remove or use it
func foo() {
    return
}
```

### Running Linters Locally
```bash
# Before committing ALWAYS run:
make lint

# Fix common issues automatically:
golangci-lint run --fix
```

### CI/CD Integration
- Linters run automatically on every PR
- **Zero tolerance**: PR cannot be merged with linter errors
- Fix all issues before pushing

---

## ⚠️ Anti-patterns (what NOT to do)

1. **Don't use ORM** (GORM and similar) — use pgx
2. **Don't create God-objects** — each service does one thing
3. **Don't ignore errors** — always check and wrap
4. **Don't hardcode configuration** — everything via ENV/config
5. **Don't write business logic in handlers** — handlers for I/O only
6. **Don't make circular dependencies** between modules
7. **Don't add features without tests** — test coverage for new code
8. **Don't skip linter checks** — always run `make lint` before committing
9. **Don't commit without package/export comments** — linters will fail in CI
