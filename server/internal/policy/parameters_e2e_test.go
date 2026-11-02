package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/valt-dev/valt/server/internal/auth"
)

// E2E Tests for Parameter-Based Policy Enforcement

const (
	e2eOwnerID     = "00000000-0000-0000-0000-000000e2e101"
	e2eAdminID     = "00000000-0000-0000-0000-000000e2e102"
	e2eMemberID    = "00000000-0000-0000-0000-000000e2e103"
	e2eViewerID    = "00000000-0000-0000-0000-000000e2e104"
	e2eOrgID       = "00000000-0000-0000-0000-000000e2e201"
	e2eWorkspaceID = "00000000-0000-0000-0000-000000e2e301"
	e2eProjectID   = "00000000-0000-0000-0000-000000e2e401"
	e2eSecretID    = "00000000-0000-0000-0000-000000e2e501"
	e2eAgentID     = "00000000-0000-0000-0000-000000e2e601"
)

// TestE2EParameterValidation validates all parameter constraints
func TestE2EParameterValidation(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		params      map[string]any
		shouldError bool
		errorMsg    string
	}{
		{
			name: "valid minimal parameters",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: false,
		},
		{
			name: "valid strict parameters",
			params: map[string]any{
				"max_duration_minutes": 120,
				"require_approval":     true,
				"allow_auto_approve":   false,
				"require_reason":       true,
				"min_reason_length":    50,
				"max_requests_per_day": 10,
				"cool_down_minutes":    30,
				"single_use":           true,
				"notify_on_access":     true,
				"require_consent":      true,
			},
			shouldError: false,
		},
		{
			name: "invalid max_duration_minutes too low",
			params: map[string]any{
				"max_duration_minutes": 0,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "out of range",
		},
		{
			name: "invalid max_duration_minutes too high",
			params: map[string]any{
				"max_duration_minutes": 1441,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "out of range",
		},
		{
			name: "invalid min_reason_length too high",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       true,
				"min_reason_length":    501,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "out of range",
		},
		{
			name: "invalid max_requests_per_day too low",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 0,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "out of range",
		},
		{
			name: "invalid max_requests_per_day too high",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 1001,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "out of range",
		},
		{
			name: "invalid cool_down_minutes too high",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    1441,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "out of range",
		},
		{
			name: "invalid allow_auto_approve with require_approval",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     true,
				"allow_auto_approve":   true,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "cannot be true when require_approval is true",
		},
		{
			name: "invalid single_use with high duration",
			params: map[string]any{
				"max_duration_minutes": 241,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           true,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "requires max_duration_minutes <= 240",
		},
		{
			name: "invalid unknown key",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
				"unknown_key":          "value",
			},
			shouldError: true,
			errorMsg:    "unknown policy key",
		},
		{
			name: "invalid type for max_duration_minutes (string)",
			params: map[string]any{
				"max_duration_minutes": "60",
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "invalid type",
		},
		{
			name: "invalid type for require_approval (string)",
			params: map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     "true",
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "invalid type",
		},
		{
			name: "missing required key max_duration_minutes",
			params: map[string]any{
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			},
			shouldError: true,
			errorMsg:    "missing policy key",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params, err := ValidateParameters(tc.params)

			if tc.shouldError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tc.errorMsg) {
					t.Errorf("expected error containing %q, got %q", tc.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if raw, ok := tc.params["max_duration_minutes"].(int); ok && params.MaxDurationMinutes == 0 && raw != 0 {
					t.Errorf("parameter not set correctly")
				}
			}
		})
	}
}

// TestE2EParameterBoundaryConditions tests edge cases for all parameters
func TestE2EParameterBoundaryConditions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name      string
		field     string
		value     any
		valid     bool
		overrides map[string]any
	}{
		// max_duration_minutes boundaries
		{"min_duration_1", "max_duration_minutes", 1, true, nil},
		{"min_duration_0", "max_duration_minutes", 0, false, nil},
		{"max_duration_1440", "max_duration_minutes", 1440, true, nil},
		{"max_duration_1441", "max_duration_minutes", 1441, false, nil},

		// min_reason_length boundaries
		{"min_reason_0", "min_reason_length", 0, true, map[string]any{"require_reason": true}},
		{"min_reason_500", "min_reason_length", 500, true, map[string]any{"require_reason": true}},
		{"min_reason_501", "min_reason_length", 501, false, map[string]any{"require_reason": true}},

		// max_requests_per_day boundaries
		{"max_req_1", "max_requests_per_day", 1, true, nil},
		{"max_req_0", "max_requests_per_day", 0, false, nil},
		{"max_req_1000", "max_requests_per_day", 1000, true, nil},
		{"max_req_1001", "max_requests_per_day", 1001, false, nil},

		// cool_down_minutes boundaries
		{"cooldown_0", "cool_down_minutes", 0, true, nil},
		{"cooldown_1440", "cool_down_minutes", 1440, true, nil},
		{"cooldown_1441", "cool_down_minutes", 1441, false, nil},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{
				"max_duration_minutes": 60,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			}

			// Set the test value
			params[tc.field] = tc.value

			// Apply overrides
			if tc.overrides != nil {
				for k, v := range tc.overrides {
					params[k] = v
				}
			}

			_, err := ValidateParameters(params)
			if tc.valid && err != nil {
				t.Errorf("expected valid but got error: %v", err)
			}
			if !tc.valid && err == nil {
				t.Errorf("expected invalid but got success")
			}
		})
	}
}

