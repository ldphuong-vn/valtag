package admin

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Handler provides admin-only endpoints for system monitoring.
type Handler struct {
	pool *pgxpool.Pool
}

// NewHandler creates a new admin Handler.
func NewHandler(pool *pgxpool.Pool) *Handler {
	return &Handler{pool: pool}
}

// AdminMiddleware checks that the authenticated user has role = 'admin'.
func AdminMiddleware(pool *pgxpool.Pool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID := auth.UserIDFromContext(r.Context())
			if userID == "" {
				apierror.Unauthorized(w, "authentication required")
				return
			}
			var role string
			err := pool.QueryRow(r.Context(),
				`SELECT COALESCE(role, 'user') FROM users WHERE id = $1`, userID,
			).Scan(&role)
			if err != nil || role != "admin" {
				apierror.Forbidden(w, "admin access required")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// statsResponse holds the system overview numbers.
type statsResponse struct {
	TotalUsers         int64            `json:"total_users"`
	TotalOrgs          int64            `json:"total_orgs"`
	TotalSecrets       int64            `json:"total_secrets"`
	TotalAgents        int64            `json:"total_agents"`
	TotalRequestsToday int64            `json:"total_requests_today"`
	TotalRequestsAll   int64            `json:"total_requests_all"`
	UsersToday         int64            `json:"users_today"`
	PlanDistribution   map[string]int64 `json:"plan_distribution"`
}

// GetStats returns system-wide counts and plan distribution.
func (h *Handler) GetStats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var resp statsResponse
	resp.PlanDistribution = make(map[string]int64)

	// Aggregate counts in a single query for efficiency.
	row := h.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM organizations),
			(SELECT COUNT(*) FROM secrets),
			(SELECT COUNT(*) FROM agent_identities),
			(SELECT COUNT(*) FROM access_requests WHERE created_at >= CURRENT_DATE),
			(SELECT COUNT(*) FROM access_requests),
			(SELECT COUNT(*) FROM users WHERE created_at >= CURRENT_DATE)
	`)
	if err := row.Scan(
		&resp.TotalUsers,
		&resp.TotalOrgs,
		&resp.TotalSecrets,
		&resp.TotalAgents,
		&resp.TotalRequestsToday,
		&resp.TotalRequestsAll,
		&resp.UsersToday,
	); err != nil {
		apierror.InternalError(w, "failed to query stats")
		return
	}

	// Plan distribution.
	planRows, err := h.pool.Query(ctx, `
		SELECT COALESCE(plan, 'free'), COUNT(*) FROM organizations GROUP BY plan
	`)
	if err != nil {
		apierror.InternalError(w, "failed to query plan distribution")
		return
	}
	defer planRows.Close()
	for planRows.Next() {
		var plan string
		var count int64
		if err := planRows.Scan(&plan, &count); err == nil {
			resp.PlanDistribution[plan] = count
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

// userRow is the shape returned by ListUsers.
type userRow struct {
	ID            string `json:"id"`
	Email         string `json:"email"`
	Status        string `json:"status"`
	EmailVerified bool   `json:"email_verified"`
	TOTPEnabled   bool   `json:"totp_enabled"`
	Role          string `json:"role"`
	CreatedAt     string `json:"created_at"`
}

// ListUsers returns paginated users, optionally filtered by email search.
func (h *Handler) ListUsers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	search := r.URL.Query().Get("search")
	offset := (page - 1) * limit

	var (
		rows pgxRows
		err  error
	)
	if search != "" {
		rows, err = h.pool.Query(ctx, `
			SELECT id, email, COALESCE(status, 'active'), COALESCE(email_verified, false),
			       COALESCE(totp_enabled, false), COALESCE(role, 'user'), created_at
			FROM users
			WHERE email ILIKE '%' || $1 || '%'
			ORDER BY created_at DESC
			LIMIT $2 OFFSET $3
		`, search, limit, offset)
	} else {
		rows, err = h.pool.Query(ctx, `
			SELECT id, email, COALESCE(status, 'active'), COALESCE(email_verified, false),
			       COALESCE(totp_enabled, false), COALESCE(role, 'user'), created_at
			FROM users
			ORDER BY created_at DESC
			LIMIT $1 OFFSET $2
		`, limit, offset)
	}
	if err != nil {
		apierror.InternalError(w, "failed to query users")
		return
	}
	defer rows.Close()

	users := []userRow{}
	for rows.Next() {
		var u userRow
		var createdAt time.Time
		if err := rows.Scan(&u.ID, &u.Email, &u.Status, &u.EmailVerified, &u.TOTPEnabled, &u.Role, &createdAt); err != nil {
			continue
		}
		u.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		users = append(users, u)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"users": users,
		"page":  page,
		"limit": limit,
	})
}

// orgRow is the shape returned by ListOrgs.
type orgRow struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	OwnerID      string `json:"owner_id"`
	Plan         string `json:"plan"`
	MemberCount  int64  `json:"member_count"`
	SecretsCount int64  `json:"secrets_count"`
	CreatedAt    string `json:"created_at"`
}

// ListOrgs returns all organizations with member and secret counts.
func (h *Handler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	rows, err := h.pool.Query(ctx, `
		SELECT
			o.id, o.name, o.slug, o.owner_id::text, COALESCE(o.plan, 'free'),
			(SELECT COUNT(*) FROM org_memberships om WHERE om.org_id = o.id),
			(
				SELECT COUNT(*) FROM secrets s
				JOIN projects p ON s.project_id = p.id
				JOIN workspaces ws ON p.workspace_id = ws.id
				WHERE ws.org_id = o.id
			),
			o.created_at
		FROM organizations o
		ORDER BY o.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		apierror.InternalError(w, "failed to query orgs")
		return
	}
	defer rows.Close()

	orgs := []orgRow{}
	for rows.Next() {
		var o orgRow
		var createdAt time.Time
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.OwnerID, &o.Plan,
			&o.MemberCount, &o.SecretsCount, &createdAt); err != nil {
			continue
		}
		o.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		orgs = append(orgs, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"orgs":  orgs,
		"page":  page,
		"limit": limit,
	})
}

// paymentRow is the shape returned by ListPayments.
type paymentRow struct {
	OrgID                string `json:"org_id"`
	OrgName              string `json:"org_name"`
	Plan                 string `json:"plan"`
	StripeCustomerID     string `json:"stripe_customer_id"`
	StripeSubscriptionID string `json:"stripe_subscription_id"`
	PlanSeats            int64  `json:"plan_seats"`
}

// ListPayments returns orgs that have Stripe subscription data.
func (h *Handler) ListPayments(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.pool.Query(ctx, `
		SELECT
			id::text, name, COALESCE(plan, 'free'),
			COALESCE(stripe_customer_id, ''),
			COALESCE(stripe_subscription_id, ''),
			COALESCE(plan_seats, 0)
		FROM organizations
		WHERE stripe_customer_id IS NOT NULL OR stripe_subscription_id IS NOT NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		apierror.InternalError(w, "failed to query payments")
		return
	}
	defer rows.Close()

	payments := []paymentRow{}
	for rows.Next() {
		var p paymentRow
		if err := rows.Scan(&p.OrgID, &p.OrgName, &p.Plan,
			&p.StripeCustomerID, &p.StripeSubscriptionID, &p.PlanSeats); err != nil {
			continue
		}
		payments = append(payments, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"payments": payments})
}

// pgxRows is a minimal interface over pgx.Rows to allow multiple query signatures.
type pgxRows interface {
	Close()
	Next() bool
	Scan(dest ...interface{}) error
}
