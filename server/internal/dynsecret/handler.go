package dynsecret

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

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

func (h *Handler) registerRoutes(r chi.Router) {
	r.With(rbac.Middleware(h.db, "project_id", rbac.ResourceDynSecret, rbac.ActionWrite)).
		Post("/projects/{project_id}/providers", h.CreateProvider)
	r.With(rbac.Middleware(h.db, "project_id", rbac.ResourceDynSecret, rbac.ActionRead)).
		Get("/projects/{project_id}/providers", h.ListProviders)
	r.Post("/providers/{provider_id}/leases", h.CreateLease)
	r.Get("/providers/{provider_id}/leases", h.ListLeases)
	r.Delete("/leases/{lease_id}", h.RevokeLease)
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
	if !h.checkProviderAccess(r.Context(), w, providerID, userID, rbac.ActionWrite) {
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
	userID := auth.UserIDFromContext(r.Context())
	providerID, err := h.service.GetLeaseProviderID(r.Context(), leaseID)
	if err != nil {
		apierror.NotFound(w, "lease not found")
		return
	}
	if !h.checkProviderAccess(r.Context(), w, providerID, userID, rbac.ActionWrite) {
		return
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
