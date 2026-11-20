package auth

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// updateProfile handles PATCH /me/profile — editable profile fields only.
// Currently: display_name (shown on the dashboard profile and in approval
// context). The account email is intentionally NOT editable here: changing
// it requires the verification flow, not a silent swap.
func (h *Handler) updateProfile(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		apierror.Unauthorized(w, "authentication required")
		return
	}

	var body struct {
		DisplayName *string `json:"display_name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&body); err != nil {
		apierror.BadRequest(w, "invalid body")
		return
	}

	displayName := ""
	if body.DisplayName != nil {
		displayName = strings.TrimSpace(*body.DisplayName)
		if len(displayName) > 100 {
			apierror.BadRequest(w, "display_name too long (max 100 characters)")
			return
		}
	}

	if _, err := h.pool.Exec(r.Context(),
		`UPDATE users SET display_name = $1, updated_at = now() WHERE id = $2`,
		displayName, userID,
	); err != nil {
		log.Printf("updateProfile: failed to update display name: %v", err)
		apierror.InternalError(w, "failed to update profile")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"ok":           true,
		"display_name": displayName,
	})
}
