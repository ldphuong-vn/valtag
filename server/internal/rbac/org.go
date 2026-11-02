package rbac

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/valt-dev/valt/server/internal/auth"
	"github.com/valt-dev/valt/server/pkg/apierror"
)

// Org membership roles, in ascending privilege order.
const (
	OrgViewer = "viewer"
	OrgMember = "member"
	OrgAdmin  = "admin"
	OrgOwner  = "owner"
)

// orgRoleRank maps an org role to its privilege rank; unknown roles rank
// below viewer so they never pass a check.
func orgRoleRank(role string) int {
	switch role {
	case OrgViewer:
		return 1
	case OrgMember:
		return 2
	case OrgAdmin:
		return 3
	case OrgOwner:
		return 4
	default:
		return 0
	}
}

// OrgRoleAtLeast reports whether the role grants at least minRole.
func OrgRoleAtLeast(role, minRole string) bool {
	return orgRoleRank(role) >= orgRoleRank(minRole)
}

// RequireOrgMembership returns middleware that requires the authenticated
// user to be a member of the org in the URL param orgParam with at least
// minRole ("viewer" < "member" < "admin" < "owner").
func RequireOrgMembership(db *pgxpool.Pool, orgParam, minRole string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.UserIDFromContext(r.Context())
			if userID == "" {
				apierror.Unauthorized(w, "authentication required")
				return
			}
			orgID := chi.URLParam(r, orgParam)
			if orgID == "" {
				apierror.BadRequest(w, orgParam+" is required")
				return
			}

			role, err := UserOrgRole(r.Context(), db, orgID, userID)
			if err != nil {
				apierror.InternalError(w, "failed to check org membership")
				return
			}
			if !OrgRoleAtLeast(role, minRole) {
				apierror.Forbidden(w, "insufficient org permissions")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
