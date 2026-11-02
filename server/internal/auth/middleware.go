package auth

import (
	"context"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

type contextKey string

const userIDKey contextKey = "userID"

// AuthMiddleware requires a valid user JWT. CLI tokens (typ=cli, 30 days)
// additionally have their ver claim checked against users.token_version, so
// revoke-all kills them immediately; access tokens stay DB-free.
// db may be nil only in unit tests that never present CLI tokens.
func AuthMiddleware(db *pgxpool.Pool, jwtMgr *JWTManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				apierror.Unauthorized(w, "missing authorization header")
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				apierror.Unauthorized(w, "invalid authorization format")
				return
			}

			userID, err := ValidateUserToken(r.Context(), db, jwtMgr, parts[1])
			if err != nil {
				apierror.Unauthorized(w, "invalid or expired token")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func UserIDFromContext(ctx context.Context) string {
	userID, _ := ctx.Value(userIDKey).(string)
	return userID
}

// WithUserID returns a context with the given user ID set.
func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}
