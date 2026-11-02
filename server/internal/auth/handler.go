package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/valt-dev/valt/server/internal/config"
	"github.com/valt-dev/valt/server/pkg/apierror"
	"github.com/valt-dev/valt/server/pkg/validator"
)

// EmailSender is the interface for sending transactional emails.
type EmailSender interface {
	Send(ctx context.Context, to, subject, body string) error
}

// Handler holds the DB pool and JWT manager for auth endpoints.
type Handler struct {
	pool              *pgxpool.Pool
	jwtMgr            *JWTManager
	oauthConfig       *oauth2.Config
	dashboardURL      string
	cliSessionHandler *CLISessionHandler
	totpHandler       *TOTPHandler
	emailSender       EmailSender
	loginLockout      *LoginLockout
}

// NewHandler constructs a Handler with required dependencies.
func NewHandler(pool *pgxpool.Pool, jwtMgr *JWTManager, cfg *config.Config) *Handler {
	oauthCfg := &oauth2.Config{
		ClientID:     cfg.GoogleClientID,
		ClientSecret: cfg.GoogleClientSecret,
		RedirectURL:  cfg.GoogleRedirectURL,
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
	cliHandler := NewCLISessionHandler(pool, cfg.BaseURL)
	return &Handler{
		pool:              pool,
		jwtMgr:            jwtMgr,
		oauthConfig:       oauthCfg,
		dashboardURL:      cfg.DashboardURL,
		cliSessionHandler: cliHandler,
		totpHandler:       nil, // set via SetTOTPHandler after masterKey is available
		loginLockout:      NewLoginLockout(),
	}
}

// SetTOTPHandler sets the TOTP handler (called after masterKey is available).
func (h *Handler) SetTOTPHandler(th *TOTPHandler) {
	h.totpHandler = th
}

// SetEmailSender sets the email sender for transactional emails.
func (h *Handler) SetEmailSender(s EmailSender) {
	h.emailSender = s
}

// CLIStart returns the CLI session start handler (for mounting outside rate limiter).
func (h *Handler) CLIStart() http.HandlerFunc {
	if h.cliSessionHandler != nil {
		return h.cliSessionHandler.Start
	}
	return func(w http.ResponseWriter, r *http.Request) {
		apierror.NotFound(w, "CLI auth not configured")
	}
}

// CLIPoll returns the CLI session poll handler (for mounting outside rate limiter).
func (h *Handler) CLIPoll() http.HandlerFunc {
	if h.cliSessionHandler != nil {
		return h.cliSessionHandler.Poll
	}
	return func(w http.ResponseWriter, r *http.Request) {
		apierror.NotFound(w, "CLI auth not configured")
	}
}

// Routes returns a chi.Router with all auth routes mounted.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
	r.Get("/google", h.googleLogin)
	r.Get("/google/callback", h.googleCallback)
	r.Get("/verify-email", h.verifyEmail)
	r.Post("/forgot-password", h.forgotPassword)
	r.Post("/reset-password", h.resetPassword)
	// CLI endpoints mounted separately in main.go (exempt from login rate limiter)
	if h.totpHandler != nil {
		r.Post("/totp/validate", h.totpHandler.Validate)
	}
	// Authenticated routes (require Bearer token).
	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware(h.pool, h.jwtMgr))
		r.Post("/resend-verification", h.resendVerification)
		r.Get("/me", h.getMe)
	})
	return r
}

// --- Request / Response types ---

