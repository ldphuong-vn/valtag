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
// orgs ("" when the user belongs to no org). Ranking is numeric — MAX over
// role names would compare text ('member' > 'admin'), so the query maps
// roles to ranks first. Used for visibility policies where audit rows are
// not org-scoped yet.
func UserMaxOrgRole(ctx context.Context, db *pgxpool.Pool, userID string) (string, error) {
	var rank int
	err := db.QueryRow(ctx,
		`SELECT COALESCE(MAX(CASE role WHEN 'owner' THEN 4 WHEN 'admin' THEN 3 WHEN 'member' THEN 2 WHEN 'viewer' THEN 1 ELSE 0 END), 0) FROM org_memberships WHERE user_id = @user_id`,
		pgx.NamedArgs{"user_id": userID},
	).Scan(&rank)
	if err != nil {
		return "", err
	}
	switch rank {
	case 4:
		return OrgOwner, nil
	case 3:
		return OrgAdmin, nil
	case 2:
		return OrgMember, nil
	case 1:
		return OrgViewer, nil
	default:
		return "", nil
	}
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
