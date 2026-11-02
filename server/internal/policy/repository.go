package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ldphuong-vn/valtag/server/internal/rbac"
)

type Repository struct {
	pool *pgxpool.Pool
}

type rowScanner interface {
	Scan(dest ...any) error
}

type SecretContext struct {
	SecretID        string
	OwnerUserID     string
	ProjectID       *string
	CredentialType  string
	SecretName      string
	SecretDeletedAt *string
}

type RuntimeEffectivePolicy struct {
	Parameters      PolicyParameters
	TemplateID      *string
	TemplateVersion *int
	Warnings        []string
	Source          string
}

type bindingRow struct {
	SecretID         string
	TemplateID       string
	TemplateName     string
	TemplateIsSystem bool
	TemplateVersion  int
	TemplateParams   []byte
	OverrideRaw      []byte
	WarningsRaw      []byte
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func marshalParameters(params PolicyParameters) ([]byte, error) {
	raw, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal policy parameters: %w", err)
	}
	return raw, nil
}

func unmarshalParameters(raw []byte) (PolicyParameters, error) {
	var input map[string]any
	if err := json.Unmarshal(raw, &input); err != nil {
		return PolicyParameters{}, fmt.Errorf("decode policy parameters: %w", err)
	}
	params, err := ValidateParameters(input)
	if err != nil {
		return PolicyParameters{}, err
	}
	return params, nil
}

func unmarshalOverride(raw []byte) (map[string]any, error) {
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	var override map[string]any
	if err := json.Unmarshal(raw, &override); err != nil {
		return nil, fmt.Errorf("decode override parameters: %w", err)
	}
	if override == nil {
		return map[string]any{}, nil
	}
	if err := ensureAllowedKeys(override); err != nil {
		return nil, err
	}
	return override, nil
}

func marshalStringList(values []string) ([]byte, error) {
	raw, err := json.Marshal(values)
	if err != nil {
		return nil, fmt.Errorf("marshal string list: %w", err)
	}
	return raw, nil
}

func unmarshalStringList(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var out []string
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("decode string list: %w", err)
	}
	if out == nil {
		return []string{}, nil
	}
	return out, nil
}

func scanTemplate(row rowScanner) (Template, error) {
	var t Template
	var paramsRaw []byte
	err := row.Scan(
		&t.ID,
		&t.ProjectID,
		&t.Name,
		&t.Description,
		&t.IsSystem,
		&t.BaseCredentialType,
		&t.CreatedBy,
		&t.CreatedAt,
		&t.UpdatedAt,
		&t.CurrentVersion,
		&paramsRaw,
	)
	if err != nil {
		return Template{}, err
	}
	params, err := unmarshalParameters(paramsRaw)
	if err != nil {
		return Template{}, fmt.Errorf("scan template parameters: %w", err)
	}
	t.Parameters = params
	return t, nil
}

func scanTemplateVersion(row rowScanner) (TemplateVersion, error) {
	var v TemplateVersion
	var paramsRaw []byte
	err := row.Scan(&v.ID, &v.TemplateID, &v.Version, &paramsRaw, &v.ChangeNote, &v.CreatedBy, &v.CreatedAt)
	if err != nil {
		return TemplateVersion{}, err
	}
	params, err := unmarshalParameters(paramsRaw)
	if err != nil {
		return TemplateVersion{}, fmt.Errorf("scan template version parameters: %w", err)
	}
	v.Parameters = params
	return v, nil
}

func scanPermission(row rowScanner) (AgentPolicyPermission, error) {
	var p AgentPolicyPermission
	err := row.Scan(
		&p.ID,
		&p.SecretID,
		&p.AgentID,
		&p.CanReadEffectivePolicy,
		&p.GrantedByUserID,
		&p.GrantedAt,
		&p.ExpiresAt,
		&p.RevokedAt,
	)
	if err != nil {
		return AgentPolicyPermission{}, err
	}
	return p, nil
}

