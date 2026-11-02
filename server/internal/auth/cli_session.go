package auth

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// CLISessionHandler handles short-lived OAuth token exchange for the valtag CLI.
type CLISessionHandler struct {
	pool    *pgxpool.Pool
	baseURL string
}

// NewCLISessionHandler creates a CLISessionHandler.
func NewCLISessionHandler(pool *pgxpool.Pool, baseURL string) *CLISessionHandler {
	return &CLISessionHandler{pool: pool, baseURL: baseURL}
}

// Start handles GET /auth/cli-start
// Creates a pending session, returns session_id + login_url.
func (h *CLISessionHandler) Start(w http.ResponseWriter, r *http.Request) {
	var sessionID string
	err := h.pool.QueryRow(r.Context(),
		`INSERT INTO cli_auth_sessions DEFAULT VALUES RETURNING id`,
	).Scan(&sessionID)
	if err != nil {
		apierror.InternalError(w, "failed to create CLI session")
		return
	}
	loginURL := h.baseURL + "/api/v1/auth/google?cli_session=" + sessionID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{ //nolint:errcheck
		"session_id": sessionID,
		"login_url":  loginURL,
	})
}

// Poll handles GET /auth/cli-poll?session={id}
// Returns pending or complete+token.
func (h *CLISessionHandler) Poll(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("session")
	if sessionID == "" {
		apierror.BadRequest(w, "session parameter required")
		return
	}
	var token *string
	var expiresAt time.Time
	err := h.pool.QueryRow(r.Context(),
		`SELECT token, expires_at FROM cli_auth_sessions WHERE id = $1`, sessionID,
	).Scan(&token, &expiresAt)
	if err != nil || time.Now().After(expiresAt) {
		apierror.NotFound(w, "session not found or expired")
		return
	}
	if token == nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "pending"}) //nolint:errcheck
		return
	}
	// Delete session after successful poll
	_, _ = h.pool.Exec(r.Context(), `DELETE FROM cli_auth_sessions WHERE id = $1`, sessionID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"status": "complete",
		"token":  *token,
	})
}
