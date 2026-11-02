package org

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Invitation represents a pending org membership invitation.
type Invitation struct {
	ID        string     `json:"id"`
	OrgID     string     `json:"org_id"`
	OrgName   string     `json:"org_name,omitempty"`
	Email     string     `json:"email"`
	Role      string     `json:"role"`
	InvitedBy string     `json:"invited_by"`
	Status    string     `json:"status"`
	ExpiresAt time.Time  `json:"expires_at"`
	CreatedAt time.Time  `json:"created_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
}

// AcceptResult is returned from acceptInvitation when the user must register first.
type AcceptResult struct {
	NeedsRegistration bool   `json:"needs_registration,omitempty"`
	Email             string `json:"email,omitempty"`
	OrgName           string `json:"org_name,omitempty"`
	Message           string `json:"message,omitempty"`
}

// generateInviteToken returns (raw token, sha256-hex hash).
func generateInviteToken() (string, string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating token bytes: %w", err)
	}
	raw := hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash := hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// isValidInviteRole returns true for assignable roles (not owner).
func isValidInviteRole(role string) bool {
	return role == "member" || role == "admin"
}

// callerIsAdminOrOwner checks that callerID has admin or owner role in the org.
func callerIsAdminOrOwner(ctx context.Context, s *Service, orgID, callerID string) (bool, error) {
	var role string
	err := s.pool.QueryRow(ctx,
		`SELECT role FROM org_memberships WHERE org_id = $1 AND user_id = $2`,
		orgID, callerID,
	).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return role == "owner" || role == "admin", nil
}

// createInvitation handles POST /orgs/{org_id}/invitations.
func (h *Handler) createInvitation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")
	callerID := auth.UserIDFromContext(r.Context())

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.Email == "" {
		apierror.BadRequest(w, "email is required")
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	if !isValidInviteRole(req.Role) {
		apierror.BadRequest(w, "role must be member or admin")
		return
	}

	ok, err := callerIsAdminOrOwner(r.Context(), h.service, orgID, callerID)
	if err != nil {
		log.Printf("org invitation: check caller role: %v", err)
		apierror.InternalError(w, "failed to verify permissions")
		return
	}
	if !ok {
		apierror.Forbidden(w, "only org admin or owner can invite members")
		return
	}

	// Fetch org name for email.
	o, err := h.service.Get(r.Context(), orgID)
	if err != nil || o == nil {
		apierror.NotFound(w, "org not found")
		return
	}

	// Check invitee is not already a member.
	var exists bool
	err = h.service.pool.QueryRow(r.Context(),
		`SELECT EXISTS(
			SELECT 1 FROM org_memberships m
			JOIN users u ON u.id = m.user_id
			WHERE m.org_id = $1 AND u.email = $2
		)`, orgID, req.Email,
	).Scan(&exists)
	if err != nil {
		log.Printf("org invitation: check existing member: %v", err)
		apierror.InternalError(w, "failed to check membership")
		return
	}
	if exists {
		apierror.Conflict(w, "user is already a member of this org")
		return
	}

	// Enforce max 20 pending invitations per org.
	var pendingCount int
	err = h.service.pool.QueryRow(r.Context(),
		`SELECT COUNT(*) FROM org_invitations WHERE org_id = $1 AND status = 'pending'`,
		orgID,
	).Scan(&pendingCount)
	if err != nil {
		log.Printf("org invitation: count pending: %v", err)
		apierror.InternalError(w, "failed to check pending invitations")
		return
	}
	if pendingCount >= 20 {
		apierror.BadRequest(w, "maximum 20 pending invitations per org")
		return
	}

	raw, hash, err := generateInviteToken()
	if err != nil {
		log.Printf("org invitation: generate token: %v", err)
		apierror.InternalError(w, "failed to generate invitation token")
		return
	}

	expiresAt := time.Now().Add(7 * 24 * time.Hour)
	var inv Invitation
	err = h.service.pool.QueryRow(r.Context(),
		`INSERT INTO org_invitations (org_id, email, role, token_hash, invited_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, org_id, email, role, invited_by, status, expires_at, created_at`,
		orgID, req.Email, req.Role, hash, callerID, expiresAt,
	).Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.InvitedBy, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt)
	if err != nil {
		// Unique constraint violation: pending invitation already exists.
		if isDuplicateInvitation(err) {
			apierror.Conflict(w, "a pending invitation for this email already exists")
			return
		}
		log.Printf("org invitation: insert: %v", err)
		apierror.InternalError(w, "failed to create invitation")
		return
	}

	// Send invite email (best-effort; nil sender is a no-op).
	if h.emailSender != nil {
		inviteURL := fmt.Sprintf("%s/accept-invite?token=%s", h.dashboardURL, raw)
		body := buildInviteEmail(o.Name, inviteURL)
		if emailErr := h.emailSender.Send(r.Context(), req.Email,
			fmt.Sprintf("You're invited to join %s on valtag", o.Name), body); emailErr != nil {
			log.Printf("org invitation: send email to %s: %v", req.Email, emailErr)
			// Continue — invitation is created; email failure is non-fatal.
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(inv) //nolint:errcheck
}

// listInvitations handles GET /orgs/{org_id}/invitations.
func (h *Handler) listInvitations(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")
	callerID := auth.UserIDFromContext(r.Context())

	ok, err := callerIsAdminOrOwner(r.Context(), h.service, orgID, callerID)
	if err != nil {
		log.Printf("org invitations list: check caller: %v", err)
		apierror.InternalError(w, "failed to verify permissions")
		return
	}
	if !ok {
		apierror.Forbidden(w, "only org admin or owner can view invitations")
		return
	}

	rows, err := h.service.pool.Query(r.Context(),
		`SELECT id, org_id, email, role, invited_by, status, expires_at, created_at, accepted_at
		 FROM org_invitations
		 WHERE org_id = $1 AND status = 'pending'
		 ORDER BY created_at DESC`,
		orgID,
	)
	if err != nil {
		log.Printf("org invitations list: query: %v", err)
		apierror.InternalError(w, "failed to list invitations")
		return
	}
	defer rows.Close()

	invitations := []Invitation{}
	for rows.Next() {
		var inv Invitation
		if err := rows.Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.Role,
			&inv.InvitedBy, &inv.Status, &inv.ExpiresAt, &inv.CreatedAt, &inv.AcceptedAt); err != nil {
			log.Printf("org invitations list: scan: %v", err)
			apierror.InternalError(w, "failed to read invitations")
			return
		}
		invitations = append(invitations, inv)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"invitations": invitations}) //nolint:errcheck
}

// cancelInvitation handles DELETE /orgs/{org_id}/invitations/{id}.
func (h *Handler) cancelInvitation(w http.ResponseWriter, r *http.Request) {
	orgID := chi.URLParam(r, "org_id")
	inviteID := chi.URLParam(r, "id")
	callerID := auth.UserIDFromContext(r.Context())

	ok, err := callerIsAdminOrOwner(r.Context(), h.service, orgID, callerID)
	if err != nil {
		log.Printf("org invitation cancel: check caller: %v", err)
		apierror.InternalError(w, "failed to verify permissions")
		return
	}
	if !ok {
		apierror.Forbidden(w, "only org admin or owner can cancel invitations")
		return
	}

	tag, err := h.service.pool.Exec(r.Context(),
		`UPDATE org_invitations SET status = 'cancelled'
		 WHERE id = $1 AND org_id = $2 AND status = 'pending'`,
		inviteID, orgID,
	)
	if err != nil {
		log.Printf("org invitation cancel: update: %v", err)
		apierror.InternalError(w, "failed to cancel invitation")
		return
	}
	if tag.RowsAffected() == 0 {
		apierror.NotFound(w, "invitation not found or already resolved")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// acceptInvitation handles POST /auth/accept-invite.
// Body: { "token": "..." }
// Requires user to be authenticated (JWT). The user's email must match the invitation email.
func (h *Handler) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	callerID := auth.UserIDFromContext(r.Context())

	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Token == "" {
		apierror.BadRequest(w, "token is required")
		return
	}

	sum := sha256.Sum256([]byte(req.Token))
	hash := hex.EncodeToString(sum[:])

	// Look up invitation.
	var inv Invitation
	err := h.service.pool.QueryRow(r.Context(),
		`SELECT i.id, i.org_id, i.email, i.role, i.status, i.expires_at, o.name
		 FROM org_invitations i
		 JOIN organizations o ON o.id = i.org_id
		 WHERE i.token_hash = $1`,
		hash,
	).Scan(&inv.ID, &inv.OrgID, &inv.Email, &inv.Role, &inv.Status, &inv.ExpiresAt, &inv.OrgName)
	if err != nil {
		if err == pgx.ErrNoRows {
			apierror.NotFound(w, "invitation not found")
			return
		}
		log.Printf("org accept invitation: lookup: %v", err)
		apierror.InternalError(w, "failed to look up invitation")
		return
	}

	if inv.Status != "pending" {
		apierror.BadRequest(w, "invitation has already been used or cancelled")
		return
	}
	if time.Now().After(inv.ExpiresAt) {
		apierror.BadRequest(w, "invitation has expired")
		return
	}

	// Verify caller's email matches invitation email.
	var callerEmail string
	err = h.service.pool.QueryRow(r.Context(),
		`SELECT email FROM users WHERE id = $1`, callerID,
	).Scan(&callerEmail)
	if err != nil {
		log.Printf("org accept invitation: get caller email: %v", err)
		apierror.InternalError(w, "failed to verify user")
		return
	}
	if callerEmail != inv.Email {
		apierror.Forbidden(w, "this invitation was sent to a different email address")
		return
	}

	// Add user to org and mark invitation accepted (transaction).
	tx, err := h.service.pool.Begin(r.Context())
	if err != nil {
		log.Printf("org accept invitation: begin tx: %v", err)
		apierror.InternalError(w, "failed to process invitation")
		return
	}
	defer tx.Rollback(r.Context()) //nolint:errcheck

	_, err = tx.Exec(r.Context(),
		`INSERT INTO org_memberships (org_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		inv.OrgID, callerID, inv.Role,
	)
	if err != nil {
		log.Printf("org accept invitation: add member: %v", err)
		apierror.InternalError(w, "failed to add org membership")
		return
	}

	_, err = tx.Exec(r.Context(),
		`UPDATE org_invitations SET status = 'accepted', accepted_at = now() WHERE id = $1`,
		inv.ID,
	)
	if err != nil {
		log.Printf("org accept invitation: mark accepted: %v", err)
		apierror.InternalError(w, "failed to update invitation status")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Printf("org accept invitation: commit: %v", err)
		apierror.InternalError(w, "failed to commit invitation acceptance")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AcceptResult{Message: "joined org successfully"}) //nolint:errcheck
}

// buildInviteEmail formats the plain-text invite email body.
func buildInviteEmail(orgName, inviteURL string) string {
	return fmt.Sprintf(`You've been invited to join %s on valtag.

Click the link below to accept your invitation:

%s

This invitation expires in 7 days.

If you did not expect this invitation, you can safely ignore this email.
`, orgName, inviteURL)
}

// isDuplicateInvitation checks if the error is a unique constraint violation
// for the org+email pending index.
func isDuplicateInvitation(err error) bool {
	return err != nil && (contains(err.Error(), "idx_org_invitations_org_email_pending") ||
		contains(err.Error(), "unique constraint") ||
		contains(err.Error(), "duplicate key"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && stringContains(s, sub))
}

func stringContains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
