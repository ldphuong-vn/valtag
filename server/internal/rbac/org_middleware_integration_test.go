package rbac

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/valt-dev/valt/server/internal/auth"
	"github.com/valt-dev/valt/server/internal/database"
	"github.com/valt-dev/valt/server/internal/testutil"
)

// newRbacIntegrationDB mirrors the workflow integration helper: per-test
// schema, full migration application, advisory-locked.
func newRbacIntegrationDB(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for rbac integration tests")
	}

	schemaName := fmt.Sprintf("it_rbac_%d_%d", time.Now().UnixNano(), rand.Intn(100000))
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
	database.EnsurePartitions(ctx, pool)

	cleanup := func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", schemaName))
		adminPool.Close()
	}
	return pool, cleanup
}

// rbacFixture holds seeded IDs for the middleware tests.
type rbacFixture struct {
	orgID       string
	projectID   string
	agentID     string
	ownerID     string // org owner
	memberID    string // org member, project member
	viewerID    string // org viewer, project viewer
	strangerID  string // authenticated, no memberships
	weirdRoleID string // org membership with an unknown role
}

func seedRbacFixture(t *testing.T, ctx context.Context, db *pgxpool.Pool) *rbacFixture {
	t.Helper()

	insertUser := func(email string) string {
		var id string
		if err := db.QueryRow(ctx,
			`INSERT INTO users (email, password_hash) VALUES ($1, 'test-hash') RETURNING id`,
			email).Scan(&id); err != nil {
			t.Fatalf("seed user %s: %v", email, err)
		}
		return id
	}
	f := &rbacFixture{
		ownerID:     insertUser("owner@example.test"),
		memberID:    insertUser("member@example.test"),
		viewerID:    insertUser("viewer@example.test"),
		strangerID:  insertUser("stranger@example.test"),
		weirdRoleID: insertUser("weird@example.test"),
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	var orgID string
	if err := db.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, owner_id) VALUES ('RBAC Org', $1, $2) RETURNING id`,
		"rbac-"+suffix, f.ownerID).Scan(&orgID); err != nil {
		t.Fatalf("seed org: %v", err)
	}
	f.orgID = orgID

	insertMembership := func(userID, role string) {
		if _, err := db.Exec(ctx,
			`INSERT INTO org_memberships (org_id, user_id, role) VALUES ($1, $2, $3)`,
			f.orgID, userID, role); err != nil {
			t.Fatalf("seed membership %s: %v", role, err)
		}
	}
	insertMembership(f.ownerID, OrgOwner)
	insertMembership(f.memberID, OrgMember)
	insertMembership(f.viewerID, OrgViewer)
	insertMembership(f.weirdRoleID, "wizard") // unknown role must never pass

	var workspaceID string
	if err := db.QueryRow(ctx,
		`INSERT INTO workspaces (org_id, name, slug) VALUES ($1, 'ws', $2) RETURNING id`,
		f.orgID, "ws-"+suffix).Scan(&workspaceID); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if err := db.QueryRow(ctx,
		`INSERT INTO projects (workspace_id, name, slug) VALUES ($1, 'proj', $2) RETURNING id`,
		workspaceID, "proj-"+suffix).Scan(&f.projectID); err != nil {
		t.Fatalf("seed project: %v", err)
	}
	insertProjectMembership := func(userID, role string) {
		if _, err := db.Exec(ctx,
			`INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`,
			f.projectID, userID, role); err != nil {
			t.Fatalf("seed project membership %s: %v", role, err)
		}
	}
	insertProjectMembership(f.memberID, OrgMember)
	insertProjectMembership(f.viewerID, OrgViewer)

	if err := db.QueryRow(ctx,
		`INSERT INTO agent_identities (project_id, name, created_by) VALUES ($1, 'probe-agent', $2) RETURNING id`,
		f.projectID, f.ownerID).Scan(&f.agentID); err != nil {
		t.Fatalf("seed agent: %v", err)
	}
	return f
}

func getWithUser(h http.Handler, userID, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if userID != "" {
		req = req.WithContext(auth.WithUserID(req.Context(), userID))
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func orgProbeRouter(db *pgxpool.Pool, minRole string) http.Handler {
	r := chi.NewRouter()
	r.With(RequireOrgMembership(db, "org_id", minRole)).
		Get("/orgs/{org_id}/probe", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	return r
}

// TestRequireOrgMembership closes the BOLA family on org endpoints: every
// caller must be an org member with at least the required role.
func TestRequireOrgMembership(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newRbacIntegrationDB(t, ctx)
	defer cleanup()
	f := seedRbacFixture(t, ctx, db)
	path := "/orgs/" + f.orgID + "/probe"

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		if rec := getWithUser(orgProbeRouter(db, OrgViewer), "", path); rec.Code != http.StatusUnauthorized {
			t.Fatalf("got %d, want 401", rec.Code)
		}
	})
	t.Run("non-member is forbidden even at viewer level", func(t *testing.T) {
		if rec := getWithUser(orgProbeRouter(db, OrgViewer), f.strangerID, path); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})
	t.Run("unknown membership role never passes", func(t *testing.T) {
		if rec := getWithUser(orgProbeRouter(db, OrgViewer), f.weirdRoleID, path); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})
	t.Run("viewer passes viewer and fails member", func(t *testing.T) {
		if rec := getWithUser(orgProbeRouter(db, OrgViewer), f.viewerID, path); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		if rec := getWithUser(orgProbeRouter(db, OrgMember), f.viewerID, path); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})
	t.Run("member passes member and fails admin", func(t *testing.T) {
		if rec := getWithUser(orgProbeRouter(db, OrgMember), f.memberID, path); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
		if rec := getWithUser(orgProbeRouter(db, OrgAdmin), f.memberID, path); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})
	t.Run("owner passes admin", func(t *testing.T) {
		if rec := getWithUser(orgProbeRouter(db, OrgAdmin), f.ownerID, path); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
	})
}

// TestRequireAgentAccess proves agent-scoped routes resolve the agent's
// owning project and apply project RBAC — org membership alone is not enough.
func TestRequireAgentAccess(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newRbacIntegrationDB(t, ctx)
	defer cleanup()
	f := seedRbacFixture(t, ctx, db)

	agentRouter := func(action Action) http.Handler {
		r := chi.NewRouter()
		r.With(RequireAgentAccess(db, "agent_id", ResourceAgent, action)).
			Get("/agents/{agent_id}/probe", func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
		return r
	}
	readRouter := agentRouter(ActionRead)
	unknownPath := "/agents/00000000-0000-0000-0000-000000000000/probe"
	agentPath := "/agents/" + f.agentID + "/probe"

	t.Run("project member reads the agent", func(t *testing.T) {
		if rec := getWithUser(readRouter, f.memberID, agentPath); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200", rec.Code)
		}
	})
	t.Run("org member without project membership is forbidden", func(t *testing.T) {
		// ownerID belongs to the org but holds no project_memberships row.
		if rec := getWithUser(readRouter, f.ownerID, agentPath); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})
	t.Run("authenticated outsider is forbidden", func(t *testing.T) {
		if rec := getWithUser(readRouter, f.strangerID, agentPath); rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403", rec.Code)
		}
	})
	t.Run("unknown agent yields 404, not 403", func(t *testing.T) {
		if rec := getWithUser(readRouter, f.memberID, unknownPath); rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404", rec.Code)
		}
	})
	t.Run("write action requires the agent-write permission", func(t *testing.T) {
		if rec := getWithUser(agentRouter(ActionWrite), f.memberID, agentPath); rec.Code != http.StatusOK {
			t.Fatalf("project member: got %d, want 200", rec.Code)
		}
		if rec := getWithUser(agentRouter(ActionWrite), f.viewerID, agentPath); rec.Code != http.StatusForbidden {
			t.Fatalf("project viewer: got %d, want 403", rec.Code)
		}
	})
}

// TestUserMaxOrgRole backs the audit visibility policy.
func TestUserMaxOrgRole(t *testing.T) {
	ctx := context.Background()
	db, cleanup := newRbacIntegrationDB(t, ctx)
	defer cleanup()
	f := seedRbacFixture(t, ctx, db)

	assertMax := func(userID, want string) {
		t.Helper()
		got, err := UserMaxOrgRole(ctx, db, userID)
		if err != nil {
			t.Fatalf("UserMaxOrgRole: %v", err)
		}
		if got != want {
			t.Fatalf("got %q, want %q", got, want)
		}
	}

	assertMax(f.ownerID, OrgOwner)
	assertMax(f.memberID, OrgMember)
	assertMax(f.viewerID, OrgViewer)
	assertMax(f.weirdRoleID, "") // unknown role maps to nothing
	assertMax(f.strangerID, "")  // no membership at all

	// Promotion in a second org raises the maximum.
	var org2 string
	if err := db.QueryRow(ctx,
		`INSERT INTO organizations (name, slug, owner_id) VALUES ('RBAC Org 2', 'rbac-second-org', $1) RETURNING id`,
		f.ownerID).Scan(&org2); err != nil {
		t.Fatalf("seed second org: %v", err)
	}
	if _, err := db.Exec(ctx,
		`INSERT INTO org_memberships (org_id, user_id, role) VALUES ($1, $2, $3)`,
		org2, f.memberID, OrgAdmin); err != nil {
		t.Fatalf("seed second membership: %v", err)
	}
	assertMax(f.memberID, OrgAdmin)
}

// TestOrgRoleAtLeast is a pure unit check of the role ranking.
func TestOrgRoleAtLeast(t *testing.T) {
	cases := []struct {
		role, min string
		want      bool
	}{
		{OrgOwner, OrgViewer, true},
		{OrgOwner, OrgAdmin, true},
		{OrgAdmin, OrgAdmin, true},
		{OrgMember, OrgMember, true},
		{OrgMember, OrgAdmin, false},
		{OrgViewer, OrgMember, false},
		{"wizard", OrgViewer, false}, // unknown role denies everything
		{"", OrgViewer, false},
	}
	for _, tc := range cases {
		if got := OrgRoleAtLeast(tc.role, tc.min); got != tc.want {
			t.Fatalf("OrgRoleAtLeast(%q, %q) = %v, want %v", tc.role, tc.min, got, tc.want)
		}
	}
}
