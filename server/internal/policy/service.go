package policy

import (
	"context"
	"errors"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/ldphuong-vn/valtag/server/internal/rbac"
)

type Service struct {
	repo *Repository
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{repo: NewRepository(pool)}
}

type systemTemplateSeed struct {
	Name           string
	CredentialType string
}

var defaultSystemTemplateSeeds = []systemTemplateSeed{
	{Name: "Default API Key", CredentialType: "api_key"},
	{Name: "Default DB Credential", CredentialType: "db_credential"},
	{Name: "Default SSH Key", CredentialType: "ssh_key"},
	{Name: "Default OAuth Token", CredentialType: "oauth_token"},
	{Name: "Default Cloud Credential", CredentialType: "cloud_credential"},
	{Name: "Default Personal Session", CredentialType: "personal_session"},
}

func (s *Service) requireProjectAdmin(ctx context.Context, projectID, userID string) error {
	ok, err := s.repo.IsProjectAdmin(ctx, projectID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPolicyForbidden
	}
	return nil
}

func (s *Service) requireSecretPolicyAdmin(ctx context.Context, secret *SecretContext, userID string) error {
	if secret.ProjectID == nil {
		if secret.OwnerUserID != userID {
			return ErrPolicyForbidden
		}
		return nil
	}

	ok, err := s.repo.IsProjectAdmin(ctx, *secret.ProjectID, userID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrPolicyForbidden
	}

	return nil
}

func (s *Service) SeedSystemTemplates(ctx context.Context, projectID string) error {
	for _, item := range defaultSystemTemplateSeeds {
		params := DefaultParametersForCredentialType(item.CredentialType)
		if err := s.repo.EnsureSystemTemplate(ctx, projectID, item.Name, item.CredentialType, params); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) ListTemplates(ctx context.Context, projectID, userID string) ([]Template, error) {
	if err := s.requireProjectAdmin(ctx, projectID, userID); err != nil {
		return nil, err
	}
	if err := s.SeedSystemTemplates(ctx, projectID); err != nil {
		return nil, err
	}
	return s.repo.ListTemplates(ctx, projectID)
}

func (s *Service) CreateTemplate(ctx context.Context, projectID, userID, name, description string, baseCredentialType *string, parameters map[string]any) (*Template, error) {
	if err := s.requireProjectAdmin(ctx, projectID, userID); err != nil {
		return nil, err
	}
	params, err := ValidateParameters(parameters)
	if err != nil {
		return nil, ErrPolicyInvalid
	}
	createdBy := userID
	t, err := s.repo.CreateTemplate(ctx, projectID, name, description, baseCredentialType, &createdBy, params, false)
	if err != nil {
		if errors.Is(err, ErrPolicyConflict) {
			return nil, ErrPolicyConflict
		}
		return nil, err
	}
	return t, nil
}

func (s *Service) GetTemplate(ctx context.Context, templateID, userID string) (*Template, error) {
	tpl, err := s.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.requireProjectAdmin(ctx, tpl.ProjectID, userID); err != nil {
		return nil, err
	}
	return tpl, nil
}

func (s *Service) UpdateTemplate(ctx context.Context, templateID, userID string, parameters map[string]any, changeNote string) (*TemplateVersion, error) {
	tpl, err := s.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if tpl.IsSystem {
		return nil, ErrPolicyForbidden
	}
	if err := s.requireProjectAdmin(ctx, tpl.ProjectID, userID); err != nil {
		return nil, err
	}
	params, err := ValidateParameters(parameters)
	if err != nil {
		return nil, ErrPolicyInvalid
	}
	createdBy := userID
	return s.repo.CreateTemplateVersion(ctx, templateID, params, changeNote, &createdBy)
}

func (s *Service) CloneTemplate(ctx context.Context, templateID, userID, newName string) (*Template, error) {
	source, err := s.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.requireProjectAdmin(ctx, source.ProjectID, userID); err != nil {
		return nil, err
	}
	createdBy := userID
	clone, err := s.repo.CreateTemplate(ctx, source.ProjectID, newName, source.Description, source.BaseCredentialType, &createdBy, source.Parameters, false)
	if err != nil {
		if errors.Is(err, ErrPolicyConflict) {
			return nil, ErrPolicyConflict
		}
		return nil, err
	}
	return clone, nil
}

func (s *Service) DeleteTemplate(ctx context.Context, templateID, userID string) error {
	tpl, err := s.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return err
	}
	if tpl.IsSystem {
		return ErrPolicyForbidden
	}
	if err := s.requireProjectAdmin(ctx, tpl.ProjectID, userID); err != nil {
		return err
	}
	return s.repo.DeleteTemplate(ctx, templateID)
}