type registerRequest struct {
	Email      string `json:"email"`
	Password   string `json:"password"`
	RegionCode string `json:"region_code"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type authResponse struct {
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	ExpiresIn       int    `json:"expires_in"`
	NeedsOnboarding bool   `json:"needs_onboarding,omitempty"`
}

// --- Handlers ---

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	if err := validator.ValidateEmail(req.Email); err != nil {
		apierror.BadRequest(w, err.Error())
		return
	}
	if err := validator.ValidatePassword(req.Password); err != nil {
		apierror.BadRequest(w, err.Error())
		return
	}
	if err := validator.ValidateRegionCode(req.RegionCode); err != nil {
		apierror.BadRequest(w, err.Error())
		return
	}

	hash, err := HashPassword(req.Password)
	if err != nil {
		log.Printf("auth: hash password: %v", err)
		apierror.InternalError(w, "failed to process registration")
		return
	}

	var userID string
	err = h.pool.QueryRow(r.Context(),
		`INSERT INTO users (email, password_hash, region_code) VALUES ($1, $2, $3) RETURNING id`,
		req.Email, hash, req.RegionCode,
	).Scan(&userID)
	if err != nil {
		if isDuplicateKeyError(err) {
			apierror.Conflict(w, "email already registered")
			return
		}
		log.Printf("auth: insert user: %v", err)
		apierror.InternalError(w, "failed to create account")
		return
	}

	// Auto-create default org + workspace for new user (fire-and-forget on failure).
	h.autoCreateOrgAndWorkspace(r.Context(), userID, req.Email)

	// Send verification email (non-blocking: log failure but don't fail registration).
	if err := h.sendVerificationEmail(r.Context(), userID, req.Email); err != nil {
		log.Printf("auth: send verification email: %v", err)
	}

	h.issueTokens(w, r, userID, http.StatusCreated)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	// Check login lockout before doing any DB work
	if h.loginLockout.IsLocked(req.Email) {
		apierror.TooManyRequests(w, "account temporarily locked due to too many failed attempts, try again in 15 minutes")
		return
	}

	var userID, passwordHash, status string
	err := h.pool.QueryRow(r.Context(),
		`SELECT id, password_hash, status FROM users WHERE email = $1`,
		req.Email,
	).Scan(&userID, &passwordHash, &status)
	if err != nil {
		if err == pgx.ErrNoRows {
			h.loginLockout.RecordFailure(req.Email)
			apierror.Unauthorized(w, "invalid email or password")
			return
		}
		log.Printf("auth: query user: %v", err)
		apierror.InternalError(w, "login failed")
		return
	}

	if status != "active" {
		apierror.Forbidden(w, "account is not active")
		return
	}

	match, err := VerifyPassword(req.Password, passwordHash)
	if err != nil || !match {
		h.loginLockout.RecordFailure(req.Email)
		apierror.Unauthorized(w, "invalid email or password")
		return
	}

	// Successful login — clear any failed attempts
	h.loginLockout.ClearFailures(req.Email)

	// Check if TOTP 2FA is enabled
	var totpEnabled bool
	_ = h.pool.QueryRow(r.Context(), `SELECT totp_enabled FROM users WHERE id = $1`, userID).Scan(&totpEnabled)
	if totpEnabled {
		totpToken, tErr := h.jwtMgr.GenerateTOTPToken(userID)
		if tErr != nil {
			apierror.InternalError(w, "failed to generate TOTP challenge")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{ // status not yet written; Encode failure falls back to empty 200
			"requires_totp": true,
			"totp_token":    totpToken,
		})
		return
	}

	h.issueTokens(w, r, userID, http.StatusOK)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}

	if req.RefreshToken == "" {
		apierror.BadRequest(w, "refresh_token is required")
		return
	}

	tokenHash := hashRefreshToken(req.RefreshToken)

	var userID string
	var expiresAt time.Time
	err := h.pool.QueryRow(r.Context(),
		`SELECT user_id, expires_at FROM refresh_tokens WHERE token_hash = $1`,
		tokenHash,
	).Scan(&userID, &expiresAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			apierror.Unauthorized(w, "invalid refresh token")
			return
		}
		log.Printf("auth: query refresh token: %v", err)
		apierror.InternalError(w, "token refresh failed")
		return
	}

	if time.Now().After(expiresAt) {
		apierror.Unauthorized(w, "refresh token expired")
		return
	}

	// Rotate: delete old token before issuing new one.
	_, _ = h.pool.Exec(r.Context(),
		`DELETE FROM refresh_tokens WHERE token_hash = $1`, tokenHash)

	h.issueTokens(w, r, userID, http.StatusOK)
}

// --- Helpers ---

// issueTokens generates an access + refresh token pair, persists the refresh
// token hash, and writes the JSON response with the given HTTP status code.
// For new users (201 Created), sets needs_onboarding=true if they have no projects.
func (h *Handler) issueTokens(w http.ResponseWriter, r *http.Request, userID string, statusCode int) {
	accessToken, err := h.jwtMgr.GenerateAccessToken(userID)
	if err != nil {
		log.Printf("auth: generate access token: %v", err)
		apierror.InternalError(w, "failed to generate tokens")
		return
	}

	refreshToken, err := h.jwtMgr.GenerateRefreshToken()
	if err != nil {
		log.Printf("auth: generate refresh token: %v", err)
		apierror.InternalError(w, "failed to generate tokens")
		return
	}

	tokenHash := hashRefreshToken(refreshToken)
	_, err = h.pool.Exec(r.Context(),
		`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, $3)`,
		userID, tokenHash, time.Now().Add(7*24*time.Hour),
	)
	if err != nil {
		log.Printf("auth: store refresh token: %v", err)
		apierror.InternalError(w, "failed to generate tokens")
		return
	}

	// Determine if user needs onboarding (no projects yet).
	needsOnboarding := h.userNeedsOnboarding(r.Context(), userID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(authResponse{ //nolint:errcheck
		AccessToken:     accessToken,
		RefreshToken:    refreshToken,
		ExpiresIn:       900, // 15 minutes in seconds
		NeedsOnboarding: needsOnboarding,
	})
}

// userNeedsOnboarding returns true if the user has no projects (across all their orgs).
func (h *Handler) userNeedsOnboarding(ctx context.Context, userID string) bool {
	var count int
	err := h.pool.QueryRow(ctx, `
		SELECT COUNT(p.id)
		FROM projects p
		JOIN workspaces w ON w.id = p.workspace_id
		JOIN organizations o ON o.id = w.org_id
		JOIN org_memberships om ON om.org_id = o.id
		WHERE om.user_id = $1
	`, userID).Scan(&count)
	if err != nil {
		return false
	}
	return count == 0
}

// autoCreateOrgAndWorkspace creates a default org (derived from email domain) and
// a default workspace for a newly registered user. Failures are logged but non-fatal.
func (h *Handler) autoCreateOrgAndWorkspace(ctx context.Context, userID, email string) {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return
	}
	domain := parts[1]

	// Derive org name: capitalize first letter of domain prefix (e.g., "acme.com" -> "Acme")
	domainPrefix := strings.SplitN(domain, ".", 2)[0]
	orgName := capitalizeFirst(domainPrefix) + "'s Org"
	orgSlug := strings.ReplaceAll(domain, ".", "-")

	var orgID string
	err := h.pool.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, owner_id, plan) VALUES ($1, $2, $3, 'free') RETURNING id`,
		orgName, orgSlug, userID,
	).Scan(&orgID)
	if err != nil {
		// Slug conflict: retry with random suffix
		orgSlug = orgSlug + "-" + randomHex4()
		err = h.pool.QueryRow(ctx,
			`INSERT INTO organizations (name, slug, owner_id, plan) VALUES ($1, $2, $3, 'free') RETURNING id`,
			orgName, orgSlug, userID,
		).Scan(&orgID)
		if err != nil {
			log.Printf("auth: auto-create org for user %s: %v", userID, err)
			return
		}
	}

	// Add user as org owner
	_, _ = h.pool.Exec(ctx,
		`INSERT INTO org_memberships (org_id, user_id, role) VALUES ($1, $2, 'owner')`,
		orgID, userID,
	)

	// Create default workspace
	_, err = h.pool.Exec(ctx,
		`INSERT INTO workspaces (org_id, name, slug) VALUES ($1, 'Default', 'default')`,
		orgID,
	)
	if err != nil {
		log.Printf("auth: auto-create workspace for org %s: %v", orgID, err)
	}
}

// capitalizeFirst returns s with the first letter uppercased.
func capitalizeFirst(s string) string {
	if s == "" {
		return s
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// randomHex4 returns a 4-character random hex string.
func randomHex4() string {
	b := make([]byte, 2)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// hashRefreshToken returns the hex-encoded SHA-256 of the raw token string.
func hashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// isDuplicateKeyError reports whether err is a PostgreSQL unique-violation (23505).
func isDuplicateKeyError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