func (r *Repository) GetSecretContext(ctx context.Context, secretID string) (*SecretContext, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, user_id, project_id, credential_type, name
		FROM secrets
		WHERE id = $1 AND deleted_at IS NULL`, secretID)
	var sc SecretContext
	err := row.Scan(&sc.SecretID, &sc.OwnerUserID, &sc.ProjectID, &sc.CredentialType, &sc.SecretName)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, fmt.Errorf("get secret context: %w", err)
	}
	return &sc, nil
}

func (r *Repository) GetProjectRole(ctx context.Context, projectID, userID string) (string, error) {
	var role string
	err := r.pool.QueryRow(ctx,
		`SELECT role FROM project_memberships WHERE project_id = $1 AND user_id = $2`,
		projectID, userID,
	).Scan(&role)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", ErrPolicyForbidden
		}
		return "", fmt.Errorf("get project role: %w", err)
	}
	return role, nil
}

func (r *Repository) IsProjectAdmin(ctx context.Context, projectID, userID string) (bool, error) {
	role, err := r.GetProjectRole(ctx, projectID, userID)
	if err != nil {
		if errors.Is(err, ErrPolicyForbidden) {
			return false, nil
		}
		return false, err
	}
	role = rbac.RoleFromProjectMembership(role)
	return role == "owner" || role == "admin", nil
}

func (r *Repository) CanProject(ctx context.Context, projectID, userID string, resource rbac.Resource, action rbac.Action) (bool, error) {
	role, err := r.GetProjectRole(ctx, projectID, userID)
	if err != nil {
		if errors.Is(err, ErrPolicyForbidden) {
			return false, nil
		}
		return false, err
	}
	return rbac.Can(rbac.RoleFromProjectMembership(role), resource, action), nil
}

func (r *Repository) AgentInProject(ctx context.Context, agentID, projectID string) (bool, error) {
	var count int
	err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM agent_identities WHERE id = $1 AND project_id = $2 AND status = 'active'`,
		agentID, projectID,
	).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("check agent project membership: %w", err)
	}
	return count > 0, nil
}

func (r *Repository) ListTemplates(ctx context.Context, projectID string) ([]Template, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT t.id, t.project_id, t.name, t.description, t.is_system, t.base_credential_type,
		       t.created_by, t.created_at, t.updated_at, v.version, v.parameters
		FROM policy_templates t
		JOIN LATERAL (
			SELECT version, parameters
			FROM policy_template_versions
			WHERE template_id = t.id
			ORDER BY version DESC
			LIMIT 1
		) v ON TRUE
		WHERE t.project_id = $1
		ORDER BY t.is_system DESC, lower(t.name) ASC`, projectID)
	if err != nil {
		return nil, fmt.Errorf("list policy templates: %w", err)
	}
	defer rows.Close()

	templates := []Template{}
	for rows.Next() {
		t, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		templates = append(templates, t)
	}
	return templates, nil
}

func (r *Repository) GetTemplate(ctx context.Context, templateID string) (*Template, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT t.id, t.project_id, t.name, t.description, t.is_system, t.base_credential_type,
		       t.created_by, t.created_at, t.updated_at, v.version, v.parameters
		FROM policy_templates t
		JOIN LATERAL (
			SELECT version, parameters
			FROM policy_template_versions
			WHERE template_id = t.id
			ORDER BY version DESC
			LIMIT 1
		) v ON TRUE
		WHERE t.id = $1`, templateID)
	t, err := scanTemplate(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	return &t, nil
}

