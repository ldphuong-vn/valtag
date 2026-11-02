package agent

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/valt-dev/valt/server/internal/auth"
	"github.com/valt-dev/valt/server/internal/rbac"
	"github.com/valt-dev/valt/server/pkg/apierror"
)

// Handler exposes agent identity HTTP endpoints.
type Handler struct {
	service *Service
	pool    *pgxpool.Pool
}

// NewHandler creates a new agent Handler.
func NewHandler(service *Service, pool *pgxpool.Pool) *Handler {
	return &Handler{service: service, pool: pool}
}

// Routes returns a chi.Router with all agent routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.registerRoutes(r)
	return r
}

// RegisterRoutes registers agent routes onto an existing router group.
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.registerRoutes(r)
}

func (h *Handler) registerRoutes(r chi.Router) {
	// Project-scoped routes: project RBAC applies directly from the URL param.
	r.With(rbac.Middleware(h.pool, "project_id", rbac.ResourceAgent, rbac.ActionWrite)).
		Post("/projects/{project_id}/agents", h.CreateAgent)
	r.With(rbac.Middleware(h.pool, "project_id", rbac.ResourceAgent, rbac.ActionRead)).
		Get("/projects/{project_id}/agents", h.ListAgents)

	// Agent-scoped routes: resolve the agent's owning project first; unknown
	// agents 404 so callers cannot probe existence.
	r.With(rbac.RequireAgentAccess(h.pool, "agent_id", rbac.ResourceAgent, rbac.ActionRead)).
		Get("/agents/{agent_id}", h.GetAgent)
	r.With(rbac.RequireAgentAccess(h.pool, "agent_id", rbac.ResourceAgent, rbac.ActionWrite)).
		Post("/agents/{agent_id}/tokens", h.IssueToken)
	r.With(rbac.RequireAgentAccess(h.pool, "agent_id", rbac.ResourceAgent, rbac.ActionWrite)).
		Delete("/agents/{agent_id}/tokens/{token_id}", h.RevokeToken)
	r.With(rbac.RequireAgentAccess(h.pool, "agent_id", rbac.ResourceAgent, rbac.ActionRead)).
		Get("/agents/{agent_id}/tokens", h.ListTokens)
}

type createAgentRequest struct {
	Name          string   `json:"name"`
	Description   string   `json:"description"`
	AgentType     string   `json:"agent_type"`
	MaxSessionTTL int      `json:"max_session_ttl"`
	AllowedScopes []string `json:"allowed_scopes"`
}

func (h *Handler) CreateAgent(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	userID := auth.UserIDFromContext(r.Context())

	var req createAgentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.Name == "" {
		apierror.BadRequest(w, "name is required")
		return
	}

	a, err := h.service.Create(r.Context(), projectID, req.Name, req.Description,
		req.AgentType, userID, req.MaxSessionTTL, req.AllowedScopes)
	if err != nil {
		log.Printf("Failed to create agent: %v", err)
		apierror.InternalError(w, "failed to create agent")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(a) //nolint:errcheck
}

func (h *Handler) ListAgents(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	agents, err := h.service.ListByProject(r.Context(), projectID)
	if err != nil {
		log.Printf("Failed to list agents: %v", err)
		apierror.InternalError(w, "failed to list agents")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(agents) //nolint:errcheck
}

func (h *Handler) GetAgent(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agent_id")

	a, err := h.service.Get(r.Context(), agentID)
	if err != nil {
		log.Printf("Failed to get agent: %v", err)
		apierror.InternalError(w, "failed to get agent")
		return
	}
	if a == nil {
		apierror.NotFound(w, "agent not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(a) //nolint:errcheck
}

type issueTokenRequest struct {
	Scopes           []string `json:"scopes"`
	ExpiresInSeconds *int     `json:"expires_in_seconds,omitempty"`
}

func (h *Handler) IssueToken(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agent_id")

	var req issueTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	var expiresAt *time.Time
	if req.ExpiresInSeconds != nil && *req.ExpiresInSeconds > 0 {
		t := time.Now().Add(time.Duration(*req.ExpiresInSeconds) * time.Second)
		expiresAt = &t
	}

	token, err := h.service.IssueToken(r.Context(), agentID, req.Scopes, expiresAt)
	if err != nil {
		log.Printf("Failed to issue token: %v", err)
		apierror.InternalError(w, "failed to issue token")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(token) //nolint:errcheck
}

func (h *Handler) RevokeToken(w http.ResponseWriter, r *http.Request) {
	tokenID := chi.URLParam(r, "token_id")

	if err := h.service.RevokeToken(r.Context(), tokenID); err != nil {
		log.Printf("Failed to revoke token: %v", err)
		apierror.NotFound(w, "token not found or already revoked")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) ListTokens(w http.ResponseWriter, r *http.Request) {
	agentID := chi.URLParam(r, "agent_id")

	tokens, err := h.service.ListTokens(r.Context(), agentID)
	if err != nil {
		log.Printf("Failed to list tokens: %v", err)
		apierror.InternalError(w, "failed to list tokens")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens) //nolint:errcheck
}
