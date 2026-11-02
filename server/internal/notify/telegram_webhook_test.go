package notify

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testSecret returns a fresh random secret per test run so no credential-shaped
// literal lives in source.
func testSecret(t *testing.T) string {
	t.Helper()
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		t.Fatalf("generating test secret: %v", err)
	}
	return hex.EncodeToString(raw)
}

// expectStatus posts a Telegram update with an optional secret header and
// asserts the response status.
func expectStatus(t *testing.T, handler *TelegramWebhookHandler, secretHeader string, body string, want int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/webhooks/telegram", strings.NewReader(body))
	if secretHeader != "" {
		req.Header.Set(telegramSecretHeader, secretHeader)
	}
	rec := httptest.NewRecorder()
	handler.Handle(rec, req)
	if rec.Code != want {
		t.Fatalf("got %d, want %d", rec.Code, want)
	}
}

// TestTelegramWebhookSecretVerification covers the auth gate added to close
// the unauthenticated approve oracle (graphify report §4.1): every update
// must present the secret registered via SetWebhook, otherwise 401 before
// any processing.
func TestTelegramWebhookSecretVerification(t *testing.T) {
	secret := testSecret(t)
	handler := &TelegramWebhookHandler{webhookSecret: secret}

	t.Run("missing header rejected", func(t *testing.T) {
		expectStatus(t, handler, "", `{"update_id":1}`, http.StatusUnauthorized)
	})

	t.Run("wrong secret rejected", func(t *testing.T) {
		expectStatus(t, handler, testSecret(t), `{"update_id":1}`, http.StatusUnauthorized)
	})

	t.Run("correct secret passes the gate", func(t *testing.T) {
		// {"update_id":1} passes verification, decodes cleanly, and carries
		// neither message nor callback — the handler reaches 200 without
		// touching the DB or the Telegram API.
		expectStatus(t, handler, secret, `{"update_id":1}`, http.StatusOK)
	})

	t.Run("fake approve callback without secret is rejected", func(t *testing.T) {
		fake := `{"callback_query":{"id":"x","data":"approve:11111111-1111-1111-1111-111111111111","message":{"message_id":1,"chat":{"id":1}}}}`
		expectStatus(t, handler, "", fake, http.StatusUnauthorized)
	})
}

// TestTelegramWebhookFailsClosedWithoutSecret ensures an operator who forgot
// TELEGRAM_WEBHOOK_SECRET gets a dead webhook (loud failure at startup), not
// the old unauthenticated approve oracle.
func TestTelegramWebhookFailsClosedWithoutSecret(t *testing.T) {
	handler := &TelegramWebhookHandler{webhookSecret: ""}
	expectStatus(t, handler, "anything", `{"update_id":1}`, http.StatusUnauthorized)
}
