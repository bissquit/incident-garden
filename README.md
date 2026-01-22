# StatusPage Service

An open-source self-hosted status page service for displaying service states and managing incidents.

## About the Project

StatusPage is a simple and lightweight cloud-native service for managing status pages and incidents. An alternative to Atlassian Statuspage, Cachet, and Instatus, but with a focus on simplicity and self-hosting.

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
git clone https://github.com/bissquit/incident-management.git
cd incident-management
```

### Local Development

```bash
# Show available commands
make help

# Run with docker-compose
make docker-up

# Run in development mode (with hot-reload)
make dev
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
