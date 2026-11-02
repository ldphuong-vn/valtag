package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ValidateUserToken validates an access/CLI JWT. For typ=cli tokens it also
// verifies the embedded ver claim against users.token_version, so a bump
// (revoke-all) invalidates every outstanding CLI/MCP token. Short-lived
// access tokens skip the lookup — the hot path stays DB-free.
// db must be non-nil for CLI tokens; a nil pool rejects them (safe default
// for unit-test contexts that only exercise access tokens).
func ValidateUserToken(ctx context.Context, db *pgxpool.Pool, jwtMgr *JWTManager, token string) (string, error) {
	userID, typ, version, err := jwtMgr.validateAccessTokenDetailed(token)
	if err != nil {
		return "", err
	}
	if typ != "cli" {
		return userID, nil
	}
	if db == nil {
		return "", errors.New("cli token presented but no database available for version check")
	}

	var current int
	err = db.QueryRow(ctx, `SELECT token_version FROM users WHERE id = $1`, userID).Scan(&current)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", fmt.Errorf("unknown user %q", userID)
		}
		return "", err
	}
	if version != current {
		return "", fmt.Errorf("token revoked (issued at version %d, current %d)", version, current)
	}
	return userID, nil
}
