package workflow

import (
	"context"
	"testing"

	"github.com/ldphuong-vn/valtag/server/internal/audit"
)

// TestRevokeAllKillsHumanAccess covers step 5 of the revoke cascade
// (report §3.2): web refresh tokens are deleted and the CLI token_version
// is bumped, so human sessions and CLI/MCP JWTs die with everything else.
func TestRevokeAllKillsHumanAccess(t *testing.T) {
	ctx := context.Background()
	pool, dynSvc, credMgr, _, cleanup := newLeaseE2EStack(t, ctx)
	defer cleanup()

	// Minimal target: a user with outstanding refresh tokens.
	var targetUserID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('revoke-human@example.test', 'test-hash') RETURNING id`,
	).Scan(&targetUserID); err != nil {
		t.Fatalf("seed target user: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := pool.Exec(ctx,
			`INSERT INTO refresh_tokens (user_id, token_hash, expires_at) VALUES ($1, $2, now() + INTERVAL '7 days')`,
			targetUserID, "hash-"+string(rune('a'+i))); err != nil {
			t.Fatalf("seed refresh token: %v", err)
		}
	}

	svc := NewRevokeService(pool, dynSvc, credMgr, audit.NewLogger(pool))
	summary, err := svc.RevokeAllForUser(ctx, targetUserID, "actor")
	if err != nil {
		t.Fatalf("revoke all: %v", err)
	}

	if summary.RefreshTokensRevoked != 2 {
		t.Fatalf("RefreshTokensRevoked = %d, want 2", summary.RefreshTokensRevoked)
	}
	if !summary.CLISessionsKilled {
		t.Fatal("CLISessionsKilled = false, want true")
	}

	var remaining int
	if err := pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM refresh_tokens WHERE user_id = $1`, targetUserID).Scan(&remaining); err != nil {
		t.Fatalf("count refresh tokens: %v", err)
	}
	if remaining != 0 {
		t.Fatalf("refresh tokens remaining = %d, want 0", remaining)
	}

	var version int
	if err := pool.QueryRow(ctx,
		`SELECT token_version FROM users WHERE id = $1`, targetUserID).Scan(&version); err != nil {
		t.Fatalf("read token_version: %v", err)
	}
	if version != 2 {
		t.Fatalf("token_version = %d, want 2 (bumped once)", version)
	}

	// Revoke-all is idempotent-safe: a second run keeps the invariant.
	if _, err := svc.RevokeAllForUser(ctx, targetUserID, "actor"); err != nil {
		t.Fatalf("second revoke all: %v", err)
	}
	if err := pool.QueryRow(ctx,
		`SELECT token_version FROM users WHERE id = $1`, targetUserID).Scan(&version); err != nil {
		t.Fatalf("read token_version after second revoke: %v", err)
	}
	if version != 3 {
		t.Fatalf("token_version after second revoke = %d, want 3", version)
	}
}
