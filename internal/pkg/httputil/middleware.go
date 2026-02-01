package httputil

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"github.com/bissquit/incident-garden/internal/domain"
)

// Cookie and header names for authentication.
const (
	AccessTokenCookie  = "access_token"
	RefreshTokenCookie = "refresh_token"
	CSRFTokenCookie    = "csrf_token"
	CSRFTokenHeader    = "X-CSRF-Token"
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
// It supports both cookie-based and header-based authentication.
// Priority: 1) Cookie (with CSRF check) 2) Authorization header (no CSRF).
func AuthMiddleware(validator TokenValidator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var token string
			var fromCookie bool

			// Try cookie first
			if cookie, err := r.Cookie(AccessTokenCookie); err == nil && cookie.Value != "" {
				token = cookie.Value
				fromCookie = true
			}

			// Fallback to Authorization header
			if token == "" {
				authHeader := r.Header.Get("Authorization")
				if authHeader != "" {
					parts := strings.SplitN(authHeader, " ", 2)
					if len(parts) == 2 && strings.ToLower(parts[0]) == "bearer" {
						token = parts[1]
					}
				}
			}

			if token == "" {
				respondError(w, http.StatusUnauthorized, "missing authentication")
				return
			}

			// CSRF check for cookie-based auth on state-changing methods
			if fromCookie && isStateChangingMethod(r.Method) {
				if !validateCSRF(r) {
					respondError(w, http.StatusForbidden, "invalid csrf token")
					return
				}
			}

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

// isStateChangingMethod returns true for methods that modify state.
func isStateChangingMethod(method string) bool {
	switch method {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	}
	return false
}

// validateCSRF checks that X-CSRF-Token header matches csrf_token cookie.
func validateCSRF(r *http.Request) bool {
	cookie, err := r.Cookie(CSRFTokenCookie)
	if err != nil || cookie.Value == "" {
		return false
	}

	headerToken := r.Header.Get(CSRFTokenHeader)
	if headerToken == "" {
		return false
	}

	return cookie.Value == headerToken
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
