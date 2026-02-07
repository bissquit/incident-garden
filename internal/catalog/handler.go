// Package catalog provides HTTP handlers and business logic for managing services and service groups.
package catalog

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/bissquit/incident-garden/internal/domain"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
)

// Handler handles HTTP requests for the catalog module.
type Handler struct {
	service   *Service
	validator *validator.Validate
}

// NewHandler creates a new catalog handler.
func NewHandler(service *Service) *Handler {
	return &Handler{
		service:   service,
		validator: validator.New(),
	}
}

// RegisterRoutes registers all HTTP routes for the catalog module.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Route("/groups", func(r chi.Router) {
		r.Get("/", h.ListGroups)
		r.Post("/", h.CreateGroup)
		r.Get("/{slug}", h.GetGroup)
		r.Patch("/{slug}", h.UpdateGroup)
		r.Delete("/{slug}", h.DeleteGroup)
		r.Post("/{slug}/restore", h.RestoreGroup)
	})

	r.Route("/services", func(r chi.Router) {
		r.Get("/", h.ListServices)
		r.Post("/", h.CreateService)
		r.Get("/{slug}", h.GetService)
		r.Patch("/{slug}", h.UpdateService)
		r.Delete("/{slug}", h.DeleteService)
		r.Post("/{slug}/restore", h.RestoreService)
		r.Get("/{slug}/tags", h.GetServiceTags)
		r.Put("/{slug}/tags", h.UpdateServiceTags)
	})
}

// CreateGroupRequest represents the request body for creating a service group.
type CreateGroupRequest struct {
	Name        string `json:"name" validate:"required,min=1,max=255"`
	Slug        string `json:"slug" validate:"required,min=1,max=255"`
	Description string `json:"description"`
	Order       int    `json:"order"`
}

// ToDomain converts the request to a domain model.
func (r *CreateGroupRequest) ToDomain() *domain.ServiceGroup {
	return &domain.ServiceGroup{
		Name:        r.Name,
		Slug:        r.Slug,
		Description: r.Description,
		ServiceIDs:  make([]string, 0),
		Order:       r.Order,
	}
}

// UpdateGroupRequest represents the request body for updating a service group.
type UpdateGroupRequest struct {
	Name        string    `json:"name" validate:"required,min=1,max=255"`
	Slug        string    `json:"slug" validate:"required,min=1,max=255"`
	Description string    `json:"description"`
	Order       int       `json:"order"`
	ServiceIDs  *[]string `json:"service_ids"`
}

// CreateServiceRequest represents the request body for creating a service.
type CreateServiceRequest struct {
	Name        string            `json:"name" validate:"required,min=1,max=255"`
	Slug        string            `json:"slug" validate:"required,min=1,max=255"`
	Description string            `json:"description"`
	Status      string            `json:"status" validate:"omitempty,oneof=operational degraded partial_outage major_outage maintenance"`
	GroupIDs    []string          `json:"group_ids"`
	Order       int               `json:"order"`
	Tags        map[string]string `json:"tags"`
}

// ToDomain converts the request to a domain model.
func (r *CreateServiceRequest) ToDomain() *domain.Service {
	status := domain.ServiceStatus(r.Status)
	if status == "" {
		status = domain.ServiceStatusOperational
	}

	groupIDs := r.GroupIDs
	if groupIDs == nil {
		groupIDs = make([]string, 0)
	}

	return &domain.Service{
		Name:        r.Name,
		Slug:        r.Slug,
		Description: r.Description,
		Status:      status,
		GroupIDs:    groupIDs,
		Order:       r.Order,
	}
}

// UpdateServiceRequest represents the request body for updating a service.
type UpdateServiceRequest struct {
	Name        string   `json:"name" validate:"required,min=1,max=255"`
	Slug        string   `json:"slug" validate:"required,min=1,max=255"`
	Description string   `json:"description"`
	Status      string   `json:"status" validate:"required,oneof=operational degraded partial_outage major_outage maintenance"`
	GroupIDs    []string `json:"group_ids"`
	Order       int      `json:"order"`
}

// UpdateServiceTagsRequest represents the request body for updating service tags.
type UpdateServiceTagsRequest struct {
	Tags map[string]string `json:"tags" validate:"required"`
}

// CreateGroup handles POST /groups request.
func (h *Handler) CreateGroup(w http.ResponseWriter, r *http.Request) {
	var req CreateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	group := req.ToDomain()
	if err := h.service.CreateGroup(r.Context(), group); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, group)
}

// GetGroup handles GET /groups/{slug} request.
func (h *Handler) GetGroup(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	group, err := h.service.GetGroupBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, group)
}

// ListGroups handles GET /groups request.
func (h *Handler) ListGroups(w http.ResponseWriter, r *http.Request) {
	filter := GroupFilter{}

	if r.URL.Query().Get("include_archived") == "true" {
		filter.IncludeArchived = true
	}

	groups, err := h.service.ListGroups(r.Context(), filter)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, groups)
}

