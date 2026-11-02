package integration

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/internal/rbac"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// SlackOAuthHandler manages Slack OAuth v2 install flow.
type SlackOAuthHandler struct {
	store        *Store
	pool         *pgxpool.Pool
	clientID     string
	clientSecret string
	redirectURI  string
	dashboardURL string
	masterKey    []byte
}

// NewSlackOAuthHandler creates a Slack OAuth handler.
func NewSlackOAuthHandler(store *Store, pool *pgxpool.Pool, clientID, clientSecret, redirectURI, dashboardURL string, masterKey []byte) *SlackOAuthHandler {
	return &SlackOAuthHandler{
		store: store, pool: pool, clientID: clientID, clientSecret: clientSecret,
		redirectURI: redirectURI, dashboardURL: dashboardURL, masterKey: masterKey,
	}
}

// Authorize redirects to Slack OAuth authorize URL. GET /oauth/slack?org_id=X
// Org admin only: the OAuth result is bound to the org from the state, so
// allowing arbitrary callers would let anyone attach their own Slack bot to
// an org and receive its approval notifications.
func (h *SlackOAuthHandler) Authorize(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		apierror.BadRequest(w, "org_id required")
		return
	}
	if !requireOrgRole(w, r, h.pool, orgID, rbac.OrgAdmin) {
		return
	}

	state := h.generateState(orgID)
	slackURL := fmt.Sprintf(
		"https://slack.com/oauth/v2/authorize?client_id=%s&scope=chat:write,users:read&state=%s&redirect_uri=%s",
		url.QueryEscape(h.clientID),
		url.QueryEscape(state),
		url.QueryEscape(h.redirectURI),
	)
	http.Redirect(w, r, slackURL, http.StatusFound)
}

// Callback handles Slack OAuth callback. GET /oauth/slack/callback?code=X&state=Y
func (h *SlackOAuthHandler) Callback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		apierror.BadRequest(w, "missing code or state")
		return
	}

	orgID, err := h.validateState(state)
	if err != nil {
		apierror.BadRequest(w, "invalid or expired state")
		return
	}

	// Exchange code for token
	token, err := h.exchangeCode(code)
	if err != nil {
		log.Printf("[slack-oauth] exchange failed: %v", err)
		apierror.InternalError(w, "failed to exchange OAuth code")
		return
	}

	if err := h.store.Upsert(r.Context(), orgID, "slack", token.AccessToken, token.TeamID, token.TeamName, token.BotUserID); err != nil {
		log.Printf("[slack-oauth] store upsert failed: %v", err)
		apierror.InternalError(w, "failed to save integration")
		return
	}

	http.Redirect(w, r, h.dashboardURL+"/settings/notifications?slack=connected", http.StatusFound)
}

type slackTokenResponse struct {
	AccessToken string `json:"access_token"`
	TeamID      string
	TeamName    string
	BotUserID   string
}

func (h *SlackOAuthHandler) exchangeCode(code string) (*slackTokenResponse, error) {
	data := url.Values{
		"client_id":     {h.clientID},
		"client_secret": {h.clientSecret},
		"code":          {code},
		"redirect_uri":  {h.redirectURI},
	}

	resp, err := http.PostForm("https://slack.com/api/oauth.v2.access", data)
	if err != nil {
		return nil, fmt.Errorf("slack oauth request: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool   `json:"ok"`
		Error       string `json:"error"`
		AccessToken string `json:"access_token"`
		Team        struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"team"`
		BotUserID string `json:"bot_user_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	if !result.OK {
		return nil, fmt.Errorf("slack error: %s", result.Error)
	}

	return &slackTokenResponse{
		AccessToken: result.AccessToken,
		TeamID:      result.Team.ID,
		TeamName:    result.Team.Name,
		BotUserID:   result.BotUserID,
	}, nil
}

// generateState creates an HMAC-signed state: base64(orgID|timestamp|hmac)
func (h *SlackOAuthHandler) generateState(orgID string) string {
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	payload := orgID + "|" + ts
	mac := hmac.New(sha256.New, h.masterKey)
	mac.Write([]byte(payload))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + sig))
}

// validateState verifies HMAC and checks timestamp (10 min window).
func (h *SlackOAuthHandler) validateState(state string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(state)
	if err != nil {
		return "", fmt.Errorf("decode state: %w", err)
	}
	parts := strings.SplitN(string(raw), "|", 3)
	if len(parts) != 3 {
		return "", fmt.Errorf("invalid state format")
	}
	orgID, tsStr, sig := parts[0], parts[1], parts[2]

	// Verify HMAC
	payload := orgID + "|" + tsStr
	mac := hmac.New(sha256.New, h.masterKey)
	mac.Write([]byte(payload))
	expectedSig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(sig), []byte(expectedSig)) {
		return "", fmt.Errorf("invalid HMAC")
	}

	// Check timestamp
	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return "", fmt.Errorf("invalid timestamp")
	}
	if time.Now().Unix()-ts > 600 {
		return "", fmt.Errorf("state expired")
	}

	return orgID, nil
}

// IntegrationHandler serves integration list/delete endpoints.
type IntegrationHandler struct {
	store *Store
	pool  *pgxpool.Pool
}

// NewIntegrationHandler creates an IntegrationHandler.
func NewIntegrationHandler(store *Store, pool *pgxpool.Pool) *IntegrationHandler {
	return &IntegrationHandler{store: store, pool: pool}
}

// requireOrgRole is the shared in-handler membership gate for endpoints that
// take org_id as a query parameter. Returns false after writing the error.
func requireOrgRole(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, orgID, minRole string) bool {
	callerID := auth.UserIDFromContext(r.Context())
	if callerID == "" {
		apierror.Unauthorized(w, "authentication required")
		return false
	}
	role, err := rbac.UserOrgRole(r.Context(), pool, orgID, callerID)
	if err != nil {
		apierror.InternalError(w, "failed to check org membership")
		return false
	}
	if !rbac.OrgRoleAtLeast(role, minRole) {
		apierror.Forbidden(w, "insufficient org permissions")
		return false
	}
	return true
}

// List returns connected integrations for an org. GET /integrations?org_id=X
func (ih *IntegrationHandler) List(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		apierror.BadRequest(w, "org_id required")
		return
	}
	if !requireOrgRole(w, r, ih.pool, orgID, rbac.OrgViewer) {
		return
	}
	integrations, err := ih.store.ListByOrg(r.Context(), orgID)
	if err != nil {
		apierror.InternalError(w, "failed to list integrations")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"integrations": integrations})
}

// DeleteSlack disconnects Slack for an org. DELETE /integrations/slack?org_id=X
func (ih *IntegrationHandler) DeleteSlack(w http.ResponseWriter, r *http.Request) {
	orgID := r.URL.Query().Get("org_id")
	if orgID == "" {
		apierror.BadRequest(w, "org_id required")
		return
	}
	if !requireOrgRole(w, r, ih.pool, orgID, rbac.OrgAdmin) {
		return
	}
	if err := ih.store.Delete(r.Context(), orgID, "slack"); err != nil {
		apierror.InternalError(w, "failed to disconnect")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
