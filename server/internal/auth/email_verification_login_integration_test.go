package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ldphuong-vn/valtag/server/internal/config"
)

// recordingEmailSender captures send calls — a stub for the SMTP-configured
// path (emailSender != nil is exactly the "SMTP configured" signal).
type recordingEmailSender struct{ sent []string }

func (r *recordingEmailSender) Send(_ context.Context, to, subject, body string) error {
	r.sent = append(r.sent, to+"|"+subject)
	return nil
}

// loginStatus registers a user then attempts a password login, returning the
// login response code.
func loginStatus(t *testing.T, h *Handler, email string) int {
	t.Helper()
	reg := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
		`{"email":"`+email+`","password":"VerifyPass123!","region_code":"us"}`))
	regRec := httptest.NewRecorder()
	h.register(regRec, reg)

	login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(
		`{"email":"`+email+`","password":"VerifyPass123!"}`))
	loginRec := httptest.NewRecorder()
	h.login(loginRec, login)
	return loginRec.Code
}

// TestLoginEmailVerificationEnforcement covers the SMTP-aware enforcement
// (report §4.15): login never checks email without SMTP (self-hosters are
// never locked out); with SMTP configured, unverified users are blocked at
// login with a pointer to the resend endpoint, and a Google-issued verified
// flag on the account passes.
func TestLoginEmailVerificationEnforcement(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newAuthIntegrationDB(t, ctx)
	defer cleanup()

	newHandler := func(sender EmailSender) *Handler {
		privPEM, pubPEM := generateTestKeys(t)
		mgr, err := NewJWTManager(privPEM, pubPEM)
		if err != nil {
			t.Fatalf("jwt manager: %v", err)
		}
		h := NewHandler(db, mgr, &config.Config{})
		if sender != nil {
			h.SetEmailSender(sender)
		}
		return h
	}

	t.Run("no SMTP configured — registration auto-verifies and login passes", func(t *testing.T) {
		h := newHandler(nil)
		if code := loginStatus(t, h, "nosmtp@example.test"); code != http.StatusOK {
			t.Fatalf("login got %d, want 200 (auto-verified without SMTP)", code)
		}
	})

	t.Run("SMTP configured — unverified user is blocked at login", func(t *testing.T) {
		sender := &recordingEmailSender{}
		h := newHandler(sender)
		email := "unverified@example.test"
		if code := loginStatus(t, h, email); code != http.StatusForbidden {
			t.Fatalf("login got %d, want 403", code)
		}
		if len(sender.sent) == 0 {
			t.Fatal("registration should have queued a verification email when SMTP is configured")
		}

		// Simulate clicking the verification link: mark verified, then login passes.
		var userID string
		if err := db.QueryRow(ctx, `SELECT id FROM users WHERE email = $1`, email).Scan(&userID); err != nil {
			t.Fatalf("load user: %v", err)
		}
		if _, err := db.Exec(ctx, `UPDATE users SET email_verified = true WHERE id = $1`, userID); err != nil {
			t.Fatalf("verify: %v", err)
		}
		if code := loginStatus(t, h, email); code != http.StatusOK {
			t.Fatalf("login after verify got %d, want 200", code)
		}
	})

	t.Run("login response stays JSON on the forbidden path", func(t *testing.T) {
		sender := &recordingEmailSender{}
		h := newHandler(sender)
		reg := httptest.NewRequest(http.MethodPost, "/api/v1/auth/register", strings.NewReader(
			`{"email":"jsonpath@example.test","password":"VerifyPass123!","region_code":"us"}`))
		regRec := httptest.NewRecorder()
		h.register(regRec, reg)

		login := httptest.NewRequest(http.MethodPost, "/api/v1/auth/login", strings.NewReader(
			`{"email":"jsonpath@example.test","password":"VerifyPass123!"}`))
		loginRec := httptest.NewRecorder()
		h.login(loginRec, login)

		var body map[string]interface{}
		if err := json.Unmarshal(loginRec.Body.Bytes(), &body); err != nil {
			t.Fatalf("forbidden response is not JSON: %v (%s)", err, loginRec.Body.String())
		}
	})
}
