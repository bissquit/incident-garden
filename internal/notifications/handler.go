package notifications

import (
	"encoding/json"
	"net/http"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

var errorMappings = []httputil.ErrorMapping{
	{Error: ErrChannelNotFound, Status: http.StatusNotFound, Message: "notification channel not found"},
	{Error: ErrSubscriptionNotFound, Status: http.StatusNotFound, Message: "subscription not found"},
	{Error: ErrChannelNotOwned, Status: http.StatusForbidden, Message: "channel does not belong to user"},
}

// Handler handles HTTP requests for the notifications module.
type Handler struct {
	service   *Service
	validator *validator.Validate
}

// NewHandler creates a new notifications handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
	}
}

// RegisterRoutes registers notification routes (require auth).
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/me/channels", func(r chi.Router) {
		r.Get("/", h.ListChannels)
		r.Post("/", h.CreateChannel)
		r.Patch("/{id}", h.UpdateChannel)
		r.Delete("/{id}", h.DeleteChannel)
		r.Post("/{id}/verify", h.VerifyChannel)
	})

	r.Route("/me/subscriptions", func(r chi.Router) {
		r.Get("/", h.GetSubscription)
		r.Post("/", h.CreateOrUpdateSubscription)
		r.Delete("/", h.DeleteSubscription)
	})
}

// CreateChannelRequest represents request body for creating a channel.
type CreateChannelRequest struct {
	Type   string `json:"type" validate:"required,oneof=email telegram"`
	Target string `json:"target" validate:"required"`
}

// UpdateChannelRequest represents request body for updating a channel.
type UpdateChannelRequest struct {
	IsEnabled bool `json:"is_enabled"`
}

// UpdateSubscriptionRequest represents request body for updating subscription.
type UpdateSubscriptionRequest struct {
	ServiceIDs []string `json:"service_ids"`
}

// ListChannels handles GET /me/channels.
func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	channels, err := h.service.ListUserChannels(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, channels)
}

// CreateChannel handles POST /me/channels.
func (h *Handler) CreateChannel(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	var req CreateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		httputil.ValidationError(w, err)
		return
	}

	channel, err := h.service.CreateChannel(r.Context(), userID, domain.ChannelType(req.Type), req.Target)
	if err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusCreated, channel)
}

// UpdateChannel handles PATCH /me/channels/{id}.
func (h *Handler) UpdateChannel(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	var req UpdateChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	channel, err := h.service.UpdateChannel(r.Context(), userID, channelID, req.IsEnabled)
	if err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, channel)
}

// DeleteChannel handles DELETE /me/channels/{id}.
func (h *Handler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	if err := h.service.DeleteChannel(r.Context(), userID, channelID); err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VerifyChannel handles POST /me/channels/{id}/verify.
func (h *Handler) VerifyChannel(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	channel, err := h.service.VerifyChannel(r.Context(), userID, channelID)
	if err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, channel)
}

// GetSubscription handles GET /me/subscriptions.
func (h *Handler) GetSubscription(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	sub, err := h.service.GetOrCreateSubscription(r.Context(), userID)
	if err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, sub)
}

// CreateOrUpdateSubscription handles POST /me/subscriptions.
func (h *Handler) CreateOrUpdateSubscription(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	var req UpdateSubscriptionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	sub, err := h.service.UpdateSubscriptionServices(r.Context(), userID, req.ServiceIDs)
	if err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, sub)
}

// DeleteSubscription handles DELETE /me/subscriptions.
func (h *Handler) DeleteSubscription(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	if err := h.service.DeleteSubscription(r.Context(), userID); err != nil {
		httputil.HandleError(w, err, errorMappings)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

