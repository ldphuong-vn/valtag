package gateway

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ProxyRoute defines a credential injection rule for an agent+endpoint pair.
type ProxyRoute struct {
	ID              string    `json:"id"`
	AgentID         string    `json:"agent_id"`
	HostPattern     string    `json:"host_pattern"`
	PathPattern     string    `json:"path_pattern"`
	SecretID        string    `json:"secret_id"`
	InjectionType   string    `json:"injection_type"`
	InjectionKey    string    `json:"injection_key"`
	InjectionFormat string    `json:"injection_format"`
	PlaceholderKey  string    `json:"placeholder_key"`
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// EndpointLimit defines per-agent per-endpoint rate limits.
type EndpointLimit struct {
	ID          string    `json:"id"`
	AgentID     string    `json:"agent_id"`
	HostPattern string    `json:"host_pattern"`
	PathPattern string    `json:"path_pattern"`
	RPM         int       `json:"rpm"`
	Blocked     bool      `json:"blocked"`
	CreatedAt   time.Time `json:"created_at"`
}

// Store handles DB queries for proxy routes and endpoint limits.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore creates a new gateway Store.
func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

// GeneratePlaceholder creates a unique placeholder key.
func GeneratePlaceholder() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generating placeholder: %w", err)
	}
	return "valtag_pk_" + base64.RawURLEncoding.EncodeToString(b), nil
}

// FindMatchingRoute finds an enabled route for an agent that matches host+path.
func (s *Store) FindMatchingRoute(ctx context.Context, agentID, host, path string) (*ProxyRoute, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, host_pattern, path_pattern, secret_id,
		        injection_type, injection_key, injection_format, COALESCE(placeholder_key, ''), enabled,
		        created_at, updated_at
		 FROM proxy_routes
		 WHERE agent_id = $1 AND enabled = true
		 ORDER BY length(path_pattern) DESC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying proxy routes: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var r ProxyRoute
		if err := rows.Scan(&r.ID, &r.AgentID, &r.HostPattern, &r.PathPattern,
			&r.SecretID, &r.InjectionType, &r.InjectionKey, &r.InjectionFormat,
			&r.PlaceholderKey, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning route: %w", err)
		}
		if r.HostPattern == host && MatchPath(r.PathPattern, path) {
			return &r, nil
		}
	}
	return nil, nil
}

// FindByPlaceholder looks up a route by its placeholder key.
func (s *Store) FindByPlaceholder(ctx context.Context, placeholder string) (*ProxyRoute, error) {
	var r ProxyRoute
	err := s.pool.QueryRow(ctx,
		`SELECT id, agent_id, host_pattern, path_pattern, secret_id,
		        injection_type, injection_key, injection_format, placeholder_key, enabled,
		        created_at, updated_at
		 FROM proxy_routes
		 WHERE placeholder_key = $1 AND enabled = true`,
		placeholder,
	).Scan(&r.ID, &r.AgentID, &r.HostPattern, &r.PathPattern,
		&r.SecretID, &r.InjectionType, &r.InjectionKey, &r.InjectionFormat,
		&r.PlaceholderKey, &r.Enabled, &r.CreatedAt, &r.UpdatedAt)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding route by placeholder: %w", err)
	}
	return &r, nil
}

// ListRoutes returns all proxy routes for an agent.
func (s *Store) ListRoutes(ctx context.Context, agentID string) ([]ProxyRoute, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, host_pattern, path_pattern, secret_id,
		        injection_type, injection_key, injection_format, COALESCE(placeholder_key, ''), enabled,
		        created_at, updated_at
		 FROM proxy_routes WHERE agent_id = $1
		 ORDER BY created_at DESC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing routes: %w", err)
	}
	defer rows.Close()

	var routes []ProxyRoute
	for rows.Next() {
		var r ProxyRoute
		if err := rows.Scan(&r.ID, &r.AgentID, &r.HostPattern, &r.PathPattern,
			&r.SecretID, &r.InjectionType, &r.InjectionKey, &r.InjectionFormat,
			&r.PlaceholderKey, &r.Enabled, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scanning route: %w", err)
		}
		routes = append(routes, r)
	}
	if routes == nil {
		routes = []ProxyRoute{}
	}
	return routes, nil
}

