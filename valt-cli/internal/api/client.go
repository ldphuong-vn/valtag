package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Client talks to the valtag REST API.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func New(baseURL, token string) *Client {
	return &Client{baseURL: baseURL, token: token, http: &http.Client{}}
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}, out interface{}) error {
	var bodyReader *bytes.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		bodyReader = bytes.NewReader(b)
	} else {
		bodyReader = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+"/api/v1"+path, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var errBody struct {
			Error string `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errBody) //nolint:errcheck
		return fmt.Errorf("API error %d: %s", resp.StatusCode, errBody.Error)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// Get performs GET /api/v1{path} and decodes JSON response into out.
func (c *Client) Get(ctx context.Context, path string, out interface{}) error {
	return c.do(ctx, "GET", path, nil, out)
}

// Post performs POST /api/v1{path} with body, decodes into out.
func (c *Client) Post(ctx context.Context, path string, body, out interface{}) error {
	return c.do(ctx, "POST", path, body, out)
}

// Delete performs DELETE /api/v1{path}.
func (c *Client) Delete(ctx context.Context, path string) error {
	return c.do(ctx, "DELETE", path, nil, nil)
}
