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
	testServer *httptest.Server
	testClient *testutil.Client
)

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
	}

	application, err := app.New(cfg)
	if err != nil {
		log.Fatalf("create app: %v", err)
	}

	testServer = httptest.NewServer(application.Router())
	testClient = testutil.NewClient(testServer.URL)

	code := m.Run()

	testServer.Close()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := application.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown app: %v", err)
	}

	os.Exit(code)
}
