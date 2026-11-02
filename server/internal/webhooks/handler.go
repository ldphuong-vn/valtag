package webhooks

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Handler serves webhook management endpoints.
type Handler struct {
	store *Store
}

// NewHandler creates a webhook Handler.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

type createRequest struct {
	URL         string   `json:"url"`
	Events      []string `json:"events"`
	Description string   `json:"description"`
}

// Create handles POST /orgs/{org_id}/webhooks.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")

	var req createRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	wh, err := h.store.Create(r.Context(), orgID, req.URL, req.Description, req.Events)
	if err != nil {
		log.Printf("[webhooks] create error: %v", err)
		apierror.BadRequest(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(wh) // status already sent; nothing to recover
}

// List handles GET /orgs/{org_id}/webhooks.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")
	whs, err := h.store.List(r.Context(), orgID)
	if err != nil {
		apierror.InternalError(w, "failed to list webhooks")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"webhooks": whs}) // status not yet written; Encode failure falls back to empty 200
}

// Delete handles DELETE /orgs/{org_id}/webhooks/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")
	whID := chi.URLParam(r, "id")
	if err := h.store.Delete(r.Context(), orgID, whID); err != nil {
		apierror.InternalError(w, "failed to delete webhook")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
