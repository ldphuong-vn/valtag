package notify

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
	"github.com/ldphuong-vn/valtag/server/pkg/validator"
)

// ChannelHandler serves notification channel CRUD endpoints.
type ChannelHandler struct {
	store *ChannelStore
}

// NewChannelHandler creates a ChannelHandler.
func NewChannelHandler(store *ChannelStore) *ChannelHandler {
	return &ChannelHandler{store: store}
}

// List handles GET /me/notification-channels
func (h *ChannelHandler) List(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	channels, err := h.store.List(r.Context(), userID)
	if err != nil {
		apierror.InternalError(w, "failed to list channels")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"channels": channels})
}

type upsertChannelBody struct {
	ChannelType string `json:"channel_type"`
	Handle      string `json:"handle"`
}

// Upsert handles POST /me/notification-channels
func (h *ChannelHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	var body upsertChannelBody
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		apierror.BadRequest(w, "invalid body")
		return
	}
	if body.ChannelType != "slack" && body.ChannelType != "telegram" && body.ChannelType != "email" {
		apierror.BadRequest(w, "channel_type must be slack, telegram, or email")
		return
	}
	if body.Handle == "" {
		apierror.BadRequest(w, "handle required")
		return
	}
	ch, err := h.store.Upsert(r.Context(), userID, body.ChannelType, body.Handle)
	if err != nil {
		apierror.InternalError(w, "failed to save channel")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(ch)
}

// Delete handles DELETE /me/notification-channels/{id}
func (h *ChannelHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())
	channelID := chi.URLParam(r, "id")
	if _, err := validator.ValidateUUID(channelID); err != nil {
		apierror.BadRequest(w, "invalid channel id")
		return
	}
	if err := h.store.Delete(r.Context(), userID, channelID); err != nil {
		apierror.NotFound(w, "channel not found")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
