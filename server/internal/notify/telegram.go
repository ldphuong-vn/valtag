package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// TelegramAdapter sends messages via Telegram Bot API (no SDK — minimal deps).
type TelegramAdapter struct {
	token  string
	client *http.Client
}

// NewTelegramAdapter creates a TelegramAdapter. Returns nil if token is empty.
func NewTelegramAdapter(token string) *TelegramAdapter {
	if token == "" {
		return nil
	}
	return &TelegramAdapter{token: token, client: &http.Client{}}
}

func (t *TelegramAdapter) apiURL(method string) string {
	return fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, method)
}

// SendApprovalRequest sends a DM with inline Approve/Reject buttons.
func (t *TelegramAdapter) SendApprovalRequest(ctx context.Context, chatID int64, requestID, secretName, requester, reason string) error {
	payload := map[string]interface{}{
		"chat_id": chatID,
		"text": fmt.Sprintf("*Access Request*\n*Secret:* %s\n*Requester:* %s\n*Reason:* %s",
			secretName, requester, reason),
		"parse_mode": "Markdown",
		"reply_markup": map[string]interface{}{
			"inline_keyboard": [][]map[string]string{
				{
					{"text": "✓ Approve", "callback_data": "approve:" + requestID},
					{"text": "✗ Reject", "callback_data": "reject:" + requestID},
				},
			},
		},
	}
	return t.post(ctx, "sendMessage", payload)
}

// EditMessage replaces the approval buttons with an outcome text.
func (t *TelegramAdapter) EditMessage(ctx context.Context, chatID int64, messageID int, text string) error {
	payload := map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
		"text":       text,
	}
	return t.post(ctx, "editMessageText", payload)
}

// AnswerCallbackQuery acknowledges a button press (clears loading state).
func (t *TelegramAdapter) AnswerCallbackQuery(ctx context.Context, callbackID, text string) error {
	payload := map[string]interface{}{"callback_query_id": callbackID, "text": text}
	return t.post(ctx, "answerCallbackQuery", payload)
}

// SetWebhook registers the Telegram webhook URL together with a secret_token:
// Telegram then presents it in the X-Telegram-Bot-Api-Secret-Token header on
// every pushed update, and the webhook handler rejects updates without it.
// Idempotent — Telegram overwrites existing webhook.
func (t *TelegramAdapter) SetWebhook(ctx context.Context, webhookURL, secretToken string) error {
	payload := map[string]interface{}{
		"url":             webhookURL,
		"allowed_updates": []string{"message", "callback_query"},
	}
	if secretToken != "" {
		payload["secret_token"] = secretToken
	}
	return t.post(ctx, "setWebhook", payload)
}

func (t *TelegramAdapter) post(ctx context.Context, method string, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", t.apiURL(method), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building telegram request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := t.client.Do(req)
	if err != nil {
		return fmt.Errorf("telegram api: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result) // decode failure leaves OK=false → error path below
	if !result.OK {
		return fmt.Errorf("telegram error: %s", result.Description)
	}
	return nil
}
