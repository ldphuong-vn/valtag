package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// SlackApprover is implemented by workflow.Handler to approve/reject without user auth.
type SlackApprover interface {
	ApproveBySystem(ctx context.Context, requestID, actor string) error
	RejectBySystem(ctx context.Context, requestID, actor, reason string) error
}

// SlackWebhookHandler handles Slack interactivity callbacks.
type SlackWebhookHandler struct {
	signingSecret string
	approver      SlackApprover
	slack         *SlackAdapter
}

// NewSlackWebhookHandler creates a SlackWebhookHandler.
func NewSlackWebhookHandler(signingSecret string, approver SlackApprover, slack *SlackAdapter) *SlackWebhookHandler {
	return &SlackWebhookHandler{signingSecret: signingSecret, approver: approver, slack: slack}
}

// Handle processes POST /webhooks/slack/interactions.
func (h *SlackWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "failed to read body", http.StatusBadRequest)
		return
	}
	if !h.verifySignature(r, body) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	vals, _ := url.ParseQuery(string(body))
	var payload struct {
		Actions []struct {
			ActionID string `json:"action_id"`
		} `json:"actions"`
		User    struct{ ID string `json:"id"` } `json:"user"`
		Channel struct{ ID string `json:"id"` } `json:"channel"`
		Message struct{ Ts string `json:"ts"` } `json:"message"`
	}
	_ = json.Unmarshal([]byte(vals.Get("payload")), &payload) // empty payload → no actions → handled below

	if len(payload.Actions) == 0 {
		w.WriteHeader(http.StatusOK)
		return
	}

	// action_id format: "approve:{requestID}" or "reject:{requestID}"
	actionID := payload.Actions[0].ActionID
	parts := strings.SplitN(actionID, ":", 2)
	if len(parts) != 2 {
		http.Error(w, "invalid action_id", http.StatusBadRequest)
		return
	}
	action, requestID := parts[0], parts[1]

	// Include Slack user ID in actor for audit trail
	actor := "slack:" + payload.User.ID

	var actionErr error
	switch action {
	case "approve":
		actionErr = h.approver.ApproveBySystem(r.Context(), requestID, actor)
	case "reject":
		actionErr = h.approver.RejectBySystem(r.Context(), requestID, actor, "Rejected via Slack")
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
		return
	}

	outcome := action + "d"
	if actionErr != nil {
		outcome = "failed: " + actionErr.Error()
	}
	_ = h.slack.UpdateMessage(r.Context(), payload.Channel.ID, payload.Message.Ts, outcome)
	w.WriteHeader(http.StatusOK)
}

// verifySignature validates the X-Slack-Signature header.
// Rejects requests older than 5 minutes to prevent replay attacks.
func (h *SlackWebhookHandler) verifySignature(r *http.Request, body []byte) bool {
	ts := r.Header.Get("X-Slack-Request-Timestamp")
	sig := r.Header.Get("X-Slack-Signature")
	if ts == "" || sig == "" {
		return false
	}
	tsInt, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		return false
	}
	if time.Now().Unix()-tsInt > 300 {
		return false // replay attack window
	}

	mac := hmac.New(sha256.New, []byte(h.signingSecret))
	fmt.Fprintf(mac, "v0:%s:%s", ts, body)
	expected := "v0=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(sig))
}
