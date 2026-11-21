package policy

import "time"

type Template struct {
	ID                 string           `json:"id"`
	ProjectID          string           `json:"project_id"`
	Name               string           `json:"name"`
	Description        string           `json:"description"`
	IsSystem           bool             `json:"is_system"`
	BaseCredentialType *string          `json:"base_credential_type,omitempty"`
	Parameters         PolicyParameters `json:"parameters"`
	CurrentVersion     int              `json:"current_version"`
	CreatedBy          *string          `json:"created_by,omitempty"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
	// Mapping visibility (2026-11-21): how many secrets bind this template
	// and their names — the policy ↔ secret relation made visible.
	BoundCount   int    `json:"bound_count"`
	BoundSecrets string `json:"bound_secrets,omitempty"`
}

type TemplateVersion struct {
	ID         string           `json:"id"`
	TemplateID string           `json:"template_id"`
	Version    int              `json:"version"`
	Parameters PolicyParameters `json:"parameters"`
	ChangeNote string           `json:"change_note"`
	CreatedBy  *string          `json:"created_by,omitempty"`
	CreatedAt  time.Time        `json:"created_at"`
}

type TemplateRef struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	IsSystem bool   `json:"is_system"`
}

type SecretPolicyBinding struct {
	SecretID            string           `json:"secret_id"`
	Template            *TemplateRef     `json:"template,omitempty"`
	TemplateVersion     *int             `json:"template_version,omitempty"`
	OverrideParameters  map[string]any   `json:"override_parameters"`
	OverrideWarnings    []string         `json:"override_warnings"`
	EffectivePolicy     PolicyParameters `json:"effective_policy"`
	AppliedPolicySource string           `json:"applied_policy_source"`
}

type AgentPolicyPermission struct {
	ID                     string     `json:"id"`
	SecretID               string     `json:"secret_id"`
	AgentID                string     `json:"agent_id"`
	CanReadEffectivePolicy bool       `json:"can_read_effective_policy"`
	GrantedByUserID        string     `json:"granted_by_user_id"`
	GrantedAt              time.Time  `json:"granted_at"`
	ExpiresAt              *time.Time `json:"expires_at,omitempty"`
	RevokedAt              *time.Time `json:"revoked_at,omitempty"`
}
