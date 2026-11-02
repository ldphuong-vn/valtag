package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// ZaloAdapter sends one-way notifications via Zalo OA API.
// Zalo ZNS is one-way only — no interactive approve/reject buttons.
type ZaloAdapter struct {
	oaToken string
	oaID    string
	client  *http.Client
}

// NewZaloAdapter creates a ZaloAdapter. Returns nil if token is empty.
func NewZaloAdapter(oaToken, oaID string) *ZaloAdapter {
	if oaToken == "" {
		return nil
	}
	return &ZaloAdapter{oaToken: oaToken, oaID: oaID, client: &http.Client{}}
}

// SendNotification sends a one-way text message to a Zalo user.
// userID here is the Zalo user_id obtained during linking.
func (z *ZaloAdapter) SendNotification(ctx context.Context, zaloUserID, message string) error {
	payload := map[string]interface{}{
		"recipient": map[string]string{"user_id": zaloUserID},
		"message":   map[string]string{"text": message},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, "POST", "https://openapi.zalo.me/v3.0/oa/message/cs", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building zalo request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("access_token", z.oaToken)

	resp, err := z.client.Do(req)
	if err != nil {
		return fmt.Errorf("zalo api: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Error   int    `json:"error"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result) // decode failure leaves Error=0 → treated as success
	if result.Error != 0 {
		return fmt.Errorf("zalo error %d: %s", result.Error, result.Message)
	}
	return nil
}
