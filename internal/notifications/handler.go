package notifications

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/bissquit/incident-garden/internal/pkg/httputil"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

var errorMappings = []httputil.ErrorMapping{
	{Error: ErrChannelNotFound, Status: http.StatusNotFound, Message: "notification channel not found"},
	{Error: ErrChannelNotOwned, Status: http.StatusForbidden, Message: "channel does not belong to user"},
	{Error: ErrChannelAlreadyExists, Status: http.StatusConflict, Message: "channel with this email already exists"},
	{Error: ErrVerificationCodeNotFound, Status: http.StatusBadRequest, Message: "verification code expired, request a new one"},
	{Error: ErrVerificationCodeInvalid, Status: http.StatusBadRequest, Message: "invalid verification code"},
	{Error: ErrTooManyAttempts, Status: http.StatusTooManyRequests, Message: "too many attempts, request a new code"},
	{Error: ErrResendTooSoon, Status: http.StatusTooManyRequests, Message: "please wait before requesting a new code"},
	{Error: ErrChannelAlreadyVerified, Status: http.StatusBadRequest, Message: "channel already verified"},
	{Error: ErrResendNotSupported, Status: http.StatusBadRequest, Message: "resend only available for email channels"},
	{Error: ErrChannelNotVerified, Status: http.StatusBadRequest, Message: "channel must be verified first"},
	{Error: ErrServicesNotFound, Status: http.StatusBadRequest, Message: "one or more services not found"},
	{Error: ErrCannotDeleteDefaultChannel, Status: http.StatusConflict, Message: "cannot delete default channel"},
	{Error: ErrChannelTypeDisabled, Status: http.StatusBadRequest, Message: "channel type is not available"},
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
		r.Post("/{id}/resend-code", h.ResendVerificationCode)
	})

	// Subscription endpoints
	r.Get("/me/subscriptions", h.GetSubscriptions)
	r.Put("/me/channels/{id}/subscriptions", h.SetChannelSubscriptions)
}

// CreateChannelRequest represents request body for creating a channel.
type CreateChannelRequest struct {
	Type   string `json:"type" validate:"required,oneof=email telegram mattermost"`
	Target string `json:"target" validate:"required"`
}

// UpdateChannelRequest represents request body for updating a channel.
type UpdateChannelRequest struct {
	IsEnabled *bool `json:"is_enabled"`
}

// VerifyChannelRequest represents request body for verifying a channel.
type VerifyChannelRequest struct {
	Code string `json:"code" validate:"required,len=6,numeric"`
}

// ListChannels handles GET /me/channels.
func (h *Handler) ListChannels(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	channels, err := h.service.ListUserChannels(r.Context(), userID)
	if err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
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
		httputil.HandleError(r.Context(), w, err, errorMappings)
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
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, channel)
}

// DeleteChannel handles DELETE /me/channels/{id}.
func (h *Handler) DeleteChannel(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	if err := h.service.DeleteChannel(r.Context(), userID, channelID); err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// VerifyChannel handles POST /me/channels/{id}/verify.
func (h *Handler) VerifyChannel(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	var req VerifyChannelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		// For non-email channels, body might be empty
		if !errors.Is(err, &json.SyntaxError{}) {
			req.Code = "" // Allow empty code for test message verification
		}
	}

	// Validate only for email channels (code required)
	if req.Code != "" {
		if err := h.validator.Struct(req); err != nil {
			httputil.Error(w, http.StatusBadRequest, "code must be 6 digits")
			return
		}
	}

	channel, err := h.service.VerifyChannel(r.Context(), userID, channelID, req.Code)
	if err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, channel)
}

// ResendVerificationCode handles POST /me/channels/{id}/resend-code.
func (h *Handler) ResendVerificationCode(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	err := h.service.ResendVerificationCode(r.Context(), userID, channelID)
	if err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, map[string]string{"message": "verification code sent"})
}

// GetSubscriptions handles GET /me/subscriptions.
func (h *Handler) GetSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())

	matrix, err := h.service.GetSubscriptionsMatrix(r.Context(), userID)
	if err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, matrix)
}

// SetSubscriptionsRequest represents request body for setting channel subscriptions.
type SetSubscriptionsRequest struct {
	SubscribeToAllServices bool     `json:"subscribe_to_all_services"`
	ServiceIDs             []string `json:"service_ids" validate:"dive,uuid"`
}

// SetChannelSubscriptions handles PUT /me/channels/{id}/subscriptions.
func (h *Handler) SetChannelSubscriptions(w http.ResponseWriter, r *http.Request) {
	userID := httputil.GetUserID(r.Context())
	channelID := chi.URLParam(r, "id")

	var req SetSubscriptionsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httputil.Error(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		httputil.ValidationError(w, err)
		return
	}

	// Validation: if subscribeAll is true, service_ids must be empty
	if req.SubscribeToAllServices && len(req.ServiceIDs) > 0 {
		httputil.Error(w, http.StatusBadRequest, "service_ids must be empty when subscribe_to_all_services is true")
		return
	}

	err := h.service.SetChannelSubscriptions(r.Context(), userID, channelID, req.SubscribeToAllServices, req.ServiceIDs)
	if err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	// Return updated subscriptions
	subscribeAll, serviceIDs, err := h.service.GetChannelSubscriptions(r.Context(), channelID)
	if err != nil {
		httputil.HandleError(r.Context(), w, err, errorMappings)
		return
	}

	httputil.Success(w, http.StatusOK, map[string]interface{}{
		"channel_id":                channelID,
		"subscribe_to_all_services": subscribeAll,
		"subscribed_service_ids":    serviceIDs,
	})
}

// GetNotificationsConfig handles GET /notifications/config.
func (h *Handler) GetNotificationsConfig(w http.ResponseWriter, _ *http.Request) {
	config := h.service.GetAvailableChannels()
	httputil.Success(w, http.StatusOK, config)
}
