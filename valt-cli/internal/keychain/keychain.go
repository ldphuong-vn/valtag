package keychain

import (
	"fmt"
	"os"

	"github.com/zalando/go-keyring"
)

// service is the unified keychain service name shared between CLI and MCP server.
const service = "valtag"

// legacyService is the old CLI service name kept for backward-compat migration.
const legacyService = "valt-cli"

const userKey = "auth-token"

// GetToken returns the stored auth token, checking env var first,
// then the unified "valtag" keychain entry, then the legacy "valt-cli" entry.
func GetToken() (string, error) {
	if v := os.Getenv("VALTAG_AUTH_TOKEN"); v != "" {
		return v, nil
	}
	// Try new unified service name first.
	token, err := keyring.Get(service, userKey)
	if err == nil && token != "" {
		return token, nil
	}
	// Fallback: try legacy service name for backward compatibility.
	token, err = keyring.Get(legacyService, userKey)
	if err == nil && token != "" {
		// Migrate: re-store under new service name.
		_ = keyring.Set(service, userKey, token)
		return token, nil
	}
	return "", nil // not found = not logged in
}

// SetToken stores the auth token in the OS keychain under the unified service name.
func SetToken(token string) error {
	if err := keyring.Set(service, userKey, token); err != nil {
		return fmt.Errorf("storing token in keychain: %w", err)
	}
	return nil
}

// DeleteToken removes the stored token from both the unified and legacy service names.
func DeleteToken() error {
	// Delete from new service name (ignore not-found errors).
	_ = keyring.Delete(service, userKey)
	// Delete from legacy service name (ignore not-found errors).
	_ = keyring.Delete(legacyService, userKey)
	return nil
}
