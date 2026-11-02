package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"

	"github.com/kelseyhightower/envconfig"
)

// Config holds all application configuration loaded from environment variables.
type Config struct {
	// cached ephemeral master key (C3 fix: MasterKey() is idempotent)
	cachedMasterKey []byte
	// Server
	Port        string `envconfig:"PORT" default:"8080"`
	CORSOrigins string `envconfig:"CORS_ORIGINS" default:"http://localhost:3000,http://localhost:8443"`

	// Database
	DatabaseURL string `envconfig:"DATABASE_URL" required:"true"`

	// JWT
	JWTPrivateKeyPath string `envconfig:"JWT_PRIVATE_KEY_PATH" default:"./keys/private.pem"`
	JWTPublicKeyPath  string `envconfig:"JWT_PUBLIC_KEY_PATH" default:"./keys/public.pem"`

	// SMTP (optional - no-op mode when host is empty)
	SMTPHost     string `envconfig:"SMTP_HOST" default:""`
	SMTPPort     int    `envconfig:"SMTP_PORT" default:"587"`
	SMTPUser     string `envconfig:"SMTP_USER" default:""`
	SMTPPassword string `envconfig:"SMTP_PASSWORD" default:""`
	SMTPFrom     string `envconfig:"SMTP_FROM" default:"noreply@valt.dev"`

	// Google OAuth
	GoogleClientID     string `envconfig:"GOOGLE_CLIENT_ID" default:""`
	GoogleClientSecret string `envconfig:"GOOGLE_CLIENT_SECRET" default:""`
	GoogleRedirectURL  string `envconfig:"GOOGLE_REDIRECT_URL" default:"http://localhost:8080/api/v1/auth/google/callback"`
	DashboardURL       string `envconfig:"DASHBOARD_URL" default:"http://localhost:3000"`

	// Vault encryption
	// VaultMasterKey is a base64-encoded 32-byte AES-256 key used to wrap DEKs.
	// If not set, an ephemeral key is generated (secrets won't survive restart).
	VaultMasterKey string `envconfig:"VAULT_MASTER_KEY" default:""`

	// Redis (optional — empty disables Redis-backed features)
	RedisURL string `envconfig:"REDIS_URL" default:""`

	// Base URL for generating action links in emails
	BaseURL string `envconfig:"BASE_URL" default:"http://localhost:8080"`

	// Slack (optional — empty disables Slack notifications)
	SlackBotToken      string `envconfig:"SLACK_BOT_TOKEN" default:""`
	SlackSigningSecret string `envconfig:"SLACK_SIGNING_SECRET" default:""`
	SlackClientID      string `envconfig:"SLACK_CLIENT_ID" default:""`
	SlackClientSecret  string `envconfig:"SLACK_CLIENT_SECRET" default:""`

	// Telegram (optional — empty disables Telegram notifications)
	TelegramBotToken    string `envconfig:"TELEGRAM_BOT_TOKEN" default:""`
	TelegramBotUsername string `envconfig:"TELEGRAM_BOT_USERNAME" default:"valtbot"`
	// Required when TELEGRAM_BOT_TOKEN is set: registered as secret_token via
	// SetWebhook and verified on every callback (fail-closed otherwise).
	TelegramWebhookSecret string `envconfig:"TELEGRAM_WEBHOOK_SECRET" default:""`

	// Zalo OA (optional — empty disables Zalo notifications)
	ZaloOAToken string `envconfig:"ZALO_OA_TOKEN" default:""`
	ZaloOAID    string `envconfig:"ZALO_OA_ID" default:""`

	// Stripe (optional — empty disables billing)
	StripeSecretKey     string `envconfig:"STRIPE_SECRET_KEY" default:""`
	StripeWebhookSecret string `envconfig:"STRIPE_WEBHOOK_SECRET" default:""`
	StripePriceProID    string `envconfig:"STRIPE_PRO_PRICE_ID" default:""`
	StripePriceTeamID   string `envconfig:"STRIPE_TEAM_PRICE_ID" default:""`

	// Policy enforcement rollout
	PolicyEnforcementV2Enabled bool `envconfig:"POLICY_ENFORCEMENT_V2_ENABLED" default:"true"`

	// Gateway proxy (optional — disabled by default)
	GatewayEnabled bool   `envconfig:"GATEWAY_ENABLED" default:"false"`
	GatewayPort    string `envconfig:"GATEWAY_PORT" default:"10256"`
}

// MasterKey decodes VaultMasterKey from base64 and returns the raw 32 bytes.
// If VaultMasterKey is empty, an ephemeral key is generated once and cached (C3 fix).
func (c *Config) MasterKey() ([]byte, error) {
	if c.VaultMasterKey == "" {
		if c.cachedMasterKey != nil {
			return c.cachedMasterKey, nil
		}
		log.Println("WARNING: VAULT_MASTER_KEY not set, using ephemeral key - secrets will not survive restart")
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generating ephemeral master key: %w", err)
		}
		c.cachedMasterKey = key
		return key, nil
	}
	key, err := base64.StdEncoding.DecodeString(c.VaultMasterKey)
	if err != nil {
		return nil, fmt.Errorf("decoding VAULT_MASTER_KEY: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("VAULT_MASTER_KEY must be 32 bytes when decoded, got %d", len(key))
	}
	return key, nil
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	var cfg Config
	if err := envconfig.Process("", &cfg); err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}
	return &cfg, nil
}
