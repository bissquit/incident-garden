package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bissquit/incident-management/internal/config"
	"github.com/bissquit/incident-management/internal/pkg/httputil"
	"github.com/bissquit/incident-management/internal/pkg/postgres"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	config *config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	server *http.Server
}

func New(cfg *config.Config) (*App, error) {
	logger := initLogger(cfg.Log)

	db, err := postgres.Connect(context.Background(), postgres.Config{
		URL:             cfg.Database.URL,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	app := &App{
		config: cfg,
		logger: logger,
		db:     db,
	}

	router := app.setupRouter()

	app.server = &http.Server{
		Addr:         fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	return app, nil
}

func (a *App) Run() error {
	a.logger.Info("starting server",
		"host", a.config.Server.Host,
		"port", a.config.Server.Port,
	)

	if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server error: %w", err)
	}

	return nil
}

func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down server")

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	a.db.Close()

	return nil
}

func (a *App) setupRouter() *chi.Mux {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	r.Get("/healthz", a.healthzHandler)
	r.Get("/readyz", a.readyzHandler)

	return r
}

func (a *App) healthzHandler(w http.ResponseWriter, r *http.Request) {
	httputil.Text(w, http.StatusOK, "OK")
}

func (a *App) readyzHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.db.Ping(ctx); err != nil {
		a.logger.Error("readiness check failed", "error", err)
		httputil.Text(w, http.StatusServiceUnavailable, "Database unavailable")
		return
	}

	httputil.Text(w, http.StatusOK, "OK")
}

func initLogger(cfg config.LogConfig) *slog.Logger {
	var level slog.Level
	switch cfg.Level {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	var handler slog.Handler
	opts := &slog.HandlerOptions{Level: level}

	if cfg.Format == "json" {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	} else {
		handler = slog.NewTextHandler(os.Stdout, opts)
	}

	return slog.New(handler)
}
