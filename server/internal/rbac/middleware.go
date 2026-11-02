package rbac

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/valt-dev/valt/server/internal/auth"
	"github.com/valt-dev/valt/server/pkg/apierror"
)

// Middleware returns an HTTP middleware that checks if the authenticated user
// has the required permission for the project in the URL parameter.
// projectParam is the chi URL param name (e.g. "project_id").
func Middleware(db *pgxpool.Pool, projectParam string, resource Resource, action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.UserIDFromContext(r.Context())
			if userID == "" {
				apierror.Unauthorized(w, "authentication required")
				return
			}

			projectID := chi.URLParam(r, projectParam)
			if projectID == "" {
				apierror.BadRequest(w, "project_id is required")
				return
			}

			ok, err := userProjectAccess(r.Context(), db, userID, projectID, resource, action)
			if err != nil {
				apierror.InternalError(w, "failed to check project access")
				return
			}
			if !ok {
				apierror.Forbidden(w, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequireAgentAccess returns middleware that resolves the agent in the URL
// param agentParam to its owning project and requires the caller to hold
// resource/action on that project. Unknown agents yield 404 so callers cannot
// probe agent existence.
func RequireAgentAccess(db *pgxpool.Pool, agentParam string, resource Resource, action Action) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.UserIDFromContext(r.Context())
			if userID == "" {
				apierror.Unauthorized(w, "authentication required")
				return
			}

			agentID := chi.URLParam(r, agentParam)
			if agentID == "" {
				apierror.BadRequest(w, agentParam+" is required")
				return
			}

			projectID, err := AgentProject(r.Context(), db, agentID)
			if err != nil {
				apierror.InternalError(w, "failed to resolve agent")
				return
			}
			if projectID == "" {
				apierror.NotFound(w, "agent not found")
				return
			}

			ok, err := userProjectAccess(r.Context(), db, userID, projectID, resource, action)
			if err != nil {
				apierror.InternalError(w, "failed to check project access")
				return
			}
			if !ok {
				apierror.Forbidden(w, "insufficient permissions")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
