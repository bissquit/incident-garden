package httputil

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/bissquit/incident-garden/internal/pkg/ctxlog"
	"github.com/go-chi/chi/v5/middleware"
)

// RequestLoggerMiddleware creates a middleware that:
// 1. Injects a logger with request_id into context
// 2. Logs HTTP requests in JSON format via slog
func RequestLoggerMiddleware(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := middleware.GetReqID(r.Context())
			logger := base.With("request_id", reqID)
			ctx := ctxlog.WithLogger(r.Context(), logger)

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r.WithContext(ctx))

			logger.Info("http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"bytes", ww.BytesWritten(),
				"duration_ms", time.Since(start).Milliseconds(),
				"remote_addr", r.RemoteAddr,
			)
		})
	}
}