// TestE2EParameterTypeCoercion tests type handling for numeric fields
func TestE2EParameterTypeCoercion(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		value   any
		valid   bool
		message string
	}{
		{"int32_valid", int32(60), true, ""},
		{"int64_valid", int64(60), true, ""},
		{"float64_valid", float64(60), true, ""},
		{"float64_non_integer", float64(60.5), false, "expected integer"},
		{"string_invalid", "60", false, "expected int"},
		{"bool_invalid", true, false, "expected int"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{
				"max_duration_minutes": tc.value,
				"require_approval":     false,
				"allow_auto_approve":   false,
				"require_reason":       false,
				"min_reason_length":    0,
				"max_requests_per_day": 100,
				"cool_down_minutes":    0,
				"single_use":           false,
				"notify_on_access":     false,
				"require_consent":      false,
			}

			_, err := ValidateParameters(params)
			if tc.valid && err != nil {
				t.Errorf("expected valid but got error: %v", err)
			}
			if !tc.valid {
				if err == nil {
					t.Errorf("expected invalid but got success")
				}
				if tc.message != "" && !strings.Contains(err.Error(), tc.message) {
					t.Errorf("expected error containing %q, got %q", tc.message, err.Error())
				}
			}
		})
	}
}

// TestE2EParameterIntegrationHTTP tests parameter enforcement via HTTP API
func TestE2EParameterIntegrationHTTP(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	pool, cleanup := newE2EIntegrationDB(t, ctx)
	defer cleanup()

	seedE2EData(t, ctx, pool)

	svc := NewService(pool)
	h := NewHandler(svc)
	r := chi.NewRouter()
	h.RegisterRoutes(r)

	t.Run("create_valid_template", func(t *testing.T) {
		params := map[string]any{
			"max_duration_minutes": 90,
			"require_approval":     true,
			"allow_auto_approve":   false,
			"require_reason":       true,
			"min_reason_length":    30,
			"max_requests_per_day": 50,
			"cool_down_minutes":    15,
			"single_use":           false,
			"notify_on_access":     true,
			"require_consent":      false,
		}

		resp := doE2ERequest(t, r, http.MethodPost,
			fmt.Sprintf("/projects/%s/policy-templates", e2eProjectID),
			e2eOwnerID, map[string]any{
				"name":        "E2E Test Template",
				"description": "Template for e2e testing",
				"parameters":  params,
			})

		if resp.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d: %s", resp.Code, resp.Body.String())
		}

		var tpl Template
		if err := json.Unmarshal(resp.Body.Bytes(), &tpl); err != nil {
			t.Fatalf("failed to decode template: %v", err)
		}

		if tpl.Name != "E2E Test Template" {
			t.Errorf("template name mismatch: got %q", tpl.Name)
		}
	})

	t.Run("create_invalid_template_auto_approve_conflict", func(t *testing.T) {
		params := map[string]any{
			"max_duration_minutes": 60,
			"require_approval":     true,
			"allow_auto_approve":   true,
			"require_reason":       false,
			"min_reason_length":    0,
			"max_requests_per_day": 100,
			"cool_down_minutes":    0,
			"single_use":           false,
			"notify_on_access":     false,
			"require_consent":      false,
		}

		resp := doE2ERequest(t, r, http.MethodPost,
			fmt.Sprintf("/projects/%s/policy-templates", e2eProjectID),
			e2eOwnerID, map[string]any{
				"name":        "Invalid Auto Approve",
				"description": "Should fail",
				"parameters":  params,
			})

		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", resp.Code, resp.Body.String())
		}
	})

	t.Run("rbac_member_cannot_create_template", func(t *testing.T) {
		params := map[string]any{
			"max_duration_minutes": 60,
			"require_approval":     false,
			"allow_auto_approve":   false,
			"require_reason":       false,
			"min_reason_length":    0,
			"max_requests_per_day": 100,
			"cool_down_minutes":    0,
			"single_use":           false,
			"notify_on_access":     false,
			"require_consent":      false,
		}

		resp := doE2ERequest(t, r, http.MethodPost,
			fmt.Sprintf("/projects/%s/policy-templates", e2eProjectID),
			e2eMemberID, map[string]any{
				"name":        "Member Template",
				"description": "Should fail",
				"parameters":  params,
			})

		if resp.Code != http.StatusForbidden {
			t.Fatalf("expected 403, got %d: %s", resp.Code, resp.Body.String())
		}
	})
}

