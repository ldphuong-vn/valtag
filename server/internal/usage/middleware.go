package usage

import (
	"fmt"
	"net/http"

	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// EnforceLimits returns middleware that checks usage limits before resource creation.
// Only applied to POST routes (creation). getOrgID extracts org ID from request context/params.
func EnforceLimits(tracker *Tracker, metric string, getOrgID func(r *http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				next.ServeHTTP(w, r)
				return
			}
			orgID := getOrgID(r)
			if orgID == "" {
				next.ServeHTTP(w, r)
				return
			}
			allowed, current, limit, err := tracker.CheckLimit(r.Context(), orgID, metric)
			if err != nil || allowed {
				next.ServeHTTP(w, r)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("X-Usage-Current", fmt.Sprintf("%d", current))
			w.Header().Set("X-Usage-Limit", fmt.Sprintf("%d", limit))
			apierror.Forbidden(w, fmt.Sprintf("Free tier limit reached: %s (%d/%d). Upgrade to Pro.", metric, current, limit))
		})
	}
}
