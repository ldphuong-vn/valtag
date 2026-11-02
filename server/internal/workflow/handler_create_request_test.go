package workflow

import (
	"context"
	"testing"

	"github.com/ldphuong-vn/valtag/server/internal/agent"
	"github.com/ldphuong-vn/valtag/server/internal/auth"
)

// TestCreateRequest_NoIdentity_BareContext verifies that a bare context
// carries no user or agent identity, which would cause CreateRequest to
// return 401 in production.
func TestCreateRequest_NoIdentity_BareContext(t *testing.T) {
	ctx := context.Background()
	if auth.UserIDFromContext(ctx) != "" {
		t.Error("expected empty userID on bare context")
	}
	if agent.AgentIDFromContext(ctx) != "" {
		t.Error("expected empty agentID on bare context")
	}
}

// TestCreateRequest_UserIdentityContext verifies that a user-authenticated
// context carries the correct user ID and no agent ID.
func TestCreateRequest_UserIdentityContext(t *testing.T) {
	ctx := context.Background()
	userCtx := auth.WithUserID(ctx, "user-abc")

	if got := auth.UserIDFromContext(userCtx); got != "user-abc" {
		t.Errorf("user context: expected user-abc, got %q", got)
	}
	if got := agent.AgentIDFromContext(userCtx); got != "" {
		t.Errorf("user context should carry no agent ID, got %q", got)
	}
}

// TestCreateRequest_AgentIdentityContext verifies that an agent-authenticated
// context carries the correct agent ID and no user ID.
func TestCreateRequest_AgentIdentityContext(t *testing.T) {
	ctx := context.Background()
	agentCtx := agent.WithAgentID(ctx, "agent-xyz")

	if got := auth.UserIDFromContext(agentCtx); got != "" {
		t.Errorf("agent context should carry no user ID, got %q", got)
	}
	if got := agent.AgentIDFromContext(agentCtx); got != "agent-xyz" {
		t.Errorf("agent context: expected agent-xyz, got %q", got)
	}
}

// TestCreateRequest_DualIdentityIsolation verifies that user and agent
// context values do not cross-contaminate each other.
func TestCreateRequest_DualIdentityIsolation(t *testing.T) {
	base := context.Background()

	userCtx := auth.WithUserID(base, "user-111")
	agentCtx := agent.WithAgentID(base, "agent-222")

	// user ctx should not expose agent ID
	if agent.AgentIDFromContext(userCtx) != "" {
		t.Error("user context must not contain agent ID")
	}
	// agent ctx should not expose user ID
	if auth.UserIDFromContext(agentCtx) != "" {
		t.Error("agent context must not contain user ID")
	}
	// combined: stacking both values on same context
	combinedCtx := agent.WithAgentID(auth.WithUserID(base, "user-333"), "agent-444")
	if auth.UserIDFromContext(combinedCtx) != "user-333" {
		t.Errorf("combined ctx: expected user-333, got %q", auth.UserIDFromContext(combinedCtx))
	}
	if agent.AgentIDFromContext(combinedCtx) != "agent-444" {
		t.Errorf("combined ctx: expected agent-444, got %q", agent.AgentIDFromContext(combinedCtx))
	}
}
