package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// acceptInviteForNewUser joins a freshly-registered user to the org that
// invited their email (invite-only deployments, 2026-11-21). The invitation
// must be pending, unexpired, and addressed to the registering email.
// Returns true when the invitation was valid and the membership created.
func (h *Handler) acceptInviteForNewUser(ctx context.Context, token, userID, email string) (bool, error) {
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])

	var invID, orgID, invEmail, invRole, invStatus string
	var invExpires time.Time
	err := h.pool.QueryRow(ctx,
		`SELECT i.id, i.org_id, i.email, i.role, i.status, i.expires_at
		 FROM org_invitations i WHERE i.token_hash = $1`,
		hash,
	).Scan(&invID, &orgID, &invEmail, &invRole, &invStatus, &invExpires)
	if err != nil {
		return false, err
	}
	if invStatus != "pending" || time.Now().After(invExpires) {
		return false, fmt.Errorf("invitation expired or already used")
	}
	if !strings.EqualFold(invEmail, email) {
		return false, fmt.Errorf("invitation is for a different email address")
	}

	tx, err := h.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	if _, err = tx.Exec(ctx,
		`INSERT INTO org_memberships (org_id, user_id, role)
		 VALUES ($1, $2, $3)
		 ON CONFLICT (org_id, user_id) DO UPDATE SET role = EXCLUDED.role`,
		orgID, userID, invRole,
	); err != nil {
		return false, err
	}

	if _, err = tx.Exec(ctx,
		`UPDATE org_invitations SET status = 'accepted', accepted_at = now() WHERE id = $1`,
		invID,
	); err != nil {
		return false, err
	}

	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}
