package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// Dispatcher sends webhook events to registered endpoints.
type Dispatcher struct {
	store  *Store
	client *http.Client
}

// NewDispatcher creates a webhook Dispatcher.
func NewDispatcher(store *Store) *Dispatcher {
	return &Dispatcher{
		store:  store,
		client: &http.Client{Timeout: 5 * time.Second},
	}
}

// EventPayload is the structure sent to webhook endpoints.
type EventPayload struct {
	Event     string      `json:"event"`
	Timestamp string      `json:"timestamp"`
	Data      interface{} `json:"data"`
}

// Dispatch sends an event to all active webhooks for an org. Fire-and-forget.
func (d *Dispatcher) Dispatch(ctx context.Context, orgID, event string, data interface{}) {
	hooks, err := d.store.ListActive(ctx, orgID, event)
	if err != nil {
		log.Printf("[webhook-dispatch] list error: %v", err)
		return
	}

	payload := EventPayload{
		Event:     event,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		log.Printf("[webhook-dispatch] marshal error: %v", err)
		return
	}

	for _, hook := range hooks {
		go d.deliver(hook, body)
	}
}

func (d *Dispatcher) deliver(hook Webhook, body []byte) {
	// Compute HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(hook.SecretKey))
	mac.Write(body)
	signature := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	deliveryID := uuid.NewString()

	req, err := http.NewRequest("POST", hook.URL, bytes.NewReader(body))
	if err != nil {
		log.Printf("[webhook-dispatch] build request error for %s: %v", hook.URL, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-valtag-Signature", signature)
	req.Header.Set("X-valtag-Event", hook.Events[0])
	req.Header.Set("X-valtag-Delivery", deliveryID)

	resp, err := d.client.Do(req)
	if err != nil {
		log.Printf("[webhook-dispatch] delivery %s to %s failed: %v", deliveryID, hook.URL, err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[webhook-dispatch] delivery %s to %s: status %d", deliveryID, hook.URL, resp.StatusCode)
}
