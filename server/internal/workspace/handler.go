package workspace

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// Routes returns workspace routes that expect to be mounted under /orgs/{org_id}/workspaces.
// The org_id param is read from the parent router's URL context.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/", h.createWorkspace)
	r.Get("/", h.listWorkspaces)
	return r
}

type createWorkspaceRequest struct {
	Name string `json:"name"`
	Slug string `json:"slug"`
}

func (h *Handler) createWorkspace(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	var req createWorkspaceRequest
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

	ws, err := h.service.Create(r.Context(), orgID, req.Name, req.Slug)
	if err != nil {
		log.Printf("Failed to create workspace: %v", err)
		apierror.InternalError(w, "failed to create workspace")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ws) //nolint:errcheck
}

func (h *Handler) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	workspaces, err := h.service.ListByOrg(r.Context(), orgID)
	if err != nil {
		log.Printf("Failed to list workspaces: %v", err)
		apierror.InternalError(w, "failed to list workspaces")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(workspaces) //nolint:errcheck
}
