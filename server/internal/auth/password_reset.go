package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
	"github.com/ldphuong-vn/valtag/server/pkg/validator"
)

type forgotPasswordRequest struct {
	Email string `json:"email"`
}

type resetPasswordRequest struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

// forgotPassword handles POST /auth/forgot-password
// Always returns 200 to prevent email enumeration.
func (h *Handler) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var req forgotPasswordRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	// Always return 200 immediately (don't leak whether email exists).
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "If the email exists, a reset link has been sent"}) //nolint:errcheck

	if req.Email == "" {
		return
	}

	// Background processing: check user, generate token, send email.
	// Use context.Background() since the HTTP request context will be cancelled after response.
	go func() {
		if err := h.processForgotPassword(context.Background(), req.Email); err != nil {
			log.Printf("auth: forgot-password processing error for %s: %v", req.Email, err)
		}
	}()
}

// processForgotPassword performs token creation and email sending.
// Silently returns nil if user does not exist or is OAuth-only (no password).
func (h *Handler) processForgotPassword(ctx context.Context, email string) error {
	var userID string
	err := h.pool.QueryRow(ctx,
		`SELECT id FROM users WHERE email = $1 AND password_hash != '' AND password_hash IS NOT NULL`,
		email,
	).Scan(&userID)
	if err != nil {
		// User not found or OAuth-only — silently succeed.
		return nil
	}

	raw, hash, err := generateToken()
	if err != nil {
		return fmt.Errorf("generating reset token: %w", err)
	}

	_, err = h.pool.Exec(ctx,
		`INSERT INTO password_reset_tokens (user_id, token_hash, expires_at)
		 VALUES ($1, $2, $3)`,
		userID, hash, time.Now().Add(1*time.Hour),
	)
	if err != nil {
		return fmt.Errorf("storing reset token: %w", err)
	}

	if h.emailSender == nil {
		log.Printf("auth: SMTP not configured — skipping password reset email for %s (token created)", email)
		return nil
	}

	resetURL := fmt.Sprintf("%s/reset-password?token=%s", h.dashboardURL, raw)
	body := fmt.Sprintf(
		"You requested a password reset for your valtag account.\n\nClick the link below to reset your password:\n\n%s\n\nThis link expires in 1 hour and can only be used once.\n\nIf you did not request a password reset, you can ignore this email.",
		resetURL,
	)
	return h.emailSender.Send(ctx, email, "Reset your valtag password", body)
}

// resetPassword handles POST /auth/reset-password
// Validates token, updates password, invalidates all refresh tokens.
func (h *Handler) resetPassword(w http.ResponseWriter, r *http.Request) {
	var req resetPasswordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	if req.Token == "" {
		apierror.BadRequest(w, "token is required")
		return
	}

	if err := validator.ValidatePassword(req.Password); err != nil {
		apierror.BadRequest(w, err.Error())
		return
	}

	tokenHash := hashToken(req.Token)

	var userID string
	var expiresAt time.Time
	var used bool
	err := h.pool.QueryRow(r.Context(),
		`SELECT user_id, expires_at, used FROM password_reset_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&userID, &expiresAt, &used)
	if err != nil {
		apierror.BadRequest(w, "invalid or expired token")
		return
	}

	if used || time.Now().After(expiresAt) {
		apierror.BadRequest(w, "invalid or expired token")
		return
	}

	newHash, err := HashPassword(req.Password)
	if err != nil {
		log.Printf("auth: hash password on reset: %v", err)
		apierror.InternalError(w, "failed to reset password")
		return
	}

	_, err = h.pool.Exec(r.Context(),
		`UPDATE users SET password_hash = $1 WHERE id = $2`, newHash, userID,
	)
	if err != nil {
		log.Printf("auth: update password: %v", err)
		apierror.InternalError(w, "failed to reset password")
		return
	}

	// Mark token as used (kept for audit trail, not deleted).
	_, _ = h.pool.Exec(r.Context(),
		`UPDATE password_reset_tokens SET used = true WHERE token_hash = $1`, tokenHash,
	)

	// Invalidate all active refresh tokens (security: force re-login on all devices).
	_, _ = h.pool.Exec(r.Context(), `DELETE FROM refresh_tokens WHERE user_id = $1`, userID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Password reset successful"}) //nolint:errcheck
}
