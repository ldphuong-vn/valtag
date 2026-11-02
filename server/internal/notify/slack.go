package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// SlackAdapter sends messages via Slack Bot API (no SDK — minimal deps).
// Token is passed per-call to support per-org OAuth tokens.
type SlackAdapter struct {
	fallbackToken string
	client        *http.Client
}

// NewSlackAdapter creates a SlackAdapter. fallbackToken is used when no per-org token is available.
func NewSlackAdapter(fallbackToken string) *SlackAdapter {
	return &SlackAdapter{fallbackToken: fallbackToken, client: &http.Client{}}
}

// FallbackToken returns the global fallback token (from env var).
func (s *SlackAdapter) FallbackToken() string {
	return s.fallbackToken
}

type slackText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type slackElement struct {
	Type     string     `json:"type"`
	Text     *slackText `json:"text"`
	ActionID string     `json:"action_id"`
	Style    string     `json:"style,omitempty"`
}

type slackBlock struct {
	Type     string         `json:"type"`
	Text     *slackText     `json:"text,omitempty"`
	Elements []slackElement `json:"elements,omitempty"`
}

// SendApprovalRequest sends a Block Kit DM with Approve/Reject buttons.
// Uses token if non-empty, otherwise falls back to fallbackToken.
func (s *SlackAdapter) SendApprovalRequest(ctx context.Context, slackUserID, requestID, secretName, requester, reason string) error {
	return s.SendApprovalRequestWithToken(ctx, s.fallbackToken, slackUserID, requestID, secretName, requester, reason)
}

// SendApprovalRequestWithToken sends a Block Kit DM using a specific bot token.
func (s *SlackAdapter) SendApprovalRequestWithToken(ctx context.Context, token, slackUserID, requestID, secretName, requester, reason string) error {
	if token == "" {
		return fmt.Errorf("no slack token available")
	}
	payload := map[string]interface{}{
		"channel": slackUserID,
		"text":    fmt.Sprintf("Approval needed: %s requests access to %s", requester, secretName),
		"blocks": []slackBlock{
			{
				Type: "section",
				Text: &slackText{Type: "mrkdwn", Text: fmt.Sprintf(
					"*Access Request*\n*Secret:* %s\n*Requester:* %s\n*Reason:* %s",
					secretName, requester, reason,
				)},
			},
			{
				Type: "actions",
				Elements: []slackElement{
					{
						Type:     "button",
						ActionID: "approve:" + requestID,
						Text:     &slackText{Type: "plain_text", Text: "✓ Approve"},
						Style:    "primary",
					},
					{
						Type:     "button",
						ActionID: "reject:" + requestID,
						Text:     &slackText{Type: "plain_text", Text: "✗ Reject"},
						Style:    "danger",
					},
				},
			},
		},
	}
	return s.post(ctx, token, "https://slack.com/api/chat.postMessage", payload)
}

// UpdateMessage replaces the approval buttons with an outcome message.
func (s *SlackAdapter) UpdateMessage(ctx context.Context, channelID, ts, outcome string) error {
	return s.UpdateMessageWithToken(ctx, s.fallbackToken, channelID, ts, outcome)
}

// UpdateMessageWithToken replaces approval buttons using a specific token.
func (s *SlackAdapter) UpdateMessageWithToken(ctx context.Context, token, channelID, ts, outcome string) error {
	if token == "" {
		return fmt.Errorf("no slack token available")
	}
	payload := map[string]interface{}{
		"channel": channelID,
		"ts":      ts,
		"text":    "Request " + outcome + ".",
		"blocks": []slackBlock{
			{Type: "section", Text: &slackText{Type: "mrkdwn", Text: "*Request " + outcome + ".* (actioned via Slack)"}},
		},
	}
	return s.post(ctx, token, "https://slack.com/api/chat.update", payload)
}

func (s *SlackAdapter) post(ctx context.Context, token, url string, payload interface{}) error {
	body, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building slack request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("slack api: %w", err)
	}
	defer resp.Body.Close()
	var result struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result) // decode failure leaves OK=false → error path below
	if !result.OK {
		return fmt.Errorf("slack error: %s", result.Error)
	}
	return nil
}
