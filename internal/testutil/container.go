package testutil

import (
	"context"
	"fmt"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// PostgresContainer wraps a postgres testcontainer.
type PostgresContainer struct {
	*postgres.PostgresContainer
	ConnectionString string
}

// MailpitContainer wraps a Mailpit testcontainer for email testing.
type MailpitContainer struct {
	testcontainers.Container
	SMTPHost string
	SMTPPort int
	APIHost  string
	APIPort  int
}

// NewPostgresContainer creates a new PostgreSQL container for testing.
func NewPostgresContainer(ctx context.Context) (*PostgresContainer, error) {
	container, err := postgres.Run(ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("testdb"),
		postgres.WithUsername("testuser"),
		postgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("start postgres container: %w", err)
	}

	connStr, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		return nil, fmt.Errorf("get connection string: %w", err)
	}

	return &PostgresContainer{
		PostgresContainer: container,
		ConnectionString:  connStr,
	}, nil
}

// NewMailpitContainer creates a new Mailpit container for testing.
// Mailpit provides a fake SMTP server with REST API to inspect received emails.
func NewMailpitContainer(ctx context.Context) (*MailpitContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        "ghcr.io/axllent/mailpit:latest",
		ExposedPorts: []string{"1025/tcp", "8025/tcp"},
		WaitingFor: wait.ForAll(
			wait.ForListeningPort("1025/tcp"),
			wait.ForHTTP("/api/v1/info").WithPort("8025/tcp"),
		).WithDeadline(30 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("start mailpit container: %w", err)
	}

	host, err := container.Host(ctx)
	if err != nil {
		return nil, fmt.Errorf("get mailpit host: %w", err)
	}

	smtpPort, err := container.MappedPort(ctx, "1025/tcp")
	if err != nil {
		return nil, fmt.Errorf("get smtp port: %w", err)
	}

	apiPort, err := container.MappedPort(ctx, "8025/tcp")
	if err != nil {
		return nil, fmt.Errorf("get api port: %w", err)
	}

	return &MailpitContainer{
		Container: container,
		SMTPHost:  host,
		SMTPPort:  smtpPort.Int(),
		APIHost:   host,
		APIPort:   apiPort.Int(),
	}, nil
}
