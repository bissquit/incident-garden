//go:build integration

package integration

import (
	"context"
	"log"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/bissquit/incident-garden/internal/app"
	"github.com/bissquit/incident-garden/internal/config"
	"github.com/bissquit/incident-garden/internal/testutil"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	testServer    *httptest.Server
	testClient    *testutil.Client
	testValidator *testutil.OpenAPIValidator
	testDB        *pgxpool.Pool

	// Mailpit for E2E email testing
	mailpitContainer *testutil.MailpitContainer
	mailpitClient    *MailpitClient
)

// OpenAPI spec path relative to the tests/integration directory.
const openAPISpecPath = "../../api/openapi/openapi.yaml"

// newTestClient creates a new test client with OpenAPI validation enabled.
// Use this at the beginning of each test that makes API calls.
func newTestClient(t *testing.T) *testutil.Client {
	t.Helper()
	client := testutil.NewClientWithValidator(testServer.URL, testValidator)
	client.SetT(t)
	return client
}

// newTestClientWithoutValidation creates a test client without OpenAPI validation.
// Use this for tests that intentionally test error responses or invalid scenarios.
func newTestClientWithoutValidation() *testutil.Client {
	return testutil.NewClient(testServer.URL)
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	pgContainer, err := testutil.NewPostgresContainer(ctx)
	if err != nil {
		log.Fatalf("start postgres: %v", err)
	}
	defer func() {
		if err := pgContainer.Terminate(ctx); err != nil {
			log.Printf("terminate postgres: %v", err)
		}
	}()

	// Start Mailpit container (for E2E email tests)
	mailpitContainer, err = testutil.NewMailpitContainer(ctx)
	if err != nil {
		log.Fatalf("start mailpit: %v", err)
	}
	defer func() {
		if err := mailpitContainer.Terminate(ctx); err != nil {
			log.Printf("terminate mailpit: %v", err)
		}
	}()

	mailpitClient = NewMailpitClient(
		mailpitContainer.APIHost,
		mailpitContainer.APIPort,
	)

	migrator, err := migrate.New(
		"file://../../migrations",
		pgContainer.ConnectionString,
	)
	if err != nil {
		log.Fatalf("create migrator: %v", err)
	}
	if err := migrator.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("run migrations: %v", err)
	}

	cfg := &config.Config{
		Server: config.ServerConfig{
			Host:         "127.0.0.1",
			Port:         "0",
			MetricsPort:  "0",
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: config.DatabaseConfig{
			URL:             pgContainer.ConnectionString,
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
			ConnectTimeout:  30 * time.Second,
			ConnectAttempts: 3,
		},
		Log: config.LogConfig{
			Level:  "error",
			Format: "text",
		},
		JWT: config.JWTConfig{
			SecretKey:            "test-secret-key",
			AccessTokenDuration:  15 * time.Minute,
			RefreshTokenDuration: 24 * time.Hour,
		},
		Cookie: config.CookieConfig{
			Secure: false, // Not using HTTPS in tests
			Domain: "",
		},
		// Notifications DISABLED at app level for test isolation.
		//
		// Why this approach:
		// 1. Mock-based tests (notifications_dispatch_test.go, notifications_events_test.go) create
		//    their own workers with mock senders to verify dispatch logic without real SMTP.
		// 2. E2E tests (notifications_email_e2e_test.go) create their own workers with Mailpit
		//    sender via setupE2ENotificationInfra() to test real SMTP delivery.
		// 3. If app-level notifications were enabled, the global worker would compete with
		//    test-specific workers for queue items, causing race conditions and flaky tests.
		// 4. This also prevents "subscribe_to_all" channels from receiving unexpected notifications
		//    during unrelated tests (e.g., event creation tests that don't test notifications).
		//
		// Alternative considered: Enable app notifications and use mailpitClient.DeleteAllMessages()
		// before each test. Rejected because it doesn't solve the mock-based test interference issue.
		Notifications: config.NotificationsConfig{
			Enabled: false,
			Email: config.EmailConfig{
				Enabled: true,
			},
			Telegram: config.TelegramConfig{
				Enabled: true,
			},
		},
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("create app: %v", err)
	}

	// Create a direct DB connection for tests that need it
	testDB, err = pgxpool.New(ctx, pgContainer.ConnectionString)
	if err != nil {
		log.Fatalf("create test db pool: %v", err)
	}

	testServer = httptest.NewServer(application.Router())

	// Load OpenAPI validator
	testValidator, err = testutil.LoadOpenAPIValidator(openAPISpecPath)
	if err != nil {
		log.Fatalf("load OpenAPI validator: %v", err)
	}

	// Create client with OpenAPI validation enabled
	testClient = testutil.NewClientWithValidator(testServer.URL, testValidator)

	code := m.Run()

	testServer.Close()
	testDB.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown app: %v", err)
	}

	os.Exit(code)
}