// E2E HTTP test helpers

func doE2ERequest(t *testing.T, handler http.Handler, method, path, userID string, body any) *httptest.ResponseRecorder {
	t.Helper()

	var payload []byte
	if body != nil {
		var err error
		payload, err = json.Marshal(body)
		if err != nil {
			t.Fatalf("marshal request body failed: %v", err)
		}
	}

	req := httptest.NewRequest(method, path, bytes.NewReader(payload))
	req = req.WithContext(auth.WithUserID(req.Context(), userID))
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// Database setup helpers

func seedE2EData(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()

	stmts := []struct {
		query string
		args  []any
	}{
		// Users
		{query: `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, args: []any{e2eOwnerID, "owner@example.com", "hash"}},
		{query: `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, args: []any{e2eAdminID, "admin@example.com", "hash"}},
		{query: `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, args: []any{e2eMemberID, "member@example.com", "hash"}},
		{query: `INSERT INTO users (id, email, password_hash) VALUES ($1, $2, $3)`, args: []any{e2eViewerID, "viewer@example.com", "hash"}},

		// Org & Workspace & Project
		{query: `INSERT INTO organizations (id, name, slug, owner_id, plan) VALUES ($1, $2, $3, $4, $5)`, args: []any{e2eOrgID, "E2E Org", "e2e-org", e2eOwnerID, "pro"}},
		{query: `INSERT INTO workspaces (id, org_id, name, slug) VALUES ($1, $2, $3, $4)`, args: []any{e2eWorkspaceID, e2eOrgID, "E2E Workspace", "e2e-workspace"}},
		{query: `INSERT INTO projects (id, workspace_id, name, slug) VALUES ($1, $2, $3, $4)`, args: []any{e2eProjectID, e2eWorkspaceID, "E2E Project", "e2e-project"}},

		// Project Memberships
		{query: `INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`, args: []any{e2eProjectID, e2eOwnerID, "owner"}},
		{query: `INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`, args: []any{e2eProjectID, e2eAdminID, "admin"}},
		{query: `INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`, args: []any{e2eProjectID, e2eMemberID, "member"}},
		{query: `INSERT INTO project_memberships (project_id, user_id, role) VALUES ($1, $2, $3)`, args: []any{e2eProjectID, e2eViewerID, "viewer"}},

		// Secret
		{query: `INSERT INTO secrets (id, user_id, name, description, storage_key, encrypted_dek, policy, credential_type, source, version, project_id) VALUES ($1,$2,$3,$4,$5,$6,$7::jsonb,$8,$9,$10,$11)`, args: []any{e2eSecretID, e2eOwnerID, "e2e-secret", "", "k1", []byte{1, 2, 3}, `{}`, "api_key", "", 1, e2eProjectID}},

		// Agent Identity
		{query: `INSERT INTO agent_identities (id, project_id, name, description, agent_type, auth_method, allowed_scopes, max_session_ttl, status, created_by) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, args: []any{e2eAgentID, e2eProjectID, "e2e-agent", "", "custom", "token", []string{"read"}, 3600, "active", e2eOwnerID}},
	}

	for _, stmt := range stmts {
		if _, err := pool.Exec(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("seed statement failed: %v\nquery=%s", err, stmt.query)
		}
	}
}

func newE2EIntegrationDB(t *testing.T, ctx context.Context) (*pgxpool.Pool, func()) {
	t.Helper()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL is required for e2e integration tests")
	}

	schemaName := fmt.Sprintf("e2e_policy_%d_%d", time.Now().UnixNano(), rand.Intn(100000))

	adminPool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("create admin pool failed: %v", err)
	}

	if _, err := adminPool.Exec(ctx, fmt.Sprintf("CREATE SCHEMA %s", schemaName)); err != nil {
		adminPool.Close()
		t.Skipf("cannot create test schema (need CREATE privilege): %v", err)
	}

	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		adminPool.Close()
		t.Fatalf("parse database url failed: %v", err)
	}
	if cfg.ConnConfig.RuntimeParams == nil {
		cfg.ConnConfig.RuntimeParams = map[string]string{}
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schemaName

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		adminPool.Close()
		t.Fatalf("create schema pool failed: %v", err)
	}

	if err := applyAllMigrations(ctx, pool); err != nil {
		pool.Close()
		_, _ = adminPool.Exec(ctx, fmt.Sprintf("DROP SCHEMA %s CASCADE", schemaName))
		adminPool.Close()
		t.Fatalf("apply migrations failed: %v", err)
	}

	cleanup := func() {
		pool.Close()
		_, _ = adminPool.Exec(context.Background(), fmt.Sprintf("DROP SCHEMA %s CASCADE", schemaName))
		adminPool.Close()
	}

	return pool, cleanup
}
