package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/database"
	"github.com/ldphuong-vn/valtag/server/internal/testutil"
	"github.com/ldphuong-vn/valtag/server/pkg/crypto"
)

// TestRotateMasterKeyEndToEnd runs the compiled rotation tool against a real
// Postgres with a seeded secret row: after rotation, the DEK must decrypt
// with the NEW key and fail with the OLD one, and the audit chain must carry
// the rotation event.
func TestRotateMasterKeyEndToEnd(t *testing.T) {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for rotation integration test")
	}

	oldKey := make([]byte, 32)
	newKey := make([]byte, 32)
	if _, err := rand.Read(oldKey); err != nil {
		t.Fatal(err)
	}
	if _, err := rand.Read(newKey); err != nil {
		t.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Per-test schema so migrations start clean regardless of what else
	// lives in the target database (same pattern as the workflow helper).
	schemaName := "it_rotate_" + base64.RawURLEncoding.EncodeToString([]byte(t.Name()))[:12]
	adminPool := pool
	if _, err := adminPool.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() { _, _ = adminPool.Exec(context.Background(), "DROP SCHEMA "+schemaName+" CASCADE") }()

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName
	schemaPool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer schemaPool.Close()
	pool = schemaPool

	if err := testutil.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	database.EnsurePartitions(ctx, pool)

	// Seed: user → org → workspace → project → secret (DEK wrapped with OLD key).
	userID := "00000000-0000-0000-0000-000000000b01"
	orgID := "00000000-0000-0000-0000-000000000b02"
	wsID := "00000000-0000-0000-0000-000000000b03"
	projectID := "00000000-0000-0000-0000-000000000b04"
	secretID := "00000000-0000-0000-0000-000000000b05"
	stmts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, email, password_hash, email_verified) VALUES ($1, $2, $3, true)`, []any{userID, "rotate@example.test", "hash"}},
		{`INSERT INTO organizations (id, name, slug, owner_id) VALUES ($1, 'Org', 'org-rotate', $2)`, []any{orgID, userID}},
		{`INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, 'WS', 'ws-rotate')`, []any{wsID, orgID}},
		{`INSERT INTO projects (id, workspace_id, name, slug) VALUES ($1, $2, 'P', 'proj-rotate')`, []any{projectID, wsID}},
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.query, s.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	rawDEK := make([]byte, 32)
	if _, err := rand.Read(rawDEK); err != nil {
		t.Fatal(err)
	}
	wrappedOld, err := crypto.EncryptAES256GCM(oldKey, rawDEK)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO secrets (id, user_id, project_id, name, storage_key, encrypted_dek) VALUES ($1, $2, $3, 'rotate-me', 'rotate-key-1', $4)`,
		secretID, userID, projectID, wrappedOld); err != nil {
		t.Fatalf("seed secret: %v", err)
	}

	// Run the same rotation function main() drives.
	if err := runRotation(ctx, pool, oldKey, newKey); err != nil {
		t.Fatalf("rotation: %v", err)
	}

	// The DEK must now decrypt with NEW and fail with OLD.
	var wrappedAfter []byte
	if err := pool.QueryRow(ctx, `SELECT encrypted_dek FROM secrets WHERE id = $1`, secretID).Scan(&wrappedAfter); err != nil {
		t.Fatal(err)
	}
	got, err := crypto.DecryptAES256GCM(newKey, wrappedAfter)
	if err != nil {
		t.Fatalf("decrypt with NEW key failed: %v", err)
	}
	if string(got) != string(rawDEK) {
		t.Fatalf("DEK content changed during rotation")
	}
	if _, err := crypto.DecryptAES256GCM(oldKey, wrappedAfter); err == nil {
		t.Fatal("OLD key must no longer decrypt the DEK")
	}

	// Audit chain carries the rotation event.
	var action string
	if err := pool.QueryRow(ctx,
		`SELECT action FROM audit_logs WHERE action = 'master_key.rotate' ORDER BY seq DESC LIMIT 1`,
	).Scan(&action); err != nil {
		t.Fatalf("rotation not audited: %v", err)
	}
}

// TestDecodeKeyRejectsWrongSize covers the input validation.
func TestDecodeKeyRejectsWrongSize(t *testing.T) {
	if _, err := decodeKey("K", base64.StdEncoding.EncodeToString(make([]byte, 16))); err == nil {
		t.Fatal("16-byte key accepted")
	}
	if _, err := decodeKey("K", "not-base64!!!"); err == nil {
		t.Fatal("non-base64 accepted")
	}
}
