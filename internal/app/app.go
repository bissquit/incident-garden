// Package app provides application initialization and lifecycle management.
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/bissquit/incident-garden/internal/catalog"
	catalogpostgres "github.com/bissquit/incident-garden/internal/catalog/postgres"
	"github.com/bissquit/incident-garden/internal/config"
	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/events"
	eventspostgres "github.com/bissquit/incident-garden/internal/events/postgres"
	"github.com/bissquit/incident-garden/internal/identity"
	"github.com/bissquit/incident-garden/internal/identity/jwt"
	identitypostgres "github.com/bissquit/incident-garden/internal/identity/postgres"
	"github.com/bissquit/incident-garden/internal/notifications"
	"github.com/bissquit/incident-garden/internal/notifications/email"
	notificationspostgres "github.com/bissquit/incident-garden/internal/notifications/postgres"
	"github.com/bissquit/incident-garden/internal/notifications/telegram"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/bissquit/incident-garden/internal/pkg/postgres"
	"github.com/bissquit/incident-garden/internal/version"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
)

// App represents the application instance.
type App struct {
	config *config.Config
	logger *slog.Logger
	db     *pgxpool.Pool
	server *http.Server
}

// New creates a new application instance.
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

// Run starts the HTTP server.
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

// Shutdown gracefully shuts down the application.
func (a *App) Shutdown(ctx context.Context) error {
	a.logger.Info("shutting down server")

	if err := a.server.Shutdown(ctx); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	a.db.Close()

	return nil
}

// Router returns the HTTP handler for testing.
func (a *App) Router() http.Handler {
	return a.server.Handler
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
	r.Get("/version", a.versionHandler)

	r.Get("/api/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/x-yaml")
		http.ServeFile(w, r, "api/openapi/openapi.yaml")
	})

	r.Get("/docs", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>StatusPage API</title>
    <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css">
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
    <script>
        SwaggerUIBundle({
            url: "/api/openapi.yaml",
            dom_id: '#swagger-ui',
            presets: [SwaggerUIBundle.presets.apis, SwaggerUIBundle.SwaggerUIStandalonePreset],
            layout: "BaseLayout"
        });
    </script>
</body>
</html>`))
	})

	identityRepo := identitypostgres.NewRepository(a.db)
	jwtAuth := jwt.NewAuthenticator(jwt.Config{
		SecretKey:            a.config.JWT.SecretKey,
		AccessTokenDuration:  a.config.JWT.AccessTokenDuration,
		RefreshTokenDuration: a.config.JWT.RefreshTokenDuration,
	}, identityRepo)
	identityService := identity.NewService(identityRepo, jwtAuth)
	identityHandler := identity.NewHandler(identityService)

	catalogRepo := catalogpostgres.NewRepository(a.db)
	catalogService := catalog.NewService(catalogRepo)
	catalogHandler := catalog.NewHandler(catalogService)

	eventsRepo := eventspostgres.NewRepository(a.db)
	eventsService := events.NewService(eventsRepo)
	eventsHandler := events.NewHandler(eventsService)

	notificationsRepo := notificationspostgres.NewRepository(a.db)
	emailSender := email.NewSender(email.Config{})
	telegramSender := telegram.NewSender(telegram.Config{})
	dispatcher := notifications.NewDispatcher(notificationsRepo, emailSender, telegramSender)
	notificationsService := notifications.NewService(notificationsRepo, dispatcher)
	notificationsHandler := notifications.NewHandler(notificationsService)

	r.Route("/api/v1", func(r chi.Router) {
		identityHandler.RegisterRoutes(r)

		eventsHandler.RegisterPublicRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(httputil.AuthMiddleware(identityService))

			identityHandler.RegisterProtectedRoutes(r)
			notificationsHandler.RegisterRoutes(r)

			r.Group(func(r chi.Router) {
				r.Use(httputil.RequireRole(domain.RoleOperator))
				eventsHandler.RegisterOperatorRoutes(r)
			})

			r.Group(func(r chi.Router) {
				r.Use(httputil.RequireRole(domain.RoleAdmin))
				catalogHandler.RegisterRoutes(r)
				eventsHandler.RegisterAdminRoutes(r)
			})
		})

		r.Get("/services", catalogHandler.ListServices)
		r.Get("/services/{slug}", catalogHandler.GetService)
		r.Get("/groups", catalogHandler.ListGroups)
		r.Get("/groups/{slug}", catalogHandler.GetGroup)
	})

	return r
}

func (a *App) healthzHandler(w http.ResponseWriter, _ *http.Request) {
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

func (a *App) versionHandler(w http.ResponseWriter, _ *http.Request) {
	httputil.JSON(w, http.StatusOK, map[string]string{
		"version":    version.Version,
		"commit":     version.GitCommit,
		"build_date": version.BuildDate,
	})
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