// CreateRoute inserts a new proxy route with auto-generated placeholder.
func (s *Store) CreateRoute(ctx context.Context, r *ProxyRoute) error {
	if r.PlaceholderKey == "" {
		pk, err := GeneratePlaceholder()
		if err != nil {
			return err
		}
		r.PlaceholderKey = pk
	}
	return s.pool.QueryRow(ctx,
		`INSERT INTO proxy_routes (agent_id, host_pattern, path_pattern, secret_id,
		        injection_type, injection_key, injection_format, placeholder_key, enabled)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		 RETURNING id, created_at, updated_at`,
		r.AgentID, r.HostPattern, r.PathPattern, r.SecretID,
		r.InjectionType, r.InjectionKey, r.InjectionFormat, r.PlaceholderKey, r.Enabled,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
}

// UpdateRoute updates an existing proxy route.
func (s *Store) UpdateRoute(ctx context.Context, id string, r *ProxyRoute) error {
	tag, err := s.pool.Exec(ctx,
		`UPDATE proxy_routes SET host_pattern = $1, path_pattern = $2, secret_id = $3,
		        injection_type = $4, injection_key = $5, injection_format = $6,
		        enabled = $7, updated_at = NOW()
		 WHERE id = $8`,
		r.HostPattern, r.PathPattern, r.SecretID,
		r.InjectionType, r.InjectionKey, r.InjectionFormat,
		r.Enabled, id,
	)
	if err != nil {
		return fmt.Errorf("updating route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// DeleteRoute removes a proxy route.
func (s *Store) DeleteRoute(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM proxy_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting route: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// FindEndpointLimit finds a matching endpoint limit for an agent+host+path.
func (s *Store) FindEndpointLimit(ctx context.Context, agentID, host, path string) (*EndpointLimit, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, host_pattern, path_pattern, rpm, blocked, created_at
		 FROM proxy_endpoint_limits WHERE agent_id = $1`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("querying endpoint limits: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var l EndpointLimit
		if err := rows.Scan(&l.ID, &l.AgentID, &l.HostPattern, &l.PathPattern,
			&l.RPM, &l.Blocked, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning limit: %w", err)
		}
		if l.HostPattern == host && MatchPath(l.PathPattern, path) {
			return &l, nil
		}
	}
	return nil, nil
}

// ListEndpointLimits returns all endpoint limits for an agent.
func (s *Store) ListEndpointLimits(ctx context.Context, agentID string) ([]EndpointLimit, error) {
	rows, err := s.pool.Query(ctx,
		`SELECT id, agent_id, host_pattern, path_pattern, rpm, blocked, created_at
		 FROM proxy_endpoint_limits WHERE agent_id = $1
		 ORDER BY created_at DESC`,
		agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("listing limits: %w", err)
	}
	defer rows.Close()

	var limits []EndpointLimit
	for rows.Next() {
		var l EndpointLimit
		if err := rows.Scan(&l.ID, &l.AgentID, &l.HostPattern, &l.PathPattern,
			&l.RPM, &l.Blocked, &l.CreatedAt); err != nil {
			return nil, fmt.Errorf("scanning limit: %w", err)
		}
		limits = append(limits, l)
	}
	if limits == nil {
		limits = []EndpointLimit{}
	}
	return limits, nil
}

// CreateEndpointLimit inserts a new endpoint limit.
func (s *Store) CreateEndpointLimit(ctx context.Context, l *EndpointLimit) error {
	return s.pool.QueryRow(ctx,
		`INSERT INTO proxy_endpoint_limits (agent_id, host_pattern, path_pattern, rpm, blocked)
		 VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at`,
		l.AgentID, l.HostPattern, l.PathPattern, l.RPM, l.Blocked,
	).Scan(&l.ID, &l.CreatedAt)
}

// DeleteEndpointLimit removes an endpoint limit.
func (s *Store) DeleteEndpointLimit(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM proxy_endpoint_limits WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("deleting limit: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}
