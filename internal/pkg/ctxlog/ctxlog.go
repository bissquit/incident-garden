// Package ctxlog provides context-aware logging utilities.
package ctxlog

import (
	"context"
	"log/slog"
)

type ctxKey struct{}

// FromContext extracts the logger from context.
// Returns slog.Default() if no logger is found.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(ctxKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, logger)
}