func (r *Repository) CreateTemplate(ctx context.Context, projectID, name, description string, baseCredentialType, createdBy *string, params PolicyParameters, isSystem bool) (*Template, error) {
	paramsRaw, err := marshalParameters(params)
	if err != nil {
		return nil, err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin create template tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var templateID string
	err = tx.QueryRow(ctx, `
		INSERT INTO policy_templates (project_id, name, description, is_system, base_credential_type, parameters, created_by)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7)
		RETURNING id`, projectID, strings.TrimSpace(name), description, isSystem, baseCredentialType, paramsRaw, createdBy).Scan(&templateID)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrPolicyConflict
		}
		return nil, fmt.Errorf("insert policy template: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO policy_template_versions (template_id, version, parameters, change_note, created_by)
		VALUES ($1, 1, $2::jsonb, '', $3)`, templateID, paramsRaw, createdBy)
	if err != nil {
		return nil, fmt.Errorf("insert policy template version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit create template tx: %w", err)
	}
	return r.GetTemplate(ctx, templateID)
}

func (r *Repository) CreateTemplateVersion(ctx context.Context, templateID string, params PolicyParameters, changeNote string, createdBy *string) (*TemplateVersion, error) {
	paramsRaw, err := marshalParameters(params)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(ctx, `
		WITH next_version AS (
			SELECT COALESCE(MAX(version), 0) + 1 AS version
			FROM policy_template_versions
			WHERE template_id = $1
		)
		INSERT INTO policy_template_versions (template_id, version, parameters, change_note, created_by)
		SELECT $1, next_version.version, $2::jsonb, $3, $4
		FROM next_version
		RETURNING id, template_id, version, parameters, change_note, created_by, created_at`,
		templateID, paramsRaw, changeNote, createdBy,
	)
	v, err := scanTemplateVersion(row)
	if err != nil {
		return nil, fmt.Errorf("create policy template version: %w", err)
	}
	_, err = r.pool.Exec(ctx, `UPDATE policy_templates SET parameters = $2::jsonb, updated_at = NOW() WHERE id = $1`, templateID, paramsRaw)
	if err != nil {
		return nil, fmt.Errorf("touch policy template: %w", err)
	}
	return &v, nil
}

func (r *Repository) DeleteTemplate(ctx context.Context, templateID string) error {
	var usage int
	err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM secret_policy_bindings WHERE template_id = $1`, templateID).Scan(&usage)
	if err != nil {
		return fmt.Errorf("check template usage: %w", err)
	}
	if usage > 0 {
		return ErrPolicyConflict
	}
	tag, err := r.pool.Exec(ctx, `DELETE FROM policy_templates WHERE id = $1`, templateID)
	if err != nil {
		return fmt.Errorf("delete policy template: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

func (r *Repository) ListTemplateVersions(ctx context.Context, templateID string) ([]TemplateVersion, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, template_id, version, parameters, change_note, created_by, created_at
		FROM policy_template_versions
		WHERE template_id = $1
		ORDER BY version DESC`, templateID)
	if err != nil {
		return nil, fmt.Errorf("list policy template versions: %w", err)
	}
	defer rows.Close()

	versions := []TemplateVersion{}
	for rows.Next() {
		v, err := scanTemplateVersion(rows)
		if err != nil {
			return nil, err
		}
		versions = append(versions, v)
	}
	return versions, nil
}

func (r *Repository) GetTemplateVersion(ctx context.Context, templateID string, version int) (*TemplateVersion, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, template_id, version, parameters, change_note, created_by, created_at
		FROM policy_template_versions
		WHERE template_id = $1 AND version = $2`, templateID, version)
	v, err := scanTemplateVersion(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	return &v, nil
}

func (r *Repository) GetSecretBinding(ctx context.Context, secretID string) (*SecretPolicyBinding, error) {
	secret, err := r.GetSecretContext(ctx, secretID)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(ctx, `
		SELECT b.secret_id, b.template_id, t.name, t.is_system, b.template_version,
		       v.parameters, b.override_parameters, b.override_warnings
		FROM secret_policy_bindings b
		JOIN policy_templates t ON t.id = b.template_id
		JOIN policy_template_versions v ON v.template_id = b.template_id AND v.version = b.template_version
		WHERE b.secret_id = $1`, secretID)

	var raw bindingRow
	err = row.Scan(
		&raw.SecretID,
		&raw.TemplateID,
		&raw.TemplateName,
		&raw.TemplateIsSystem,
		&raw.TemplateVersion,
		&raw.TemplateParams,
		&raw.OverrideRaw,
		&raw.WarningsRaw,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			defaults := DefaultParametersForCredentialType(secret.CredentialType)
			return &SecretPolicyBinding{
				SecretID:            secretID,
				Template:            nil,
				TemplateVersion:     nil,
				OverrideParameters:  map[string]any{},
				OverrideWarnings:    []string{},
				EffectivePolicy:     defaults,
				AppliedPolicySource: PolicySourceDefault,
			}, nil
		}
		return nil, fmt.Errorf("get secret policy binding: %w", err)
	}

	tpl, err := unmarshalParameters(raw.TemplateParams)
	if err != nil {
		return nil, err
	}
	override, err := unmarshalOverride(raw.OverrideRaw)
	if err != nil {
		return nil, err
	}
	warnings, err := unmarshalStringList(raw.WarningsRaw)
	if err != nil {
		return nil, err
	}

	effective, resolvedWarnings, err := ResolveEffectivePolicy(tpl, override)
	if err != nil {
		return nil, err
	}
	if len(warnings) == 0 {
		warnings = resolvedWarnings
	}
	v := raw.TemplateVersion
	return &SecretPolicyBinding{
		SecretID:            raw.SecretID,
		Template:            &TemplateRef{ID: raw.TemplateID, Name: raw.TemplateName, IsSystem: raw.TemplateIsSystem},
		TemplateVersion:     &v,
		OverrideParameters:  override,
		OverrideWarnings:    warnings,
		EffectivePolicy:     effective,
		AppliedPolicySource: sourceForBinding(override),
	}, nil
}

func sourceForBinding(override map[string]any) string {
	if len(override) > 0 {
		return PolicySourceTemplateOverride
	}
	return PolicySourceTemplate
}

func (r *Repository) UpsertSecretBinding(ctx context.Context, secretID, templateID string, templateVersion int, override map[string]any, updatedBy *string) (*SecretPolicyBinding, error) {
	tplVersion, err := r.GetTemplateVersion(ctx, templateID, templateVersion)
	if err != nil {
		return nil, err
	}
	if override == nil {
		override = map[string]any{}
	}
	effective, warnings, err := ResolveEffectivePolicy(tplVersion.Parameters, override)
	if err != nil {
		return nil, err
	}
	overrideRaw, err := json.Marshal(override)
	if err != nil {
		return nil, err
	}
	warningsRaw, err := marshalStringList(warnings)
	if err != nil {
		return nil, err
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO secret_policy_bindings (secret_id, template_id, template_version, override_parameters, override_warnings, updated_by)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6)
		ON CONFLICT (secret_id)
		DO UPDATE SET template_id = EXCLUDED.template_id,
		              template_version = EXCLUDED.template_version,
		              override_parameters = EXCLUDED.override_parameters,
		              override_warnings = EXCLUDED.override_warnings,
		              updated_by = EXCLUDED.updated_by,
		              updated_at = NOW()`,
		secretID, templateID, templateVersion, overrideRaw, warningsRaw, updatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert secret policy binding: %w", err)
	}
	binding, err := r.GetSecretBinding(ctx, secretID)
	if err != nil {
		return nil, err
	}
	binding.EffectivePolicy = effective
	binding.OverrideWarnings = warnings
	binding.AppliedPolicySource = sourceForBinding(override)
	return binding, nil
}

func (r *Repository) GrantAgentPermission(ctx context.Context, secretID, agentID, grantedBy string) (*AgentPolicyPermission, error) {
	_, err := r.pool.Exec(ctx, `
		UPDATE secret_policy_agent_permissions
		SET revoked_at = NOW()
		WHERE secret_id = $1 AND agent_id = $2 AND revoked_at IS NULL`,
		secretID, agentID,
	)
	if err != nil {
		return nil, fmt.Errorf("revoke previous agent permission: %w", err)
	}

	row := r.pool.QueryRow(ctx, `
		INSERT INTO secret_policy_agent_permissions (
			secret_id, agent_id, can_read_effective_policy, granted_by_user_id
		) VALUES ($1, $2, TRUE, $3)
		RETURNING id, secret_id, agent_id, can_read_effective_policy,
		          granted_by_user_id, granted_at, expires_at, revoked_at`,
		secretID, agentID, grantedBy,
	)
	p, err := scanPermission(row)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *Repository) RevokeAgentPermission(ctx context.Context, secretID, agentID string) error {
	tag, err := r.pool.Exec(ctx, `
		UPDATE secret_policy_agent_permissions
		SET revoked_at = NOW()
		WHERE secret_id = $1 AND agent_id = $2 AND revoked_at IS NULL`,
		secretID, agentID,
	)
	if err != nil {
		return fmt.Errorf("revoke agent permission: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrPolicyNotFound
	}
	return nil
}

func (r *Repository) GetActiveAgentPermission(ctx context.Context, secretID, agentID string) (*AgentPolicyPermission, error) {
	row := r.pool.QueryRow(ctx, `
		SELECT id, secret_id, agent_id, can_read_effective_policy,
		       granted_by_user_id, granted_at, expires_at, revoked_at
		FROM secret_policy_agent_permissions
		WHERE secret_id = $1 AND agent_id = $2
		  AND revoked_at IS NULL
		  AND (expires_at IS NULL OR expires_at > NOW())
		ORDER BY granted_at DESC
		LIMIT 1`, secretID, agentID)
	p, err := scanPermission(row)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrPolicyNotFound
		}
		return nil, err
	}
	return &p, nil
}

func (r *Repository) EnsureSystemTemplate(ctx context.Context, projectID, name, credentialType string, params PolicyParameters) error {
	paramsRaw, err := marshalParameters(params)
	if err != nil {
		return err
	}

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin ensure system template tx: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	var templateID string
	err = tx.QueryRow(ctx, `
		SELECT id FROM policy_templates
		WHERE project_id = $1 AND is_system = TRUE AND lower(name) = lower($2)
		LIMIT 1`, projectID, name).Scan(&templateID)
	if err == nil {
		return tx.Commit(ctx)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return fmt.Errorf("find existing system template: %w", err)
	}

	err = tx.QueryRow(ctx, `
		INSERT INTO policy_templates (project_id, name, description, is_system, base_credential_type, parameters)
		VALUES ($1, $2, '', TRUE, $3, $4::jsonb)
		RETURNING id`, projectID, strings.TrimSpace(name), credentialType, paramsRaw).Scan(&templateID)
	if err != nil {
		if isUniqueViolation(err) {
			return tx.Commit(ctx)
		}
		return fmt.Errorf("insert system template: %w", err)
	}

	_, err = tx.Exec(ctx, `
		INSERT INTO policy_template_versions (template_id, version, parameters, change_note, created_by)
		VALUES ($1, 1, $2::jsonb, 'seeded from default tier mapping', NULL)`, templateID, paramsRaw)
	if err != nil {
		return fmt.Errorf("insert system template version: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit ensure system template tx: %w", err)
	}
	return nil
}

func (r *Repository) ResolveRuntimePolicyBySecretID(ctx context.Context, secretID string) (*RuntimeEffectivePolicy, error) {
	secret, err := r.GetSecretContext(ctx, secretID)
	if err != nil {
		return nil, err
	}

	row := r.pool.QueryRow(ctx, `
		SELECT b.template_id, b.template_version, v.parameters, b.override_parameters, b.override_warnings
		FROM secret_policy_bindings b
		JOIN policy_template_versions v ON v.template_id = b.template_id AND v.version = b.template_version
		WHERE b.secret_id = $1`, secretID)

	var templateID string
	var templateVersion int
	var templateRaw []byte
	var overrideRaw []byte
	var warningsRaw []byte
	err = row.Scan(&templateID, &templateVersion, &templateRaw, &overrideRaw, &warningsRaw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			ObserveResolution(PolicySourceDefault, "ok", nil)
			defaults := DefaultParametersForCredentialType(secret.CredentialType)
			return &RuntimeEffectivePolicy{
				Parameters: defaults,
				Warnings:   []string{},
				Source:     PolicySourceDefault,
			}, nil
		}
		ObserveResolution(PolicySourceDefault, "runtime_query_failed", nil)
		return nil, fmt.Errorf("resolve runtime policy binding: %w", err)
	}

	tpl, err := unmarshalParameters(templateRaw)
	if err != nil {
		ObserveResolution(PolicySourceTemplate, "runtime_decode_failed", nil)
		return nil, fmt.Errorf("decode runtime template parameters: %w", err)
	}
	override, err := unmarshalOverride(overrideRaw)
	if err != nil {
		ObserveResolution(PolicySourceTemplate, "runtime_decode_failed", nil)
		return nil, fmt.Errorf("decode runtime override parameters: %w", err)
	}
	warnings, err := unmarshalStringList(warningsRaw)
	if err != nil {
		ObserveResolution(PolicySourceTemplate, "runtime_decode_failed", nil)
		return nil, fmt.Errorf("decode runtime warnings: %w", err)
	}

	effective, resolvedWarnings, err := ResolveEffectivePolicy(tpl, override)
	if err != nil {
		ObserveResolution(sourceForBinding(override), "runtime_resolve_failed", nil)
		return nil, fmt.Errorf("resolve runtime effective policy: %w", err)
	}
	if len(warnings) == 0 {
		warnings = resolvedWarnings
	}

	return &RuntimeEffectivePolicy{
		Parameters:      effective,
		TemplateID:      &templateID,
		TemplateVersion: &templateVersion,
		Warnings:        warnings,
		Source:          sourceForBinding(override),
	}, nil
}
