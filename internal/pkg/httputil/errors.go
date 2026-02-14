package httputil

import (
	"context"
	"errors"
	"net/http"

	"github.com/bissquit/incident-garden/internal/pkg/ctxlog"
)

// ErrorMapping defines how a domain error maps to an HTTP response.
type ErrorMapping struct {
	Error   error
	Status  int
	Message string // if empty, uses err.Error()
}

// HandleError maps a domain error to an HTTP response using provided mappings.
// If no mapping matches, logs the error and returns 500 Internal Server Error.
func HandleError(ctx context.Context, w http.ResponseWriter, err error, mappings []ErrorMapping) {
	for _, m := range mappings {
		if errors.Is(err, m.Error) {
			msg := m.Message
			if msg == "" {
				msg = err.Error()
			}
			Error(w, m.Status, msg)
			return
		}
	}
	ctxlog.FromContext(ctx).Error("internal error", "error", err)
	Error(w, http.StatusInternalServerError, "internal error")
}
