package identity

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// Handler handles HTTP requests for the identity module.
type Handler struct {
	service   *Service
	validator *validator.Validate
}

// NewHandler creates a new identity handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
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
	User   *domain.User `json:"user"`
	Tokens *TokenPair   `json:"tokens"`
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

	h.respondJSON(w, http.StatusOK, LoginResponse{
		User:   user,
		Tokens: tokens,
	})
}

// RefreshRequest represents refresh token request.
type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// Refresh handles POST /auth/refresh.
func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	tokens, err := h.service.RefreshTokens(r.Context(), req.RefreshToken)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, tokens)
}

// Logout handles POST /auth/logout.
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	var req RefreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.service.Logout(r.Context(), req.RefreshToken); err != nil {
		slog.Warn("logout error", "error", err)
	}

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
