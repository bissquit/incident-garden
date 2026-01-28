package httputil

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bissquit/incident-garden/internal/domain"
)

// CORSMiddleware creates CORS middleware that handles preflight requests
// and adds appropriate CORS headers to responses.
func CORSMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	originsSet := make(map[string]bool, len(allowedOrigins))
	for _, o := range allowedOrigins {
		originsSet[o] = true
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			// Check if origin is allowed
			if originsSet[origin] || originsSet["*"] {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}

			// Handle preflight OPTIONS request
			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

type contextKey string

// Context keys for storing user information.
const (
	UserIDKey contextKey = "user_id"
	RoleKey   contextKey = "role"
)

// TokenValidator interface for validating tokens.
type TokenValidator interface {
	ValidateToken(ctx context.Context, token string) (userID string, role domain.Role, err error)
}

// AuthMiddleware creates authentication middleware.
func AuthMiddleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				respondError(w, http.StatusUnauthorized, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				respondError(w, http.StatusUnauthorized, "invalid authorization header format")
				return
			}

			token := parts[1]

			userID, role, err := validator.ValidateToken(r.Context(), token)
			if err != nil {
				respondError(w, http.StatusUnauthorized, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), UserIDKey, userID)
			ctx = context.WithValue(ctx, RoleKey, role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// RequireRole creates RBAC middleware.
func RequireRole(minRole domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := r.Context().Value(RoleKey).(domain.Role)
			if !ok {
				respondError(w, http.StatusUnauthorized, "unauthorized")
				return
			}

			if !role.HasPermission(minRole) {
				respondError(w, http.StatusForbidden, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// GetUserID extracts user ID from context.
func GetUserID(ctx context.Context) string {
	if id, ok := ctx.Value(UserIDKey).(string); ok {
		return id
	}
	return ""
}

// GetRole extracts role from context.
func GetRole(ctx context.Context) domain.Role {
	if role, ok := ctx.Value(RoleKey).(domain.Role); ok {
		return role
	}
	return ""
}

func respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"message": message},
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}
