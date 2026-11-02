package workflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/ldphuong-vn/valtag/server/internal/policy"
)

func TestCreateRequest_PolicyV2BindingOverridesDefaults(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, cleanup := newWorkflowIntegrationDB(t, ctx)
	defer cleanup()
	seedWorkflowPolicyData(t, ctx, pool)

	ps := policy.NewService(pool)
	params := map[string]any{
		"max_duration_minutes": 30,
		"require_approval":     true,
		"allow_auto_approve":   false,
		"require_reason":       true,
		"min_reason_length":    20,
		"max_requests_per_day": 100,
		"cool_down_minutes":    0,
		"single_use":           true,
		"notify_on_access":     true,
		"require_consent":      false,
	}
	tpl, err := ps.CreateTemplate(ctx, projectID, ownerID, "Workflow Policy", "", strPtr("api_key"), params)
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}
	b, err := ps.UpdateBinding(ctx, secretID, ownerID, tpl.ID, tpl.CurrentVersion, map[string]any{"max_duration_minutes": 45})
	if err != nil {
		t.Fatalf("update binding failed: %v", err)
	}
	if b.TemplateVersion == nil {
		t.Fatal("binding template version must be set")
	}

	svc := NewService(pool, true)
	reason := "this reason is long enough"
	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		SecretID:        secretID,
		RequesterUserID: requesterID,
		RequesterType:   "human",
		Reason:          reason,
		DurationMinutes: 120,
		CredentialType:  "api_key",
	})
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if req.Status != "pending" {
		t.Fatalf("expected pending from binding policy, got %s", req.Status)
	}
	if req.RequestedDurationMinutes != 45 {
		t.Fatalf("expected capped duration 45 from override, got %d", req.RequestedDurationMinutes)
	}

	var source string
	var templateID *string
	var templateVersion *int
	var appliedRaw []byte
	var warningsRaw []byte
	err = pool.QueryRow(ctx, `
		SELECT applied_policy_source, applied_template_id, applied_template_version, applied_policy, applied_policy_warnings
		FROM access_requests WHERE id = $1`, req.ID).Scan(&source, &templateID, &templateVersion, &appliedRaw, &warningsRaw)
	if err != nil {
		t.Fatalf("load applied snapshot failed: %v", err)
	}
	if source != policy.PolicySourceTemplateOverride {
		t.Fatalf("expected source template+override, got %s", source)
	}
	if templateID == nil || *templateID != tpl.ID {
		t.Fatalf("unexpected applied template id: %v", templateID)
	}
	if templateVersion == nil || *templateVersion != *b.TemplateVersion {
		t.Fatalf("unexpected applied template version: %v", templateVersion)
	}
	var applied policy.PolicyParameters
	if err := json.Unmarshal(appliedRaw, &applied); err != nil {
		t.Fatalf("decode applied policy failed: %v", err)
	}
	if applied.MaxDurationMinutes != 45 || !applied.RequireApproval || !applied.SingleUse {
		t.Fatalf("unexpected applied policy: %+v", applied)
	}
	var warnings []string
	if err := json.Unmarshal(warningsRaw, &warnings); err != nil {
		t.Fatalf("decode warnings failed: %v", err)
	}
	if !containsPolicyWarning(warnings, "weaker:max_duration_minutes") {
		t.Fatalf("expected weaker warning, got %v", warnings)
	}
}

func TestCreateRequest_FeatureFlagOffUsesDefaultParity(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, cleanup := newWorkflowIntegrationDB(t, ctx)
	defer cleanup()
	seedWorkflowPolicyData(t, ctx, pool)

	svc := NewService(pool, false)
	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		SecretID:        secretID,
		RequesterUserID: requesterID,
		RequesterType:   "human",
		Reason:          "",
		DurationMinutes: 999,
		CredentialType:  "api_key",
	})
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}
	if req.Status != "approved" {
		t.Fatalf("expected approved for api_key default, got %s", req.Status)
	}
	if req.RequestedDurationMinutes != policy.ForCredentialType("api_key").MaxDurationMinutes {
		t.Fatalf("expected default capped duration, got %d", req.RequestedDurationMinutes)
	}

	var source string
	err = pool.QueryRow(ctx, `SELECT applied_policy_source FROM access_requests WHERE id = $1`, req.ID).Scan(&source)
	if err != nil {
		t.Fatalf("query snapshot source failed: %v", err)
	}
	if source != policy.PolicySourceDefault {
		t.Fatalf("expected default source when feature disabled, got %s", source)
	}
}

