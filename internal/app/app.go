// Package app provides application initialization and lifecycle management.
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
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
	"github.com/bissquit/incident-garden/internal/notifications/mattermost"
	notificationspostgres "github.com/bissquit/incident-garden/internal/notifications/postgres"
	"github.com/bissquit/incident-garden/internal/notifications/telegram"
	"github.com/bissquit/incident-garden/internal/pkg/ctxlog"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/bissquit/incident-garden/internal/pkg/metrics"
	"github.com/bissquit/incident-garden/internal/pkg/postgres"
	"github.com/bissquit/incident-garden/internal/version"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// App represents the application instance.
type App struct {
	config             *config.Config
	logger             *slog.Logger
	db                 *pgxpool.Pool
	server             *http.Server
	metricsServer      *http.Server
	metricsCancel      context.CancelFunc
	notificationWorker *notifications.Worker
}

// New creates a new application instance.
func New(cfg *config.Config) (*App, error) {
	logger := initLogger(cfg.Log)

	connectCtx, connectCancel := context.WithTimeout(context.Background(), cfg.Database.ConnectTimeout)
	defer connectCancel()

	db, err := postgres.Connect(connectCtx, postgres.Config{
		URL:             cfg.Database.URL,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		ConnectAttempts: cfg.Database.ConnectAttempts,
	})
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}

	metricsCtx, metricsCancel := context.WithCancel(context.Background())

	app := &App{
		config:        cfg,
		logger:        logger,
		db:            db,
		metricsCancel: metricsCancel,
	}

	go app.collectDBMetrics(metricsCtx)

	router, notificationWorker, err := app.setupRouter(metricsCtx)
	if err != nil {
		db.Close()
		metricsCancel()
		return nil, fmt.Errorf("setup router: %w", err)
	}

	app.notificationWorker = notificationWorker

	app.server = &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.Port),
		Handler:           router,
		ReadTimeout:       cfg.Server.ReadTimeout,
		ReadHeaderTimeout: cfg.Server.ReadHeaderTimeout,
		WriteTimeout:      cfg.Server.WriteTimeout,
		IdleTimeout:       cfg.Server.IdleTimeout,
	}

	// Metrics server on separate port
	metricsRouter := chi.NewRouter()
	metricsRouter.Handle("/metrics", promhttp.Handler())

	app.metricsServer = &http.Server{
		Addr:              fmt.Sprintf("%s:%s", cfg.Server.Host, cfg.Server.MetricsPort),
		Handler:           metricsRouter,
		ReadTimeout:       5 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	return app, nil
}

// Run starts the HTTP servers.
func (a *App) Run() error {
	// Start metrics server in background
	go func() {
		a.logger.Info("starting metrics server",
			"host", a.config.Server.Host,
			"port", a.config.Server.MetricsPort,
		)
		if err := a.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			a.logger.Error("metrics server error", "error", err)
		}
	}()

	// Start main server
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
	a.logger.Info("shutting down servers")

	a.metricsCancel()

	// Stop notification worker first
	if a.notificationWorker != nil {
		a.notificationWorker.Stop()
	}

	// Shutdown both servers in parallel
	var wg sync.WaitGroup
	var errs []error
	var mu sync.Mutex

	wg.Add(2)

	go func() {
		defer wg.Done()
		if err := a.server.Shutdown(ctx); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("shutdown server: %w", err))
			mu.Unlock()
		}
	}()

	go func() {
		defer wg.Done()
		if err := a.metricsServer.Shutdown(ctx); err != nil {
			mu.Lock()
			errs = append(errs, fmt.Errorf("shutdown metrics server: %w", err))
			mu.Unlock()
		}
	}()

	wg.Wait()

	a.db.Close()

	return errors.Join(errs...)
}

func (a *App) collectDBMetrics(ctx context.Context) {
	// Collect immediately on start
	metrics.RecordDBPoolMetrics(a.db)

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			metrics.RecordDBPoolMetrics(a.db)
		case <-ctx.Done():
			return
		}
	}
}

func (a *App) collectQueueMetrics(ctx context.Context, repo notifications.Repository) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			stats, err := repo.GetQueueStats(ctx)
			if err != nil {
				slog.Error("failed to get queue stats", "error", err)
				continue
			}
			notifications.RecordQueueStats(stats)
		case <-ctx.Done():
			return
		}
	}
}

// Router returns the HTTP handler for testing.
func (a *App) Router() http.Handler {
	return a.server.Handler
}

// NotificationWorker returns the notification worker instance.
// Used in tests to access worker state. Returns nil if notifications disabled.
func (a *App) NotificationWorker() *notifications.Worker {
	return a.notificationWorker
}

