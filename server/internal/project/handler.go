package project

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/internal/policy"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes returns project routes. Workspace-scoped routes are mounted under
// /workspaces/{workspace_id}/projects. Project-direct routes use {project_id}.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.registerRoutes(r)
	return r
}

// RegisterRoutes registers project routes onto an existing router group.
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.registerRoutes(r)
}

func (h *Handler) registerRoutes(r chi.Router) {
	// workspace-scoped
	r.Post("/workspaces/{workspace_id}/projects", h.CreateProject)
	r.Get("/workspaces/{workspace_id}/projects", h.ListProjects)
	// project-direct
	r.Get("/projects/{project_id}", h.GetProject)
	r.Post("/projects/{project_id}/members", h.AddMember)
	r.Get("/projects/{project_id}/members", h.ListMembers)
}

type createProjectRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *Handler) CreateProject(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")

	var req createProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.Name == "" {
		apierror.BadRequest(w, "name is required")
		return
	}
	if req.Slug == "" {
		apierror.BadRequest(w, "slug is required")
		return
	}

	p, err := h.service.Create(r.Context(), workspaceID, req.Name, req.Slug)
	if err != nil {
		log.Printf("Failed to create project: %v", err)
		apierror.InternalError(w, "failed to create project")
		return
	}

	// Auto-add creator as project owner
	userID := auth.UserIDFromContext(r.Context())
	if userID != "" {
		if _, err := h.service.AddMember(r.Context(), p.ID, userID, "owner"); err != nil {
			log.Printf("Failed to add creator as project owner: %v", err)
			// Project was created but ownership failed — not fatal, log and continue
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p) //nolint:errcheck
}

func (h *Handler) ListProjects(w http.ResponseWriter, r *http.Request) {
	workspaceID := chi.URLParam(r, "workspace_id")

	projects, err := h.service.ListByWorkspace(r.Context(), workspaceID)
	if err != nil {
		log.Printf("Failed to list projects: %v", err)
		apierror.InternalError(w, "failed to list projects")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(projects) //nolint:errcheck
}

func (h *Handler) GetProject(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	p, err := h.service.Get(r.Context(), projectID)
	if err != nil {
		log.Printf("Failed to get project: %v", err)
		apierror.InternalError(w, "failed to get project")
		return
	}
	if p == nil {
		apierror.NotFound(w, "project not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p) //nolint:errcheck
}

type addMemberRequest struct {
	UserID string `json:"user_id"`
	Role   string `json:"role"`
}

func (h *Handler) AddMember(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	var req addMemberRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.UserID == "" {
		apierror.BadRequest(w, "user_id is required")
		return
	}
	if req.Role == "" {
		apierror.BadRequest(w, "role is required")
		return
	}

	m, err := h.service.AddMember(r.Context(), projectID, req.UserID, req.Role)
	if err != nil {
		log.Printf("Failed to add project member: %v", err)
		apierror.InternalError(w, "failed to add member")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(m) //nolint:errcheck
}

func (h *Handler) ListMembers(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	members, err := h.service.ListMembers(r.Context(), projectID)
	if err != nil {
		log.Printf("Failed to list project members: %v", err)
		apierror.InternalError(w, "failed to list members")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(members) //nolint:errcheck
}

// GetProjectPolicy handles GET /projects/{project_id}/policy
func (h *Handler) GetProjectPolicy(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	policyJSON, err := h.service.GetPolicy(r.Context(), projectID)
	if err != nil {
		apierror.NotFound(w, "project not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(policyJSON) //nolint:errcheck
}

// PutProjectPolicy handles PUT /projects/{project_id}/policy
func (h *Handler) PutProjectPolicy(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	var cp policy.CustomPolicy
	if err := json.NewDecoder(r.Body).Decode(&cp); err != nil {
		apierror.BadRequest(w, "invalid policy body")
		return
	}
	if err := cp.Validate(); err != nil {
		apierror.BadRequest(w, err.Error())
		return
	}
	policyJSON, _ := json.Marshal(cp)
	if err := h.service.SetPolicy(r.Context(), projectID, policyJSON); err != nil {
		apierror.NotFound(w, "project not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(policyJSON) //nolint:errcheck
}
