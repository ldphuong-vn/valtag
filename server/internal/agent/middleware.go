package agent

import (
	"context"
	"net/http"
	"strings"

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

type contextKey string

const agentIDKey contextKey = "agent_id"

// AgentIDFromContext retrieves the agent ID stored by AuthMiddleware.
func AgentIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(agentIDKey).(string)
	return v
}

// WithAgentID returns a context with the given agent ID set.
func WithAgentID(ctx context.Context, agentID string) context.Context {
	return context.WithValue(ctx, agentIDKey, agentID)
}

// AuthMiddleware validates agent Bearer tokens and stores the agent ID in context.
func AuthMiddleware(svc *Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
				apierror.Unauthorized(w, "missing agent token")
				return
			}
			plainToken := strings.TrimPrefix(authHeader, "Bearer ")
			token, err := svc.ValidateToken(r.Context(), plainToken)
			if err != nil || token == nil {
				apierror.Unauthorized(w, "invalid or expired agent token")
				return
			}
			ctx := context.WithValue(r.Context(), agentIDKey, token.AgentID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