func (a *App) setupRouter(ctx context.Context) (*chi.Mux, *notifications.Worker, error) {
	r := chi.NewRouter()

	// Metrics middleware must be first to measure full request time
	r.Use(httputil.MetricsMiddleware)

	// CORS must be early to handle preflight requests before other middleware
	r.Use(httputil.CORSMiddleware(a.config.CORS.AllowedOrigins))
	r.Use(middleware.RequestID)
	r.Use(httputil.RequestLoggerMiddleware(a.logger))
	r.Use(middleware.RealIP)
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

	catalogRepo := catalogpostgres.NewRepository(a.db)
	catalogService := catalog.NewService(catalogRepo)

	// Setup notifications first (needed for identity hook)
	notificationsRepo := notificationspostgres.NewRepository(a.db)
	var notificationsService *notifications.Service
	var notificationsHandler *notifications.Handler
	var notifier events.EventNotifier
	var notificationWorker *notifications.Worker

	slog.Info("notifications configured",
		"enabled", a.config.Notifications.Enabled,
		"email_enabled", a.config.Notifications.Email.Enabled,
		"telegram_enabled", a.config.Notifications.Telegram.Enabled,
	)

	if a.config.Notifications.Enabled {
		emailSender, err := email.NewSender(email.Config{
			Enabled:      a.config.Notifications.Email.Enabled,
			SMTPHost:     a.config.Notifications.Email.SMTPHost,
			SMTPPort:     a.config.Notifications.Email.SMTPPort,
			SMTPUser:     a.config.Notifications.Email.SMTPUser,
			SMTPPassword: a.config.Notifications.Email.SMTPPassword,
			FromAddress:  a.config.Notifications.Email.FromAddress,
			BatchSize:    a.config.Notifications.Email.BatchSize,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create email sender: %w", err)
		}

		if !a.config.Notifications.Email.Enabled {
			slog.Warn("email sender is disabled: email notifications and verification codes will not be sent")
		}

		telegramSender, err := telegram.NewSender(telegram.Config{
			Enabled:   a.config.Notifications.Telegram.Enabled,
			BotToken:  a.config.Notifications.Telegram.BotToken,
			RateLimit: a.config.Notifications.Telegram.RateLimit,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create telegram sender: %w", err)
		}

		if !a.config.Notifications.Telegram.Enabled {
			slog.Warn("telegram sender is disabled: telegram notifications will not be sent")
		}

		// Mattermost is always available (webhook URL is set per-channel by user)
		mattermostSender := mattermost.NewSender(mattermost.Config{})

		dispatcher := notifications.NewDispatcher(notificationsRepo, emailSender, telegramSender, mattermostSender)

		renderer, err := notifications.NewRenderer()
		if err != nil {
			return nil, nil, fmt.Errorf("create notification renderer: %w", err)
		}

		notifierConfig := notifications.NotifierConfig{
			MaxAttempts: a.config.Notifications.Retry.MaxAttempts,
		}

		notifier = notifications.NewNotifierWithConfig(
			notificationsRepo,
			renderer,
			dispatcher,
			catalogService, // implements ServiceNameResolver
			a.config.Notifications.BaseURL,
			notifierConfig,
		)

		// Create and start notification worker
		workerConfig := notifications.WorkerConfig{
			BatchSize:         a.config.Notifications.Worker.BatchSize,
			PollInterval:      a.config.Notifications.Worker.PollInterval,
			MaxAttempts:       a.config.Notifications.Retry.MaxAttempts,
			InitialBackoff:    a.config.Notifications.Retry.InitialBackoff,
			MaxBackoff:        a.config.Notifications.Retry.MaxBackoff,
			BackoffMultiplier: a.config.Notifications.Retry.BackoffMultiplier,
			NumWorkers:        a.config.Notifications.Worker.NumWorkers,
		}

		notificationWorker = notifications.NewWorker(workerConfig, notificationsRepo, dispatcher, renderer)
		notificationWorker.Start(ctx)

		// Start queue metrics collection
		go a.collectQueueMetrics(ctx, notificationsRepo)

		notificationsService = notifications.NewService(notificationsRepo, dispatcher, catalogService)
	} else {
		// Notifications disabled - create service with nil dispatcher
		notificationsService = notifications.NewService(notificationsRepo, nil, catalogService)
	}
	notificationsHandler = notifications.NewHandler(notificationsService)

	// Setup identity with notifications hook
	identityRepo := identitypostgres.NewRepository(a.db)
	jwtAuth := jwt.NewAuthenticator(jwt.Config{
		SecretKey:            a.config.JWT.SecretKey,
		AccessTokenDuration:  a.config.JWT.AccessTokenDuration,
		RefreshTokenDuration: a.config.JWT.RefreshTokenDuration,
	}, identityRepo)
	identityService := identity.NewService(identityRepo, jwtAuth, notificationsService)
	identityHandler := identity.NewHandler(identityService, identity.CookieSettings{
		Secure:               a.config.Cookie.Secure,
		Domain:               a.config.Cookie.Domain,
		AccessTokenDuration:  a.config.JWT.AccessTokenDuration,
		RefreshTokenDuration: a.config.JWT.RefreshTokenDuration,
	})

	// Setup events with notifier
	eventsRepo := eventspostgres.NewRepository(a.db)
	eventsService := events.NewService(eventsRepo, catalogService, catalogService, notifier)
	eventsHandler := events.NewHandler(eventsService)

	catalogHandler := catalog.NewHandler(catalogService, eventsService)

	r.Route("/api/v1", func(r chi.Router) {
		identityHandler.RegisterRoutes(r)

		eventsHandler.RegisterPublicRoutes(r)
		eventsHandler.RegisterPublicEventRoutes(r)

		r.Group(func(r chi.Router) {
			r.Use(httputil.AuthMiddleware(identityService))

			identityHandler.RegisterProtectedRoutes(r)
			notificationsHandler.RegisterRoutes(r)

			r.Group(func(r chi.Router) {
				r.Use(httputil.RequireRole(domain.RoleOperator))
				eventsHandler.RegisterOperatorRoutes(r)
				catalogHandler.RegisterOperatorRoutes(r)
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

		catalogHandler.RegisterPublicServiceRoutes(r)
	})

	return r, notificationWorker, nil
}

func (a *App) healthzHandler(w http.ResponseWriter, _ *http.Request) {
	httputil.Text(w, http.StatusOK, "OK")
}

func (a *App) readyzHandler(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := a.db.Ping(ctx); err != nil {
		ctxlog.FromContext(r.Context()).Error("readiness check failed", "error", err)
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
