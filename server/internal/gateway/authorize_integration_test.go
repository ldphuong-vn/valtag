package gateway

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/audit"
	"github.com/ldphuong-vn/valtag/server/internal/database"
	"github.com/ldphuong-vn/valtag/server/internal/dynsecret"
	"github.com/ldphuong-vn/valtag/server/internal/testutil"
	"github.com/ldphuong-vn/valtag/server/pkg/crypto"
)

// TestAuthorizeEndpoint covers POST /gateway/v1/authorize — the ext-auth
// contract for the Higress demo (Model A): derived key in → real credential
// headers out, fail-closed on anything else (research §3).
func TestAuthorizeEndpoint(t *testing.T) {
	ctx := context.Background()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for gateway authorize integration test")
	}

	base, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer base.Close()

	// Per-test schema so migrations start clean regardless of the target DB.
	schemaName := "it_authorize_" + fmt.Sprintf("%d", os.Getpid()) + "_" + strings.ReplaceAll(t.Name(), "/", "_")
	if _, err := base.Exec(ctx, "CREATE SCHEMA "+schemaName); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	defer func() { _, _ = base.Exec(context.Background(), "DROP SCHEMA "+schemaName+" CASCADE") }()

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	if err := testutil.ApplyMigrations(ctx, pool); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	database.EnsurePartitions(ctx, pool)

	masterKey := make([]byte, 32)
	if _, err := rand.Read(masterKey); err != nil {
		t.Fatal(err)
	}
	masterKeyB64 := base64.StdEncoding.EncodeToString(masterKey)

	// Seed: user → org → workspace → project → derived_api_key provider → agent.
	userID, orgID, wsID, projectID := "00000000-0000-0000-0000-000000000c01", "00000000-0000-0000-0000-000000000c02", "00000000-0000-0000-0000-000000000c03", "00000000-0000-0000-0000-000000000c04"
	stmts := []struct {
		query string
		args  []any
	}{
		{`INSERT INTO users (id, email, password_hash, email_verified) VALUES ($1, $2, $3, true)`, []any{userID, "authorize@example.test", "hash"}},
		{`INSERT INTO organizations (id, name, slug, owner_id) VALUES ($1, 'Org', 'org-authorize', $2)`, []any{orgID, userID}},
		{`INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, 'WS', 'ws-authorize')`, []any{wsID, orgID}},
		{`INSERT INTO projects (id, workspace_id, name, slug) VALUES ($1, $2, 'P', 'proj-authorize')`, []any{projectID, wsID}},
	}
	for _, s := range stmts {
		if _, err := pool.Exec(ctx, s.query, s.args...); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	dynSvc := dynsecret.NewService(pool, masterKey)
	encCfg, err := crypto.EncryptAES256GCM(masterKey, []byte(`{"master_key":"`+masterKeyB64+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	var providerID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO dynamic_providers (project_id, name, provider_type, config_enc, created_by) VALUES ($1, 'authorize-test', 'derived_api_key', $2, $3) RETURNING id`,
		projectID, encCfg, userID).Scan(&providerID); err != nil {
		t.Fatalf("seed provider: %v", err)
	}

	var agentID string
	if err := pool.QueryRow(ctx,
		`INSERT INTO agent_identities (project_id, name, created_by) VALUES ($1, 'authorize-agent', $2) RETURNING id`,
		projectID, userID).Scan(&agentID); err != nil {
		t.Fatalf("seed agent: %v", err)
	}

	// Issue the lease the way the agent path does; the api_key credential is
	// the raw derived key the agent holds.
	leaseRow, err := dynSvc.CreateLease(ctx, providerID, agentID, "", 300)
	if err != nil {
		t.Fatalf("create lease: %v", err)
	}
	rawDerivedKey := leaseRow.Credentials["api_key"]
	if !strings.HasPrefix(rawDerivedKey, "valtag_dk_") {
		t.Fatalf("derived key missing valtag_dk_ prefix: %q", rawDerivedKey)
	}

	server := NewServer(nil, nil, nil, dynSvc, audit.NewLogger(pool), masterKey, "0")

	call := func(headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/gateway/v1/authorize", nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		server.ServeHTTP(rec, req)
		return rec
	}

	t.Run("derived key in returns real credential headers", func(t *testing.T) {
		rec := call(map[string]string{"Authorization": "Bearer " + rawDerivedKey})
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200 — body: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
		}
		if got := rec.Header().Get("Authorization"); got != "Bearer "+masterKeyB64 {
			t.Fatalf("credential header = %q, want bearer master key", got)
		}
		for _, h := range []string{"x-valtag-agent-id", "x-valtag-lease-id", "x-valtag-lease-expires"} {
			if rec.Header().Get(h) == "" {
				t.Fatalf("missing %s", h)
			}
		}
		if got := rec.Header().Get("x-valtag-agent-id"); got != agentID {
			t.Fatalf("agent id = %q, want %q", got, agentID)
		}
	})

	t.Run("agent token binding rides along in Proxy-Authorization", func(t *testing.T) {
		rec := call(map[string]string{
			"Proxy-Authorization": "Bearer not-a-real-agent-token",
			"Authorization":       "Bearer " + rawDerivedKey,
		})
		// Derived-key-only path validates the lease; the unparseable agent
		// token is tolerated (binding enforced when the token is real).
		if rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
	})

	t.Run("unknown derived key is denied 401", func(t *testing.T) {
		rec := call(map[string]string{"Authorization": "Bearer valtag_dk_ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})

	t.Run("no derived key at all is denied 401", func(t *testing.T) {
		rec := call(nil)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})

	t.Run("revoked lease is denied 401", func(t *testing.T) {
		if _, err := pool.Exec(ctx, `UPDATE dynamic_leases SET revoked_at = now() WHERE provider_id = $1`, providerID); err != nil {
			t.Fatal(err)
		}
		rec := call(map[string]string{"Authorization": "Bearer " + rawDerivedKey})
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})
}