// UpdateGroup handles PATCH /groups/{slug} request.
func (h *Handler) UpdateGroup(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	existing, err := h.service.GetGroupBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	var req UpdateGroupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	existing.Name = req.Name
	existing.Slug = req.Slug
	existing.Description = req.Description
	existing.Order = req.Order

	if err := h.service.UpdateGroup(r.Context(), existing); err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Update service memberships if provided
	if req.ServiceIDs != nil {
		if err := h.service.UpdateGroupServices(r.Context(), existing.ID, *req.ServiceIDs); err != nil {
			h.handleServiceError(w, err)
			return
		}
		existing.ServiceIDs = *req.ServiceIDs
	}

	h.respondJSON(w, http.StatusOK, existing)
}

// DeleteGroup handles DELETE /groups/{slug} request.
func (h *Handler) DeleteGroup(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	group, err := h.service.GetGroupBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	if err := h.service.DeleteGroup(r.Context(), group.ID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestoreGroup handles POST /groups/{slug}/restore request.
func (h *Handler) RestoreGroup(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	group, err := h.service.GetGroupBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	if err := h.service.RestoreGroup(r.Context(), group.ID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Return the restored group
	group, err = h.service.GetGroupBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, group)
}

// CreateService handles POST /services request.
func (h *Handler) CreateService(w http.ResponseWriter, r *http.Request) {
	var req CreateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	service := req.ToDomain()
	if err := h.service.CreateService(r.Context(), service); err != nil {
		h.handleServiceError(w, err)
		return
	}

	if len(req.Tags) > 0 {
		if err := h.service.UpdateServiceTags(r.Context(), service.ID, req.Tags); err != nil {
			h.handleServiceError(w, err)
			return
		}
	}

	// Return with effective status
	result, err := h.service.GetServiceByIDWithEffectiveStatus(r.Context(), service.ID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusCreated, result)
}

// GetService handles GET /services/{slug} request.
func (h *Handler) GetService(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	service, err := h.service.GetServiceBySlugWithEffectiveStatus(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, service)
}

// ListServices handles GET /services request.
func (h *Handler) ListServices(w http.ResponseWriter, r *http.Request) {
	filter := ServiceFilter{}

	if groupID := r.URL.Query().Get("group_id"); groupID != "" {
		filter.GroupID = &groupID
	}

	if status := r.URL.Query().Get("status"); status != "" {
		s := domain.ServiceStatus(status)
		filter.Status = &s
	}

	if r.URL.Query().Get("include_archived") == "true" {
		filter.IncludeArchived = true
	}

	services, err := h.service.ListServicesWithEffectiveStatus(r.Context(), filter)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, services)
}

// UpdateService handles PATCH /services/{slug} request.
func (h *Handler) UpdateService(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	existing, err := h.service.GetServiceBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	var req UpdateServiceRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	existing.Name = req.Name
	existing.Slug = req.Slug
	existing.Description = req.Description
	existing.Status = domain.ServiceStatus(req.Status)
	existing.GroupIDs = req.GroupIDs
	if existing.GroupIDs == nil {
		existing.GroupIDs = make([]string, 0)
	}
	existing.Order = req.Order

	if err := h.service.UpdateService(r.Context(), existing); err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Return with effective status
	result, err := h.service.GetServiceBySlugWithEffectiveStatus(r.Context(), existing.Slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// DeleteService handles DELETE /services/{slug} request.
func (h *Handler) DeleteService(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	service, err := h.service.GetServiceBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	if err := h.service.DeleteService(r.Context(), service.ID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RestoreService handles POST /services/{slug}/restore request.
func (h *Handler) RestoreService(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	service, err := h.service.GetServiceBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	if err := h.service.RestoreService(r.Context(), service.ID); err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Return the restored service with effective status
	result, err := h.service.GetServiceBySlugWithEffectiveStatus(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, result)
}

// GetServiceTags handles GET /services/{slug}/tags request.
func (h *Handler) GetServiceTags(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	service, err := h.service.GetServiceBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	tags, err := h.service.GetServiceTags(r.Context(), service.ID)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	tagsMap := make(map[string]string)
	for _, tag := range tags {
		tagsMap[tag.Key] = tag.Value
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"tags": tagsMap})
}

// UpdateServiceTags handles PUT /services/{slug}/tags request.
func (h *Handler) UpdateServiceTags(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")

	service, err := h.service.GetServiceBySlug(r.Context(), slug)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	var req UpdateServiceTagsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "invalid json")
		return
	}

	if err := h.validator.Struct(req); err != nil {
		h.respondValidationError(w, err)
		return
	}

	if err := h.service.UpdateServiceTags(r.Context(), service.ID, req.Tags); err != nil {
		h.handleServiceError(w, err)
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]interface{}{"tags": req.Tags})
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
	case errors.Is(err, ErrServiceNotFound), errors.Is(err, ErrGroupNotFound):
		h.respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrSlugExists):
		h.respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrInvalidSlug):
		h.respondError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrServiceHasActiveEvents):
		h.respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrGroupHasActiveEvents):
		h.respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrGroupHasServices):
		h.respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrAlreadyArchived):
		h.respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrNotArchived):
		h.respondError(w, http.StatusConflict, err.Error())
	default:
		slog.Error("internal error", "error", err)
		h.respondError(w, http.StatusInternalServerError, "internal error")
	}
}
