package notify

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// telegramSecretHeader is the header Telegram uses to present the secret_token
// registered via SetWebhook on every webhook update it pushes.
const telegramSecretHeader = "X-Telegram-Bot-Api-Secret-Token"

// TelegramWebhookHandler handles Telegram Update callbacks.
type TelegramWebhookHandler struct {
	telegram      *TelegramAdapter
	channels      *ChannelStore
	pool          *pgxpool.Pool
	approver      SlackApprover // reuse same interface — ApproveBySystem/RejectBySystem
	botUsername   string
	webhookSecret string
}

// NewTelegramWebhookHandler creates a TelegramWebhookHandler. webhookSecret is
// the value sent as secret_token in SetWebhook; Handle rejects updates that
// fail to present it.
func NewTelegramWebhookHandler(telegram *TelegramAdapter, channels *ChannelStore, pool *pgxpool.Pool, approver SlackApprover, botUsername, webhookSecret string) *TelegramWebhookHandler {
	return &TelegramWebhookHandler{
		telegram:      telegram,
		channels:      channels,
		pool:          pool,
		approver:      approver,
		botUsername:   botUsername,
		webhookSecret: webhookSecret,
	}
}

// telegramUpdate is the minimal Telegram Update payload we care about.
type telegramUpdate struct {
	Message *struct {
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
	CallbackQuery *struct {
		ID   string `json:"id"`
		Data string `json:"data"`
		Message struct {
			MessageID int `json:"message_id"`
			Chat      struct {
				ID int64 `json:"id"`
			} `json:"chat"`
		} `json:"message"`
	} `json:"callback_query"`
}

// Handle processes POST /api/v1/webhooks/telegram. Every update must present
// the webhook secret registered via SetWebhook; unauthenticated updates are
// rejected before any processing (fail-closed).
func (h *TelegramWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if !h.verifySecret(r) {
		http.Error(w, "invalid secret", http.StatusUnauthorized)
		return
	}

	var update telegramUpdate
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusOK) // always 200 to Telegram
		return
	}

	// /start {link_token} — link user's Telegram account
	if update.Message != nil && strings.HasPrefix(update.Message.Text, "/start ") {
		linkToken := strings.TrimPrefix(update.Message.Text, "/start ")
		h.handleLinkToken(r.Context(), strings.TrimSpace(linkToken), update.Message.Chat.ID)
	}

	// Callback from Approve/Reject inline button
	if update.CallbackQuery != nil {
		cq := update.CallbackQuery
		parts := strings.SplitN(cq.Data, ":", 2)
		if len(parts) == 2 {
			action, requestID := parts[0], parts[1]
			var outcome string
			switch action {
			case "approve":
				if err := h.approver.ApproveBySystem(r.Context(), requestID, "telegram-action"); err != nil {
					outcome = "Failed: " + err.Error()
				} else {
					outcome = "✓ Approved"
				}
			case "reject":
				if err := h.approver.RejectBySystem(r.Context(), requestID, "telegram-action", "Rejected via Telegram"); err != nil {
					outcome = "Failed: " + err.Error()
				} else {
					outcome = "✗ Rejected"
				}
			default:
				outcome = "Unknown action"
			}
			_ = h.telegram.AnswerCallbackQuery(r.Context(), cq.ID, outcome)
			_ = h.telegram.EditMessage(r.Context(), cq.Message.Chat.ID, cq.Message.MessageID, outcome)
		}
	}

	w.WriteHeader(http.StatusOK)
}

// verifySecret compares the presented secret against the configured value in
// constant time. Fails closed when no secret is configured.
func (h *TelegramWebhookHandler) verifySecret(r *http.Request) bool {
	if h.webhookSecret == "" {
		return false
	}
	presented := r.Header.Get(telegramSecretHeader)
	return subtle.ConstantTimeCompare([]byte(presented), []byte(h.webhookSecret)) == 1
}

// handleLinkToken validates the link token and upserts the Telegram channel for the user.
func (h *TelegramWebhookHandler) handleLinkToken(ctx context.Context, token string, chatID int64) {
	var userID string
	err := h.pool.QueryRow(ctx,
		`UPDATE telegram_link_tokens SET used_at = NOW()
		 WHERE token = $1 AND used_at IS NULL AND expires_at > NOW()
		 RETURNING user_id`,
		token,
	).Scan(&userID)
	if err != nil {
		return // invalid or expired token — silently ignore
	}
	_, _ = h.channels.Upsert(ctx, userID, "telegram", fmt.Sprintf("%d", chatID))
	// Mark the channel as verified immediately since the user proved they own this chat
	_, _ = h.pool.Exec(ctx,
		`UPDATE user_notification_channels SET verified = true WHERE user_id = $1 AND channel_type = 'telegram'`,
		userID,
	)
}

// GenerateLinkURL creates a short-lived token and returns the Telegram deep link URL.
func (h *TelegramWebhookHandler) GenerateLinkURL(ctx context.Context, userID string) (string, error) {
	raw := make([]byte, 32)
	if _, randErr := rand.Read(raw); randErr != nil {
		return "", fmt.Errorf("generating link token: %w", randErr)
	}
	token := hex.EncodeToString(raw)
	if _, dbErr := h.pool.Exec(ctx,
		`INSERT INTO telegram_link_tokens (token, user_id, expires_at) VALUES ($1, $2, NOW() + INTERVAL '10 minutes')`,
		token, userID,
	); dbErr != nil {
		return "", fmt.Errorf("storing telegram link token: %w", dbErr)
	}
	return fmt.Sprintf("https://t.me/%s?start=%s", h.botUsername, token), nil
}
