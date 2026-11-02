package auth

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/valt-dev/valt/server/internal/testutil"
)

// newAuthIntegrationDB mirrors the workflow integration helper: per-test
// schema, full migration application, advisory-locked.
func newAuthIntegrationDB(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for auth integration tests")
	}

	schemaName := fmt.Sprintf("it_auth_%d_%d", time.Now().UnixNano(), rand.Intn(100000))
	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create admin pool failed: %v", err)
	}
	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName)); err != nil {
		adminPool.Close()
		t.Skipf("cannot create test schema (need CREATE privilege): %v", err)
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse database url failed: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		adminPool.Close()
		t.Fatalf("create schema pool failed: %v", err)
	}
	if err := testutil.ApplyMigrations(ctx, pool); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schemaName))
		adminPool.Close()
		t.Fatalf("apply migrations failed: %v", err)
	}

	cleanup := func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", schemaName))
		adminPool.Close()
	}
	return pool, cleanup
}

// TestAuthMiddlewareCLITokenVersion proves revoke-all's token_version bump
// kills outstanding CLI/MCP tokens (typ=cli) while short-lived access tokens
// stay unaffected — the "revoke-all kills human sessions too" guarantee.
func TestAuthMiddlewareCLITokenVersion(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newAuthIntegrationDB(t, ctx)
	defer cleanup()

	var userID string
	if err := db.QueryRow(ctx,
		`INSERT INTO users (email, password_hash) VALUES ('cli-version@example.test', 'test-hash') RETURNING id`,
	).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	privPEM, pubPEM := generateTestKeys(t)
	mgr, err := NewJWTManager(privPEM, pubPEM)
	if err != nil {
		t.Fatalf("jwt manager: %v", err)
	}

	handler := AuthMiddleware(db, mgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	request := func(token string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	t.Run("CLI token with current version passes", func(t *testing.T) {
		cliToken, err := mgr.GenerateCLIToken(userID, 1)
		if err != nil {
			t.Fatalf("generate cli token: %v", err)
		}
		if rec := request(cliToken); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
	})

	t.Run("version bump kills the outstanding CLI token", func(t *testing.T) {
		if _, err := db.Exec(ctx, `UPDATE users SET token_version = token_version + 1 WHERE id = $1`, userID); err != nil {
			t.Fatalf("bump token_version: %v", err)
		}
		staleToken, err := mgr.GenerateCLIToken(userID, 1)
		if err != nil {
			t.Fatalf("generate stale cli token: %v", err)
		}
		if rec := request(staleToken); rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})

	t.Run("CLI token re-issued after the bump passes", func(t *testing.T) {
		freshToken, err := mgr.GenerateCLIToken(userID, 2)
		if err != nil {
			t.Fatalf("generate fresh cli token: %v", err)
		}
		if rec := request(freshToken); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
	})

	t.Run("access token survives the bump", func(t *testing.T) {
		accessToken, err := mgr.GenerateAccessToken(userID)
		if err != nil {
			t.Fatalf("generate access token: %v", err)
		}
		if rec := request(accessToken); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
	})

	t.Run("nil pool rejects CLI tokens (fail-closed)", func(t *testing.T) {
		nilHandler := AuthMiddleware(nil, mgr)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		token, err := mgr.GenerateCLIToken(userID, 2)
		if err != nil {
			t.Fatalf("generate cli token: %v", err)
		}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		nilHandler.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})
}
