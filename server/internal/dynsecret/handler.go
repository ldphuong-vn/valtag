package dynsecret

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/agent"
	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/internal/rbac"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Handler exposes dynamic secret provider and lease HTTP endpoints.
type Handler struct {
	service *Service
	db      *pgxpool.Pool
}

// NewHandler creates a new Handler.
func NewHandler(service *Service, db *pgxpool.Pool) *Handler {
	return &Handler{service: service, db: db}
}

// Routes returns a chi.Router with all dynamic secret routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.registerRoutes(r)
	return r
}

// RegisterRoutes registers dynsecret routes onto an existing router group.
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.registerRoutes(r)
}

// RegisterLeaseRoute registers lease routes on the dual-auth group (user JWT
// or agent token) — the MCP request_dynamic_secret tool calls it with an agent
// token and got a 401 on the JWT-only group (report §3.3 #4). CreateLease and
// RevokeLease enforce the agent/user authz split themselves. DELETE lives here
// too: agents revoke their own leases (the MCP revoke_credential tool — an
// agent-owned lease was previously unrecoverable by its owner, dogfood
// 2026-11-12).
func (h *Handler) RegisterLeaseRoute(r chi.Router) {
	r.Post("/providers/{provider_id}/leases", h.CreateLease)
	r.Delete("/leases/{lease_id}", h.RevokeLease)
}

func (h *Handler) registerRoutes(r chi.Router) {
	r.With(rbac.Middleware(h.db, "project_id", rbac.ResourceDynSecret, rbac.ActionWrite)).
		Post("/projects/{project_id}/providers", h.CreateProvider)
	r.With(rbac.Middleware(h.db, "project_id", rbac.ResourceDynSecret, rbac.ActionRead)).
		Get("/projects/{project_id}/providers", h.ListProviders)
	r.Get("/providers/{provider_id}/leases", h.ListLeases)
}

type createProviderRequest struct {
	Name         string            `json:"name"`
	ProviderType string            `json:"provider_type"`
	Config       map[string]string `json:"config"`
}

type createLeaseRequest struct {
	AgentID    string `json:"agent_id"`
	RequestID  string `json:"request_id,omitempty"`
	TTLSeconds int    `json:"ttl_seconds"`
}

type leaseResponse struct {
	ID          string            `json:"id"`
	Credentials map[string]string `json:"credentials"`
	ExpiresAt   time.Time         `json:"expires_at"`
	TTLSeconds  int               `json:"ttl_seconds"`
}

func (h *Handler) CreateProvider(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	userID := auth.UserIDFromContext(r.Context())

	var req createProviderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.Name == "" {
		apierror.BadRequest(w, "name is required")
		return
	}
	if req.ProviderType == "" {
		apierror.BadRequest(w, "provider_type is required")
		return
	}

	pc, err := h.service.CreateProvider(r.Context(), projectID, req.Name, req.ProviderType, req.Config, userID)
	if err != nil {
		log.Printf("Failed to create provider: %v", err)
		apierror.InternalError(w, "failed to create provider")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(pc.ToResponse()) //nolint:errcheck
}

func (h *Handler) ListProviders(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	providers, err := h.service.ListProviders(r.Context(), projectID)
	if err != nil {
		log.Printf("Failed to list providers: %v", err)
		apierror.InternalError(w, "failed to list providers")
		return
	}

	responses := make([]ProviderResponse, len(providers))
	for i := range providers {
		responses[i] = providers[i].ToResponse()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"providers": responses}) //nolint:errcheck
}

func (h *Handler) CreateLease(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "provider_id")
	userID := auth.UserIDFromContext(r.Context())
	agentID := agent.AgentIDFromContext(r.Context())
	if userID == "" && agentID == "" {
		apierror.Unauthorized(w, "authentication required")
		return
	}

	var req createLeaseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.TTLSeconds <= 0 || req.TTLSeconds > 3600 {
		apierror.BadRequest(w, "ttl_seconds must be between 1 and 3600")
		return
	}

	if agentID != "" {
		// Agent path (report §3.3 #4): agents mint leases for themselves on
		// providers of their own project — never for another agent.
		if req.AgentID != agentID {
			apierror.Forbidden(w, "agents may only mint leases for themselves")
			return
		}
		if !h.checkAgentProviderAccess(r.Context(), w, providerID, agentID) {
			return
		}
	} else if !h.checkProviderAccess(r.Context(), w, providerID, userID, rbac.ActionWrite) {
		return
	}

	lease, err := h.service.CreateLease(r.Context(), providerID, req.AgentID, req.RequestID, req.TTLSeconds)
	if err != nil {
		log.Printf("Failed to create lease: %v", err)
		apierror.InternalError(w, "failed to create lease")
		return
	}

	resp := leaseResponse{
		ID:          lease.ID,
		Credentials: lease.Credentials,
		ExpiresAt:   lease.ExpiresAt,
		TTLSeconds:  req.TTLSeconds,
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(resp) //nolint:errcheck
}

