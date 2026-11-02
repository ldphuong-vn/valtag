// Package valtag provides a Go client for the valtag secret vault API.
package valtag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client is the valtag API client.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// New creates a new valtag client.
func New(baseURL, token string) *Client {
	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Secret represents a vault secret.
type Secret struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	CredentialType string    `json:"credential_type"`
	Status         string    `json:"status"`
	CreatedAt      time.Time `json:"created_at"`
}

// Credential represents an approved credential.
type Credential struct {
	ID        string    `json:"id"`
	RequestID string    `json:"request_id"`
	ExpiresAt time.Time `json:"expires_at"`
	Status    string    `json:"status"`
}

// AccessRequest represents the response from requesting access to a secret.
type AccessRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ListSecrets returns all secrets visible to the authenticated caller.
func (c *Client) ListSecrets(ctx context.Context) ([]Secret, error) {
	var result struct {
		Secrets []Secret `json:"secrets"`
	}
	if err := c.get(ctx, "/secrets", &result); err != nil {
		return nil, err
	}
	return result.Secrets, nil
}

// RequestAccess requests temporary access to a secret.
// Returns the access request ID.
func (c *Client) RequestAccess(ctx context.Context, secretID, reason string, durationMinutes int) (string, error) {
	body := map[string]interface{}{
		"reason":           reason,
		"duration_minutes": durationMinutes,
	}
	var result AccessRequest
	if err := c.post(ctx, fmt.Sprintf("/secrets/%s/access-requests", secretID), body, &result); err != nil {
		return "", err
	}
	return result.ID, nil
}

// GetCredential retrieves an approved credential by its request ID.
func (c *Client) GetCredential(ctx context.Context, requestID string) (*Credential, error) {
	var cred Credential
	if err := c.get(ctx, fmt.Sprintf("/credentials/%s", requestID), &cred); err != nil {
		return nil, err
	}
	return &cred, nil
}

func (c *Client) get(ctx context.Context, path string, out interface{}) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	return c.do(req, out)
}

func (c *Client) post(ctx context.Context, path string, body interface{}, out interface{}) error {
	data, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, out)
}

func (c *Client) do(req *http.Request, out interface{}) error {
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("valtag: API error %d: %s", resp.StatusCode, string(raw))
	}
	if out != nil {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("valtag: decoding response: %w", err)
		}
	}
	return nil
}