func TestCreateRequest_PolicyV2ResolutionFailureFallsBackDefault(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, cleanup := newWorkflowIntegrationDB(t, ctx)
	defer cleanup()
	seedWorkflowPolicyData(t, ctx, pool)

	ps := policy.NewService(pool)
	tplParams := map[string]any{
		"max_duration_minutes": 30,
		"require_approval":     true,
		"allow_auto_approve":   false,
		"require_reason":       true,
		"min_reason_length":    20,
		"max_requests_per_day": 10,
		"cool_down_minutes":    0,
		"single_use":           false,
		"notify_on_access":     true,
		"require_consent":      false,
	}
	tpl, err := ps.CreateTemplate(ctx, projectID, ownerID, "Broken Binding Template", "", strPtr("api_key"), tplParams)
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}
	if _, err := ps.UpdateBinding(ctx, secretID, ownerID, tpl.ID, tpl.CurrentVersion, map[string]any{}); err != nil {
		t.Fatalf("bind template failed: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE policy_template_versions SET parameters = '{"oops":true}'::jsonb WHERE template_id = $1 AND version = 1`, tpl.ID); err != nil {
		t.Fatalf("corrupt template version failed: %v", err)
	}

	svc := NewService(pool, true)
	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		SecretID:        secretID,
		RequesterUserID: requesterID,
		RequesterType:   "human",
		Reason:          "",
		DurationMinutes: 999,
		CredentialType:  "api_key",
	})
	if err != nil {
		t.Fatalf("create request should fallback to default, got err=%v", err)
	}
	if req.Status != "approved" {
		t.Fatalf("expected default auto-approved on fallback, got %s", req.Status)
	}

	counters := policy.SnapshotCounters()
	if counters["policy_resolution_total|source=default|status=runtime_fallback_default"] == 0 {
		t.Fatalf("expected runtime fallback counter increment, got counters=%v", counters)
	}
}

func TestEffectivePolicyForRequest_UsesSnapshotAfterBindingChanges(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	pool, cleanup := newWorkflowIntegrationDB(t, ctx)
	defer cleanup()
	seedWorkflowPolicyData(t, ctx, pool)

	ps := policy.NewService(pool)
	tplA, err := ps.CreateTemplate(ctx, projectID, ownerID, "Snapshot A", "", strPtr("api_key"), map[string]any{
		"max_duration_minutes": 30,
		"require_approval":     true,
		"allow_auto_approve":   false,
		"require_reason":       true,
		"min_reason_length":    20,
		"max_requests_per_day": 100,
		"cool_down_minutes":    0,
		"single_use":           true,
		"notify_on_access":     true,
		"require_consent":      false,
	})
	if err != nil {
		t.Fatalf("create template A failed: %v", err)
	}
	if _, err := ps.UpdateBinding(ctx, secretID, ownerID, tplA.ID, tplA.CurrentVersion, map[string]any{}); err != nil {
		t.Fatalf("bind template A failed: %v", err)
	}

	svc := NewService(pool, true)
	req, err := svc.CreateRequest(ctx, CreateRequestInput{
		SecretID:        secretID,
		RequesterUserID: requesterID,
		RequesterType:   "human",
		Reason:          "reason long enough for policy",
		DurationMinutes: 20,
		CredentialType:  "api_key",
	})
	if err != nil {
		t.Fatalf("create request failed: %v", err)
	}

	tplB, err := ps.CreateTemplate(ctx, projectID, ownerID, "Snapshot B", "", strPtr("api_key"), map[string]any{
		"max_duration_minutes": 480,
		"require_approval":     false,
		"allow_auto_approve":   true,
		"require_reason":       false,
		"min_reason_length":    0,
		"max_requests_per_day": 100,
		"cool_down_minutes":    0,
		"single_use":           false,
		"notify_on_access":     false,
		"require_consent":      false,
	})
	if err != nil {
		t.Fatalf("create template B failed: %v", err)
	}
	if _, err := ps.UpdateBinding(ctx, secretID, ownerID, tplB.ID, tplB.CurrentVersion, map[string]any{}); err != nil {
		t.Fatalf("rebind template B failed: %v", err)
	}

	requestPolicy := svc.EffectivePolicyForRequest(ctx, req.ID, "api_key")
	if !requestPolicy.SingleUse {
		t.Fatalf("expected request snapshot to retain single_use=true, got false")
	}
	if !requestPolicy.NotifyOnAccess {
		t.Fatalf("expected request snapshot to retain notify_on_access=true, got false")
	}

	secretPolicy := svc.EffectivePolicyForSecret(ctx, secretID, "api_key")
	if secretPolicy.SingleUse {
		t.Fatalf("expected secret runtime policy single_use=false after rebind")
	}
}
