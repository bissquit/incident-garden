package identity

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// CookieSettings contains settings for authentication cookies.
type CookieSettings struct {
	Secure               bool
	Domain               string
	AccessTokenDuration  time.Duration
	RefreshTokenDuration time.Duration
}

// Handler handles HTTP requests for the identity module.
type Handler struct {
	service        *Service
	validator      *validator.Validate
	cookieSettings CookieSettings
}

// NewHandler creates a new identity handler.
func NewHandler(service *Service, cookieSettings CookieSettings) *Handler {
	return &Handler{
		service:        service,
		validator:      validator.New(),
		cookieSettings: cookieSettings,
	}
}

// RegisterRoutes registers identity routes.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)
		r.Post("/logout", h.Logout)
	})
}

// RegisterProtectedRoutes registers routes that require authentication.
func (h *Handler) RegisterProtectedRoutes(r chi.Router) {
	r.Get("/me", h.Me)
}

// RegisterRequest represents registration request body.
type RegisterRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

// Register handles POST /auth/register.
func (h *Handler) Register(w http.ResponseWriter, r *http.Request) {
	var req RegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	user, err := h.service.Register(r.Context(), RegisterInput(req))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, user)
}

// LoginRequest represents login request body.
type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse represents login response.
type LoginResponse struct {
	User *domain.User `json:"user"`
}

// Login handles POST /auth/login.
func (h *Handler) Login(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	user, tokens, err := h.service.Login(r.Context(), LoginInput(req))
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.setAuthCookies(w, tokens)

	h.respondJSON(w, http.StatusOK, LoginResponse{
		User: user,
	})
}

// Refresh handles POST /auth/refresh.
// Reads refresh_token from cookie, issues new tokens.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	refreshToken := h.getRefreshTokenFromRequest(r)
	if refreshToken == "" {
		h.respondError(w, http.StatusBadRequest, "missing refresh token")
		return
	}

	tokens, err := h.service.RefreshTokens(r.Context(), refreshToken)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.setAuthCookies(w, tokens)

	w.WriteHeader(http.StatusNoContent)
}

// Logout handles POST /auth/logout.
// Reads refresh_token from cookie, invalidates it, clears all auth cookies.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	refreshToken := h.getRefreshTokenFromRequest(r)
	if refreshToken != "" {
		if err := h.service.Logout(r.Context(), refreshToken); err != nil {
			slog.Warn("logout error", "error", err)
		}
	}

	h.clearAuthCookies(w)

	w.WriteHeader(http.StatusNoContent)
}

// Me handles GET /me.
func (h *Handler) Me(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	if userID == "" {
		h.respondError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	user, err := h.service.GetUserByID(r.Context(), userID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, user)
}

func (h *Handler) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{"data": data}); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

func (h *Handler) respondError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"message": message},
	}); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

func (h *Handler) respondValidationError(w http.ResponseWriter, err error) {
	var details []map[string]string
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		for _, e := range validationErrors {
			details = append(details, map[string]string{
				"field":   e.Field(),
				"message": e.Tag(),
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"message": "validation error",
			"details": details,
		},
	}); err != nil {
		slog.Error("failed to encode validation error response", "error", err)
	}
}

// setAuthCookies sets access_token, refresh_token, and csrf_token cookies.
func (h *Handler) setAuthCookies(w http.ResponseWriter, tokens *TokenPair) {
	// Access token cookie - available to all paths
	http.SetCookie(w, &http.Cookie{
		Name:     httputil.AccessTokenCookie,
		Value:    tokens.AccessToken,
		Path:     "/",
		Domain:   h.cookieSettings.Domain,
		MaxAge:   int(h.cookieSettings.AccessTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSettings.Secure,
		SameSite: http.SameSiteLaxMode,
	})

	// Refresh token cookie - only for /api/v1/auth paths
	http.SetCookie(w, &http.Cookie{
		Name:     httputil.RefreshTokenCookie,
		Value:    tokens.RefreshToken,
		Path:     "/api/v1/auth",
		Domain:   h.cookieSettings.Domain,
		MaxAge:   int(h.cookieSettings.RefreshTokenDuration.Seconds()),
		HttpOnly: true,
		Secure:   h.cookieSettings.Secure,
		SameSite: http.SameSiteStrictMode,
	})

	// CSRF token cookie - readable by JavaScript
	csrfToken := generateCSRFToken()
	http.SetCookie(w, &http.Cookie{
		Name:     httputil.CSRFTokenCookie,
		Value:    csrfToken,
		Path:     "/",
		Domain:   h.cookieSettings.Domain,
		MaxAge:   int(h.cookieSettings.AccessTokenDuration.Seconds()),
		HttpOnly: false, // Must be readable by JavaScript
		Secure:   h.cookieSettings.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// clearAuthCookies removes all auth cookies by setting Max-Age=0.
func (h *Handler) clearAuthCookies(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     httputil.AccessTokenCookie,
		Value:    "",
		Path:     "/",
		Domain:   h.cookieSettings.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSettings.Secure,
		SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     httputil.RefreshTokenCookie,
		Value:    "",
		Path:     "/api/v1/auth",
		Domain:   h.cookieSettings.Domain,
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   h.cookieSettings.Secure,
		SameSite: http.SameSiteStrictMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     httputil.CSRFTokenCookie,
		Value:    "",
		Path:     "/",
		Domain:   h.cookieSettings.Domain,
		MaxAge:   -1,
		HttpOnly: false,
		Secure:   h.cookieSettings.Secure,
		SameSite: http.SameSiteLaxMode,
	})
}

// getRefreshTokenFromRequest extracts refresh token from cookie or request body (for backward compatibility).
func (h *Handler) getRefreshTokenFromRequest(r *http.Request) string {
	// Try cookie first
	if cookie, err := r.Cookie(httputil.RefreshTokenCookie); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	// Fallback to request body for API clients
	var body struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.RefreshToken != "" {
		return body.RefreshToken
	}

	return ""
}

// generateCSRFToken generates a random CSRF token.
func generateCSRFToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// Fallback to less secure but functional token
		return hex.EncodeToString([]byte(time.Now().String()))
	}
	return hex.EncodeToString(b)
}

func (h *Handler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrUserNotFound):
		h.respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrEmailExists):
		h.respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidCredentials):
		h.respondError(w, http.StatusUnauthorized, err.Error())
	case errors.Is(err, ErrInvalidToken):
		h.respondError(w, http.StatusUnauthorized, err.Error())
	default:
		slog.Error("internal error", "error", err)
		h.respondError(w, http.StatusInternalServerError, "internal error")
	}
}