func (s *Service) ListTemplateVersions(ctx context.Context, templateID, userID string) ([]TemplateVersion, error) {
	tpl, err := s.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := s.requireProjectAdmin(ctx, tpl.ProjectID, userID); err != nil {
		return nil, err
	}
	return s.repo.ListTemplateVersions(ctx, templateID)
}

func (s *Service) GetBinding(ctx context.Context, secretID, userID string) (*SecretPolicyBinding, error) {
	secret, err := s.repo.GetSecretContext(ctx, secretID)
	if err != nil {
		return nil, err
	}
	if secret.ProjectID != nil {
		allowed, err := s.repo.CanProject(ctx, *secret.ProjectID, userID, rbac.ResourceSecret, rbac.ActionRead)
		if err != nil {
			return nil, err
		}
		if !allowed {
			return nil, ErrPolicyForbidden
		}
	} else if secret.OwnerUserID != userID {
		return nil, ErrPolicyForbidden
	}
	return s.repo.GetSecretBinding(ctx, secretID)
}

func (s *Service) UpdateBinding(ctx context.Context, secretID, userID, templateID string, templateVersion int, override map[string]any) (*SecretPolicyBinding, error) {
	secret, err := s.repo.GetSecretContext(ctx, secretID)
	if err != nil {
		return nil, err
	}
	tpl, err := s.repo.GetTemplate(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if secret.ProjectID == nil || tpl.ProjectID != *secret.ProjectID {
		log.Printf("Forbidden because mismatch project")
		return nil, ErrPolicyForbidden
	}
	allowed, err := s.repo.CanProject(ctx, tpl.ProjectID, userID, rbac.ResourceSecret, rbac.ActionWrite)
	if err != nil {
		return nil, err
	}
	if !allowed {
		log.Printf("Forbidden because user cannot write to project")
		return nil, ErrPolicyForbidden
	}
	if _, err := s.repo.GetTemplateVersion(ctx, templateID, templateVersion); err != nil {
		return nil, err
	}
	updatedBy := userID
	binding, err := s.repo.UpsertSecretBinding(ctx, secretID, templateID, templateVersion, override, &updatedBy)
	if err != nil {
		if errors.Is(err, ErrPolicyInvalid) {
			return nil, ErrPolicyInvalid
		}
		return nil, err
	}
	return binding, nil
}

func (s *Service) GrantPolicyReadToAgent(ctx context.Context, secretID, agentID, userID string) (*AgentPolicyPermission, error) {
	secret, err := s.repo.GetSecretContext(ctx, secretID)
	if err != nil {
		return nil, err
	}
	if err := s.requireSecretPolicyAdmin(ctx, secret, userID); err != nil {
		return nil, err
	}

	if secret.ProjectID != nil {
		ok, err := s.repo.AgentInProject(ctx, agentID, *secret.ProjectID)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, ErrPolicyForbidden
		}
	}

	return s.repo.GrantAgentPermission(ctx, secretID, agentID, userID)
}

func (s *Service) RevokePolicyReadFromAgent(ctx context.Context, secretID, agentID, userID string) error {
	secret, err := s.repo.GetSecretContext(ctx, secretID)
	if err != nil {
		return err
	}
	if err := s.requireSecretPolicyAdmin(ctx, secret, userID); err != nil {
		return err
	}
	return s.repo.RevokeAgentPermission(ctx, secretID, agentID)
}
