package auth

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"

	"github.com/pquerna/otp/totp"

	"github.com/valt-dev/valt/server/pkg/apierror"
)

// TOTPHandler manages TOTP 2FA endpoints.
type TOTPHandler struct {
	store  *TOTPStore
	jwtMgr *JWTManager
}

// NewTOTPHandler creates a TOTPHandler.
func NewTOTPHandler(store *TOTPStore, jwtMgr *JWTManager) *TOTPHandler {
	return &TOTPHandler{store: store, jwtMgr: jwtMgr}
}

// Setup generates a TOTP secret for the user. POST /me/totp/setup
func (h *TOTPHandler) Setup(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	// Check if already enabled
	_, enabled, err := h.store.GetTOTPSecret(r.Context(), userID)
	if err == nil && enabled {
		apierror.BadRequest(w, "2FA is already enabled; disable first")
		return
	}

	// Get user email for TOTP account name
	var email string
	if scanErr := h.store.pool.QueryRow(r.Context(),
		`SELECT email FROM users WHERE id = $1`, userID,
	).Scan(&email); scanErr != nil {
		apierror.InternalError(w, "failed to load user for 2FA setup")
		return
	}

	key, err := totp.Generate(totp.GenerateOpts{
		Issuer:      "Valt",
		AccountName: email,
	})
	if err != nil {
		apierror.InternalError(w, "failed to generate TOTP secret")
		return
	}

	if err := h.store.SetSetupSecret(r.Context(), userID, key.Secret()); err != nil {
		apierror.InternalError(w, "failed to save setup secret")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{ // status not yet written; Encode failure falls back to empty 200
		"secret": key.Secret(),
		"qr_uri": key.URL(),
	})
}

// Verify validates the TOTP code and enables 2FA. POST /me/totp/verify
func (h *TOTPHandler) Verify(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		apierror.BadRequest(w, "code is required")
		return
	}

	secret, err := h.store.GetSetupSecret(r.Context(), userID)
	if err != nil {
		apierror.BadRequest(w, "no pending TOTP setup; call /me/totp/setup first")
		return
	}

	if !totp.Validate(req.Code, secret) {
		apierror.Unauthorized(w, "invalid TOTP code")
		return
	}

	if err := h.store.EnableTOTP(r.Context(), userID, secret); err != nil {
		apierror.InternalError(w, "failed to enable 2FA")
		return
	}

	// Generate backup codes
	codes := generateBackupCodes(10)
	if err := h.store.StoreBackupCodes(r.Context(), userID, codes); err != nil {
		apierror.InternalError(w, "failed to store backup codes")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{ // status not yet written; Encode failure falls back to empty 200
		"enabled":      true,
		"backup_codes": codes,
	})
}

// Disable disables 2FA. Requires current TOTP code. DELETE /me/totp
func (h *TOTPHandler) Disable(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Code == "" {
		apierror.BadRequest(w, "current TOTP code is required to disable 2FA")
		return
	}

	secret, enabled, err := h.store.GetTOTPSecret(r.Context(), userID)
	if err != nil || !enabled {
		apierror.BadRequest(w, "2FA is not enabled")
		return
	}

	if !totp.Validate(req.Code, secret) {
		apierror.Unauthorized(w, "invalid TOTP code")
		return
	}

	if err := h.store.DisableTOTP(r.Context(), userID); err != nil {
		apierror.InternalError(w, "failed to disable 2FA")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RegenerateBackup creates new backup codes. POST /me/totp/backup-codes
func (h *TOTPHandler) RegenerateBackup(w http.ResponseWriter, r *http.Request) {
	userID := UserIDFromContext(r.Context())

	_, enabled, err := h.store.GetTOTPSecret(r.Context(), userID)
	if err != nil || !enabled {
		apierror.BadRequest(w, "2FA is not enabled")
		return
	}

	codes := generateBackupCodes(10)
	if err := h.store.StoreBackupCodes(r.Context(), userID, codes); err != nil {
		apierror.InternalError(w, "failed to regenerate backup codes")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"backup_codes": codes}) // status not yet written; Encode failure falls back to empty 200
}

// Validate checks a TOTP code during login. POST /auth/totp/validate
func (h *TOTPHandler) Validate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		TOTPToken string `json:"totp_token"`
		Code      string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.TOTPToken == "" || req.Code == "" {
		apierror.BadRequest(w, "totp_token and code are required")
		return
	}

	userID, err := h.jwtMgr.ValidateTOTPToken(req.TOTPToken)
	if err != nil {
		apierror.Unauthorized(w, "invalid or expired TOTP token")
		return
	}

	secret, enabled, err := h.store.GetTOTPSecret(r.Context(), userID)
	if err != nil || !enabled {
		apierror.Unauthorized(w, "2FA is not enabled for this account")
		return
	}

	// Try TOTP code
	valid := totp.Validate(req.Code, secret)

	// If TOTP fails, try backup code
	if !valid {
		used, bcErr := h.store.UseBackupCode(r.Context(), userID, req.Code)
		if bcErr != nil || !used {
			apierror.Unauthorized(w, "invalid TOTP or backup code")
			return
		}
	}

	// Issue normal JWT tokens
	accessToken, err := h.jwtMgr.GenerateAccessToken(userID)
	if err != nil {
		apierror.InternalError(w, "failed to generate tokens")
		return
	}
	refreshToken, err := h.jwtMgr.GenerateRefreshToken()
	if err != nil {
		apierror.InternalError(w, "failed to generate tokens")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(authResponse{ // status not yet written; Encode failure falls back to empty 200
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    900,
	})
}

// generateBackupCodes creates n random 8-char hex codes.
func generateBackupCodes(n int) []string {
	codes := make([]string, n)
	for i := range codes {
		b := make([]byte, 4)
		rand.Read(b)
		codes[i] = hex.EncodeToString(b)
	}
	return codes
}
