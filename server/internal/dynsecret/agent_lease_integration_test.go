package dynsecret

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/agent"
)

// TestCreateLeaseAgentAuthz covers the MCP request_dynamic_secret path
// (report §3.3 #4): agents mint leases with their own token, restricted to
// providers of their own project and to their own agent identity.
func TestCreateLeaseAgentAuthz(t *testing.T) {
	ctx := context.Background()
	pool, cleanup := newDynsecretIntegrationDB(t, ctx)
	defer cleanup()

	ownerID, projectID := seedSweepProjectChain(t, ctx, pool)
	masterKey := newSweepMasterKey(t)
	providerID := seedSweepProvider(t, ctx, pool, masterKey, ownerID, projectID, "postgres", map[string]string{
		"host": "localhost", "port": "15432", "database": "postgres",
		"admin_user": "postgres", "admin_password": "", "ssl_mode": "disable",
	})

	// Agents: one in the provider's project, one in a second project, plus a stranger.
	secondProjectID := seedSecondProject(t, ctx, pool, projectID)
	agentInProject := seedAgent(t, ctx, pool, projectID, ownerID)
	agentOtherProject := seedAgent(t, ctx, pool, secondProjectID, ownerID)

	h := NewHandler(&Service{db: pool, masterKey: masterKey}, pool)
	router := chi.NewRouter()
	router.Post("/providers/{provider_id}/leases", h.CreateLease)

	call := func(agentID, bodyAgentID, provider string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/providers/"+provider+"/leases",
			strings.NewReader(`{"ttl_seconds":300,"agent_id":"`+bodyAgentID+`"}`))
		req.Header.Set("Content-Type", "application/json")
		if agentID != "" {
			req = req.WithContext(agent.WithAgentID(req.Context(), agentID))
		}
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		return rec
	}

	t.Run("agent mints a lease on its own project's provider", func(t *testing.T) {
		if rec := call(agentInProject, agentInProject, providerID); rec.Code != http.StatusCreated {
			t.Fatalf("got %d, want 201 — body: %s", rec.Code, strings.TrimSpace(rec.Body.String()))
		}
	})

	t.Run("agent cannot mint for another agent identity", func(t *testing.T) {
		if rec := call(agentInProject, agentOtherProject, providerID); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})

	t.Run("agent cannot mint on another project's provider", func(t *testing.T) {
		if rec := call(agentOtherProject, agentOtherProject, providerID); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})

	t.Run("unknown agent gets 404, not 403", func(t *testing.T) {
		if rec := call("99999999-9999-9999-9999-999999999999", "99999999-9999-9999-9999-999999999999", providerID); rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", rec.Code)
		}
	})

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		if rec := call("", "", providerID); rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})
}

// seedSecondProject clones the seeded workspace's project under a new slug
// so a second agent can live in a different project of the same org.
func seedSecondProject(t *testing.T, ctx context.Context, pool *pgxpool.Pool, firstProjectID string) string {
	t.Helper()
	var wsID string
	if err := pool.QueryRow(ctx, `SELECT workspace_id FROM projects WHERE id = $1`, firstProjectID).Scan(&wsID); err != nil {
		t.Fatalf("lookup workspace: %v", err)
	}
	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, slug) VALUES ($1, 'Second', 'proj-sweep-2') RETURNING id`,
		wsID,
	).Scan(&id); err != nil {
		t.Fatalf("seed second project: %v", err)
	}
	return id
}

// seedAgent inserts an agent identity and returns its ID.
func seedAgent(t *testing.T, ctx context.Context, pool *pgxpool.Pool, projectID, createdBy string) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(ctx,
		`INSERT INTO agent_identities (project_id, name, created_by) VALUES ($1, $2, $3) RETURNING id`,
		projectID, "authz-agent-"+createdBy[:8], createdBy,
	).Scan(&id); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return id
}