func (h *Handler) ListLeases(w http.ResponseWriter, r *http.Request) {
	providerID := chi.URLParam(r, "provider_id")
	userID := auth.UserIDFromContext(r.Context())
	if !h.checkProviderAccess(r.Context(), w, providerID, userID, rbac.ActionRead) {
		return
	}

	leases, err := h.service.ListActiveLeases(r.Context(), providerID)
	if err != nil {
		log.Printf("Failed to list leases: %v", err)
		apierror.InternalError(w, "failed to list leases")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"leases": leases}) //nolint:errcheck
}

func (h *Handler) RevokeLease(w http.ResponseWriter, r *http.Request) {
	leaseID := chi.URLParam(r, "lease_id")
	agentID := agent.AgentIDFromContext(r.Context())
	userID := auth.UserIDFromContext(r.Context())

	if agentID != "" {
		// Agent path: an agent revokes only a lease minted for itself — never
		// another agent's or a user's lease.
		ownerAgentID, err := h.service.GetLeaseAgentID(r.Context(), leaseID)
		if err != nil {
			apierror.NotFound(w, "lease not found")
			return
		}
		if ownerAgentID == nil || *ownerAgentID != agentID {
			apierror.Forbidden(w, "lease belongs to a different agent")
			return
		}
	} else {
		providerID, err := h.service.GetLeaseProviderID(r.Context(), leaseID)
		if err != nil {
			apierror.NotFound(w, "lease not found")
			return
		}
		if !h.checkProviderAccess(r.Context(), w, providerID, userID, rbac.ActionWrite) {
			return
		}
	}

	if err := h.service.RevokeLease(r.Context(), leaseID); err != nil {
		log.Printf("Failed to revoke lease: %v", err)
		apierror.NotFound(w, "lease not found or already revoked")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// checkProviderAccess verifies the user has the required permission on the provider's project.
func (h *Handler) checkProviderAccess(ctx context.Context, w http.ResponseWriter, providerID, userID string, action rbac.Action) bool {
	pc, err := h.service.GetProvider(ctx, providerID)
	if err != nil {
		apierror.NotFound(w, "provider not found")
		return false
	}
	var role string
	err = h.db.QueryRow(ctx, `SELECT role FROM project_memberships WHERE project_id = $1 AND user_id = $2`, pc.ProjectID, userID).Scan(&role)
	if err != nil {
		apierror.Forbidden(w, "access denied")
		return false
	}
	if !rbac.Can(rbac.RoleFromProjectMembership(role), rbac.ResourceDynSecret, action) {
		apierror.Forbidden(w, "insufficient permissions")
		return false
	}
	return true
}

// checkAgentProviderAccess grants an agent lease-minting rights only when
// the provider belongs to the agent's own project.
func (h *Handler) checkAgentProviderAccess(ctx context.Context, w http.ResponseWriter, providerID, agentID string) bool {
	pc, err := h.service.GetProvider(ctx, providerID)
	if err != nil {
		apierror.NotFound(w, "provider not found")
		return false
	}
	agentProject, err := agentGetProject(ctx, h.db, agentID)
	if err != nil || agentProject == "" {
		apierror.NotFound(w, "agent not found")
		return false
	}
	if pc.ProjectID != agentProject {
		apierror.Forbidden(w, "provider belongs to a different project")
		return false
	}
	return true
}

// agentGetProject resolves the owning project of an agent identity.
func agentGetProject(ctx context.Context, db *pgxpool.Pool, agentID string) (string, error) {
	var projectID string
	err := db.QueryRow(ctx, `SELECT project_id FROM agent_identities WHERE id = $1`, agentID).Scan(&projectID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return projectID, nil
}
