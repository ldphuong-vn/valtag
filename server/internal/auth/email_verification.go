package auth

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

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// generateToken returns a random 32-byte hex string and its SHA-256 hash.
func generateToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generating token: %w", err)
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

// hashToken returns SHA-256 hex of raw token (for lookup).
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// sendVerificationEmail inserts a token and sends the verification link.
// If emailSender is nil (SMTP not configured), logs a warning but still creates the token.
func (h *Handler) sendVerificationEmail(ctx context.Context, userID, email string) error {
	raw, hash, err := generateToken()
	if err != nil {
		return err
	}

	// Delete any old tokens for this user before inserting a new one.
	_, _ = h.pool.Exec(ctx, `DELETE FROM email_verification_tokens WHERE user_id = $1`, userID)

	_, err = h.pool.Exec(ctx,
		`INSERT INTO email_verification_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, hash, time.Now().Add(24*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("storing verification token: %w", err)
	}

	if h.emailSender == nil {
		log.Printf("auth: SMTP not configured — skipping verification email for %s (token created)", email)
		return nil
	}

	verifyURL := fmt.Sprintf("%s/api/v1/auth/verify-email?token=%s", h.dashboardURL, raw)
	body := fmt.Sprintf(
		"Welcome to valtag!\n\nPlease verify your email address by clicking the link below:\n\n%s\n\nThis link expires in 24 hours.\n\nIf you did not create a valtag account, you can ignore this email.",
		verifyURL,
	)
	return h.emailSender.Send(ctx, email, "Verify your valtag email", body)
}

// verifyEmail handles GET /auth/verify-email?token=X
// Validates the token, sets email_verified=true, redirects to dashboard.
func (h *Handler) verifyEmail(w http.ResponseWriter, r *http.Request) {
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		apierror.BadRequest(w, "token is required")
		return
	}

	tokenHash := hashToken(rawToken)

	var userID string
	var expiresAt time.Time
	err := h.pool.QueryRow(r.Context(),
		`SELECT user_id, expires_at FROM email_verification_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&userID, &expiresAt)
	if err != nil {
		apierror.BadRequest(w, "invalid or expired token")
		return
	}

	if time.Now().After(expiresAt) {
		apierror.BadRequest(w, "invalid or expired token")
		return
	}

	// Mark verified and delete token (single-use).
	_, _ = h.pool.Exec(r.Context(), `UPDATE users SET email_verified = true WHERE id = $1`, userID)
	_, _ = h.pool.Exec(r.Context(), `DELETE FROM email_verification_tokens WHERE token_hash = $1`, tokenHash)

	http.Redirect(w, r, h.dashboardURL+"/?verified=true", http.StatusFound)
}

// resendVerification handles POST /auth/resend-verification (authenticated).
// Rate-limited to once per 5 minutes per user.
func (h *Handler) resendVerification(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		apierror.Unauthorized(w, "authentication required")
		return
	}

	// Check rate limit: was a token sent in the last 5 minutes?
	var lastSent time.Time
	err := h.pool.QueryRow(r.Context(),
		`SELECT created_at FROM email_verification_tokens WHERE user_id = $1 ORDER BY created_at DESC LIMIT 1`,
		userID,
	).Scan(&lastSent)
	if err == nil && time.Since(lastSent) < 5*time.Minute {
		apierror.BadRequest(w, "please wait before requesting another verification email")
		return
	}

	// Check if already verified.
	var email string
	var verified bool
	err = h.pool.QueryRow(r.Context(),
		`SELECT email, email_verified FROM users WHERE id = $1`, userID,
	).Scan(&email, &verified)
	if err != nil {
		apierror.InternalError(w, "failed to load user")
		return
	}
	if verified {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "email already verified"}) //nolint:errcheck
		return
	}

	if err := h.sendVerificationEmail(r.Context(), userID, email); err != nil {
		log.Printf("auth: resend verification: %v", err)
		apierror.InternalError(w, "failed to send verification email")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "verification email sent"}) //nolint:errcheck
}

// getMe handles GET /auth/me — returns current user profile including email_verified.
func (h *Handler) getMe(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())
	if userID == "" {
		apierror.Unauthorized(w, "authentication required")
		return
	}

	var email string
	var emailVerified bool
	var createdAt time.Time
	var role string
	err := h.pool.QueryRow(r.Context(),
		`SELECT email, email_verified, created_at, role FROM users WHERE id = $1`, userID,
	).Scan(&email, &emailVerified, &createdAt, &role)
	if err != nil {
		apierror.InternalError(w, "failed to load user")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{ //nolint:errcheck
		"id":             userID,
		"email":          email,
		"email_verified": emailVerified,
		"created_at":     createdAt,
		"role":           role,
	})
}
