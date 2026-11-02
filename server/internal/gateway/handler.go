package gateway

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Handler provides REST API endpoints for managing proxy routes and endpoint limits.
type Handler struct {
	store *Store
}

// NewHandler creates a new gateway Handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes mounts gateway management routes on the given router.
func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/proxy-routes", h.ListRoutes)
	r.Post("/proxy-routes", h.CreateRoute)
	r.Put("/proxy-routes/{id}", h.UpdateRoute)
	r.Delete("/proxy-routes/{id}", h.DeleteRoute)

	r.Get("/proxy-endpoint-limits", h.ListEndpointLimits)
	r.Post("/proxy-endpoint-limits", h.CreateEndpointLimit)
	r.Delete("/proxy-endpoint-limits/{id}", h.DeleteEndpointLimit)
}

// ListRoutes returns all proxy routes for an agent.
func (h *Handler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		apierror.BadRequest(w, "agent_id query parameter required")
		return
	}

	routes, err := h.store.ListRoutes(r.Context(), agentID)
	if err != nil {
		apierror.InternalError(w, "failed to list routes")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(routes) //nolint:errcheck
}

type createRouteRequest struct {
	AgentID         string `json:"agent_id"`
	HostPattern     string `json:"host_pattern"`
	PathPattern     string `json:"path_pattern"`
	SecretID        string `json:"secret_id"`
	InjectionType   string `json:"injection_type"`
	InjectionKey    string `json:"injection_key"`
	InjectionFormat string `json:"injection_format"`
}

// CreateRoute creates a new proxy route with auto-generated placeholder.
func (h *Handler) CreateRoute(w http.ResponseWriter, r *http.Request) {
	_ = auth.UserIDFromContext(r.Context()) // ensure JWT auth

	var req createRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.AgentID == "" || req.HostPattern == "" || req.SecretID == "" {
		apierror.BadRequest(w, "agent_id, host_pattern, and secret_id are required")
		return
	}

	// Defaults
	if req.PathPattern == "" {
		req.PathPattern = "/*"
	}
	if req.InjectionType == "" {
		req.InjectionType = "bearer"
	}
	if req.InjectionKey == "" {
		req.InjectionKey = "Authorization"
	}
	if req.InjectionFormat == "" {
		req.InjectionFormat = "Bearer {value}"
	}

	route := &ProxyRoute{
		AgentID:         req.AgentID,
		HostPattern:     req.HostPattern,
		PathPattern:     req.PathPattern,
		SecretID:        req.SecretID,
		InjectionType:   req.InjectionType,
		InjectionKey:    req.InjectionKey,
		InjectionFormat: req.InjectionFormat,
		Enabled:         true,
	}

	if err := h.store.CreateRoute(r.Context(), route); err != nil {
		apierror.InternalError(w, "failed to create route")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(route) //nolint:errcheck
}

type updateRouteRequest struct {
	HostPattern     string `json:"host_pattern"`
	PathPattern     string `json:"path_pattern"`
	SecretID        string `json:"secret_id"`
	InjectionType   string `json:"injection_type"`
	InjectionKey    string `json:"injection_key"`
	InjectionFormat string `json:"injection_format"`
	Enabled         *bool  `json:"enabled"`
}

// UpdateRoute updates an existing proxy route.
func (h *Handler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	var req updateRouteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	route := &ProxyRoute{
		HostPattern:     req.HostPattern,
		PathPattern:     req.PathPattern,
		SecretID:        req.SecretID,
		InjectionType:   req.InjectionType,
		InjectionKey:    req.InjectionKey,
		InjectionFormat: req.InjectionFormat,
		Enabled:         enabled,
	}

	if err := h.store.UpdateRoute(r.Context(), id, route); err != nil {
		apierror.NotFound(w, "route not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"}) //nolint:errcheck
}

// DeleteRoute removes a proxy route.
func (h *Handler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteRoute(r.Context(), id); err != nil {
		apierror.NotFound(w, "route not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ListEndpointLimits returns all endpoint limits for an agent.
func (h *Handler) ListEndpointLimits(w http.ResponseWriter, r *http.Request) {
	agentID := r.URL.Query().Get("agent_id")
	if agentID == "" {
		apierror.BadRequest(w, "agent_id query parameter required")
		return
	}

	limits, err := h.store.ListEndpointLimits(r.Context(), agentID)
	if err != nil {
		apierror.InternalError(w, "failed to list limits")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(limits) //nolint:errcheck
}

type createEndpointLimitRequest struct {
	AgentID     string `json:"agent_id"`
	HostPattern string `json:"host_pattern"`
	PathPattern string `json:"path_pattern"`
	RPM         int    `json:"rpm"`
	Blocked     bool   `json:"blocked"`
}

// CreateEndpointLimit creates a new endpoint rate limit.
func (h *Handler) CreateEndpointLimit(w http.ResponseWriter, r *http.Request) {
	var req createEndpointLimitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.AgentID == "" || req.HostPattern == "" {
		apierror.BadRequest(w, "agent_id and host_pattern are required")
		return
	}
	if req.PathPattern == "" {
		req.PathPattern = "/*"
	}
	if req.RPM <= 0 {
		req.RPM = 60
	}

	limit := &EndpointLimit{
		AgentID:     req.AgentID,
		HostPattern: req.HostPattern,
		PathPattern: req.PathPattern,
		RPM:         req.RPM,
		Blocked:     req.Blocked,
	}

	if err := h.store.CreateEndpointLimit(r.Context(), limit); err != nil {
		apierror.InternalError(w, "failed to create limit")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(limit) //nolint:errcheck
}

// DeleteEndpointLimit removes an endpoint limit.
func (h *Handler) DeleteEndpointLimit(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := h.store.DeleteEndpointLimit(r.Context(), id); err != nil {
		apierror.NotFound(w, "limit not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
