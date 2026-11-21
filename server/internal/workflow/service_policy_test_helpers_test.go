package workflow

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	ownerID     = "00000000-0000-0000-0000-000000000101"
	requesterID = "00000000-0000-0000-0000-000000000102"
	projectID   = "00000000-0000-0000-0000-000000000201"
	secretID    = "00000000-0000-0000-0000-000000000301"
)

func seedWorkflowPolicyData(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	stmts := []struct {
		query string
		args  []any
	}{
		{query: `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, args: []any{ownerID, "owner@example.com", "hash"}},
		{query: `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, args: []any{requesterID, "requester@example.com", "hash"}},
		{query: `INSERT INTO organizations (id, name, slug, owner_id) VALUES ($1, $2, $3, $4)`, args: []any{"00000000-0000-0000-0000-000000000401", "Org", "org-workflow", ownerID}},
		{query: `INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`, args: []any{"00000000-0000-0000-0000-000000000501", "00000000-0000-0000-0000-000000000401", "Workspace", "workspace-workflow"}},
		{query: `INSERT INTO projects (id, workspace_id, name, slug) VALUES ($1, $2, $3, $4)`, args: []any{projectID, "00000000-0000-0000-0000-000000000501", "Project", "project-workflow"}},
		{query: `INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`, args: []any{projectID, ownerID, "owner"}},
		{query: `INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`, args: []any{projectID, requesterID, "member"}},
		{query: `INSERT INTO secrets (id, user_id, name, description, storage_key, encrypted_dek, policy, credential_type, source, version, project_id) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11)`, args: []any{secretID, ownerID, "api-token", "", "k1", []byte{1, 2, 3}, `{}`, "api_key", "", 1, projectID}},
	}
	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("seed statement failed: %v\nquery=%s", err, stmt.query)
		}
	}
}

func containsPolicyWarning(warnings []string, expected string) bool {
	for _, warning := range warnings {
		if warning == expected {
			return true
		}
	}
	return false
}

func strPtr(v string) *string {
	return &v
}
