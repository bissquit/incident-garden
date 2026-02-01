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
)

var (
	testServer    *httptest.Server
	testClient    *testutil.Client
	testValidator *testutil.OpenAPIValidator
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
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
		},
		Database: config.DatabaseConfig{
			URL:             pgContainer.ConnectionString,
			MaxOpenConns:    5,
			MaxIdleConns:    2,
			ConnMaxLifetime: 5 * time.Minute,
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
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("create app: %v", err)
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown app: %v", err)
	}

	os.Exit(code)
}
