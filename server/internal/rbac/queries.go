package rbac

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This file holds the SQL layer of the rbac package: static query strings
// with pgx named parameters (@name) — no request-derived value is ever
// interpolated into SQL text. Request-derived values reach these functions
// only as named query arguments.

// userProjectAccess reports whether the user's project-membership role
// grants the resource/action pair.
func userProjectAccess(ctx context.Context, db *pgxpool.Pool, userID, projectID string, resource Resource, action Action) (bool, error) {
	var role string
	err := db.QueryRow(ctx,
		`SELECT role FROM project_memberships WHERE project_id = @project_id AND user_id = @user_id`,
		pgx.NamedArgs{"project_id": projectID, "user_id": userID},
	).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return false, nil
		}
		return false, err
	}
	return Can(RoleFromProjectMembership(role), resource, action), nil
}

// UserOrgRole returns the caller's role in the given org ("" when the user
// is not a member).
func UserOrgRole(ctx context.Context, db *pgxpool.Pool, orgID, userID string) (string, error) {
	var role string
	err := db.QueryRow(ctx,
		`SELECT role FROM org_memberships WHERE org_id = @org_id AND user_id = @user_id`,
		pgx.NamedArgs{"org_id": orgID, "user_id": userID},
	).Scan(&role)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return role, nil
}

// UserMaxOrgRole returns the strongest org role the user holds across all
// orgs ("" when the user belongs to no org). Used for visibility policies
// where audit rows are not org-scoped yet.
func UserMaxOrgRole(ctx context.Context, db *pgxpool.Pool, userID string) (string, error) {
	var role *string
	err := db.QueryRow(ctx,
		`SELECT MAX(CASE role WHEN 'owner' THEN 'owner' WHEN 'admin' THEN 'admin' WHEN 'member' THEN 'member' WHEN 'viewer' THEN 'viewer' END) FROM org_memberships WHERE user_id = @user_id`,
		pgx.NamedArgs{"user_id": userID},
	).Scan(&role)
	if err != nil {
		return "", err
	}
	if role == nil {
		return "", nil
	}
	return *role, nil
}

// AgentProject returns the project that owns the agent identity ("" when the
// agent does not exist).
func AgentProject(ctx context.Context, db *pgxpool.Pool, agentID string) (string, error) {
	var projectID string
	err := db.QueryRow(ctx,
		`SELECT project_id FROM agent_identities WHERE id = @agent_id`,
		pgx.NamedArgs{"agent_id": agentID},
	).Scan(&projectID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	return projectID, nil
}
