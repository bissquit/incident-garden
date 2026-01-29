# :deciduous_tree: IncidentGarden

Hi, dude! You're reading the only paragraph written by a human. The rest is created by AI, even a name of the project.

## About the Project

IncidentGarden is a simple and lightweight cloud-native service for managing status pages and incidents. An alternative to Atlassian Statuspage, Cachet, and Instatus, but with a focus on simplicity and self-hosting.

### Key Features

- 📊 Service status display (operational, degraded, partial_outage, major_outage, maintenance)
- 🚨 Incident management with timeline updates
- 👥 RBAC: user → operator → admin
- 🔔 Notification subscriptions (Email, Telegram)
- 🔌 REST API first (web interface is a separate project)

## Quick Start

### Requirements

- Go 1.22+
- Docker & Docker Compose
- Make

### Installation

```bash
git clone https://github.com/bissquit/incident-garden.git
cd incident-garden
```

### Local Development

```bash
# Show available commands
make help

# Option 1: Full stack with Docker (PostgreSQL + migrations + app)
make docker-build    # Build image
make docker-up       # Start stack

# Option 2: Only PostgreSQL with Docker, app locally
make docker-postgres # Start only PostgreSQL
make dev             # Run app with hot-reload

# View logs
make docker-logs
```

## Project Structure

```
├── cmd/statuspage/          # Application entry point
├── internal/                # Internal code
│   ├── app/                 # Application initialization
│   ├── config/              # Configuration
│   ├── domain/              # Domain entities
│   ├── identity/            # Authentication and RBAC
│   ├── catalog/             # Service management
│   ├── incidents/           # Incident management
│   ├── notifications/       # Notifications
│   └── pkg/                 # Common utilities
├── api/openapi/             # OpenAPI specification
├── migrations/              # Database migrations
└── deployments/             # Docker and Helm charts
```

## Development

### Make Commands

```bash
make test           # Run all tests
make test-unit      # Unit tests only
make test-int       # Integration tests only
make lint           # Run linters
make build          # Build binary
```

### Migrations

```bash
make migrate-up                       # Apply migrations
make migrate-down                     # Rollback migration
make migrate-create NAME=add_users    # Create new migration
```

## 🐳 Docker

### Quick Start

1. Copy `.env.example` to `.env` and configure:
```bash
cp .env.example .env
# Edit .env - IMPORTANT: change JWT_SECRET_KEY and POSTGRES_PASSWORD
# Set POSTGRES_PORT if needed (default: 5432)
```

2. Build and start:
```bash
make docker-build    # Build image
make docker-up       # Start full stack (PostgreSQL + migrations + app)
```

3. Verify:
```bash
# Health check
curl http://localhost:8080/healthz

# API
curl http://localhost:8080/api/v1/status

# View logs
make docker-logs
```

### Docker Commands

```bash
# Full stack
make docker-build    # Build Docker image
make docker-up       # Start full stack (PostgreSQL + migrations + app)
make docker-down     # Stop full stack
make docker-logs     # Show logs
make docker-ps       # Show container status
make docker-restart  # Restart application

# PostgreSQL only (for local development)
make docker-postgres # Start only PostgreSQL on POSTGRES_PORT (default: 5432)

# Registry
make docker-push     # Push image to GitHub Container Registry
```

### Using Pre-built Image

Pull from GitHub Container Registry:
```bash
docker pull ghcr.io/bissquit/incident-garden:latest
```

Or specify in `.env`:
```bash
IMAGE_NAME=ghcr.io/bissquit/incident-garden
IMAGE_TAG=v1.0.0
```

### Configuration

Environment variables in `.env`:
- `POSTGRES_PORT` - PostgreSQL host port (default: 5432)
- `APP_PORT` - Application host port (default: 8080)
- `IMAGE_NAME` - Docker image name (default: statuspage)
- `IMAGE_TAG` - Docker image tag (default: latest)
- `JWT_SECRET_KEY` - **Required**, min 32 characters
- `POSTGRES_PASSWORD` - **Change in production**

**Note:** All Docker Compose commands explicitly use `.env` file from project root via `--env-file .env` flag.

## Documentation

### API Documentation

Full REST API documentation is available in [docs/api/](./docs/api/):

- [Overview and basics](./docs/api/README.md)
- [Authentication](./docs/api/01-auth.md)
- [Service catalog](./docs/api/02-catalog.md)
- [Events (incidents and scheduled maintenance)](./docs/api/03-events.md)
- [Event templates](./docs/api/04-templates.md)
- [Notifications](./docs/api/05-notifications.md)
- [Public endpoints](./docs/api/06-public-status.md)

### Test Users

By default, test users are created in the system:

| Email                | Password  | Role     | Description                   |
|----------------------|-----------|----------|-------------------------------|
| admin@example.com    | admin123  | admin    | Full access to all features   |
| operator@example.com | admin123  | operator | Incident and event management |
| user@example.com     | user123   | user     | Basic user                    |

**⚠️ IMPORTANT:** For development and testing only!

### Architecture

Detailed documentation on architecture, principles, and roadmap is available in [CLAUDE.md](./CLAUDE.md).

## Technologies

- **Language**: Go 1.22+
- **HTTP Router**: chi
- **Database**: PostgreSQL 15+
- **Migrations**: golang-migrate
- **Logging**: slog (stdlib)
- **Metrics**: Prometheus

## CI/CD

The project uses GitHub Actions for automation:

- **Lint**: code checking with golangci-lint
- **Test**: running unit and integration tests with PostgreSQL
- **Build**: binary build and successful compilation check

CI configuration is available in [.github/workflows/ci.yml](./.github/workflows/ci.yml)

## License

Apache License 2.0

## Contributing

Any contributions are welcome! Please create issues and pull requests.
