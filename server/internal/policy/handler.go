package policy

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.registerRoutes(r)
	return r
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	h.registerRoutes(r)
}

func (h *Handler) registerRoutes(r chi.Router) {
	r.Get("/projects/{project_id}/policy-templates", h.listTemplates)
	r.Post("/projects/{project_id}/policy-templates", h.createTemplate)
	r.Get("/policy-templates/{template_id}", h.getTemplate)
	r.Put("/policy-templates/{template_id}", h.updateTemplate)
	r.Post("/policy-templates/{template_id}/clone", h.cloneTemplate)
	r.Delete("/policy-templates/{template_id}", h.deleteTemplate)
	r.Get("/policy-templates/{template_id}/versions", h.listTemplateVersions)

	r.Get("/secrets/{secret_id}/policy-binding", h.getBinding)
	r.Put("/secrets/{secret_id}/policy-binding", h.updateBinding)

	r.Post("/secrets/{secret_id}/policy-agent-permissions", h.grantAgentPermission)
	r.Delete("/secrets/{secret_id}/policy-agent-permissions/{agent_id}", h.revokeAgentPermission)
}

func userIDOrUnauthorized(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID := auth.UserIDFromContext(r.Context())
	if userID == "" {
		apierror.Unauthorized(w, "authentication required")
		return "", false
	}
	return userID, true
}

func decodeJSONBody(r *http.Request, dst any) error {
	if err := json.NewDecoder(r.Body).Decode(dst); err != nil {
		return ErrPolicyInvalid
	}
	return nil
}

func urlParamOrInvalid(r *http.Request, name string) (string, error) {
	value := chi.URLParam(r, name)
	if value == "" {
		return "", ErrPolicyInvalid
	}
	return value, nil
}

func writePolicyErr(w http.ResponseWriter, err error) {
	switch {
	case err == nil:
		return
	case errors.Is(err, ErrPolicyForbidden):
		apierror.Forbidden(w, "access denied")
	case errors.Is(err, ErrPolicyNotFound):
		apierror.NotFound(w, "resource not found")
	case errors.Is(err, ErrPolicyConflict):
		apierror.Conflict(w, "resource conflict")
	case errors.Is(err, ErrPolicyInvalid):
		apierror.BadRequest(w, "invalid policy payload")
	default:
		apierror.InternalError(w, "policy operation failed")
	}
}

type createTemplateRequest struct {
	Name               string         `json:"name"`
	Description        string         `json:"description"`
	BaseCredentialType *string        `json:"base_credential_type,omitempty"`
	Parameters         map[string]any `json:"parameters"`
}

type updateTemplateRequest struct {
	Parameters map[string]any `json:"parameters"`
	ChangeNote string         `json:"change_note"`
}

type cloneTemplateRequest struct {
	Name string `json:"name"`
}

type updateBindingRequest struct {
	TemplateID      string         `json:"template_id"`
	TemplateVersion int            `json:"template_version"`
	Override        map[string]any `json:"override_parameters"`
}

type grantAgentPermissionRequest struct {
	AgentID string `json:"agent_id"`
}

func (h *Handler) listTemplates(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	projectID := chi.URLParam(r, "project_id")
	if projectID == "" {
		writePolicyErr(w, ErrPolicyInvalid)
		return
	}
	templates, err := h.service.ListTemplates(r.Context(), projectID, userID)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"templates": templates})
}

func (h *Handler) createTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	projectID, err := urlParamOrInvalid(r, "project_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	var req createTemplateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writePolicyErr(w, err)
		return
	}
	tpl, err := h.service.CreateTemplate(r.Context(), projectID, userID, req.Name, req.Description, req.BaseCredentialType, req.Parameters)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

func (h *Handler) getTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	templateID, err := urlParamOrInvalid(r, "template_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	tpl, err := h.service.GetTemplate(r.Context(), templateID, userID)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, tpl)
}

func (h *Handler) updateTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	templateID, err := urlParamOrInvalid(r, "template_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	var req updateTemplateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writePolicyErr(w, err)
		return
	}
	v, err := h.service.UpdateTemplate(r.Context(), templateID, userID, req.Parameters, req.ChangeNote)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

func (h *Handler) cloneTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	templateID, err := urlParamOrInvalid(r, "template_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	var req cloneTemplateRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writePolicyErr(w, err)
		return
	}
	if req.Name == "" {
		writePolicyErr(w, ErrPolicyInvalid)
		return
	}
	tpl, err := h.service.CloneTemplate(r.Context(), templateID, userID, req.Name)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, tpl)
}

func (h *Handler) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	templateID, err := urlParamOrInvalid(r, "template_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	if err := h.service.DeleteTemplate(r.Context(), templateID, userID); err != nil {
		writePolicyErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listTemplateVersions(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	templateID, err := urlParamOrInvalid(r, "template_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	versions, err := h.service.ListTemplateVersions(r.Context(), templateID, userID)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"versions": versions})
}

func (h *Handler) getBinding(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	secretID, err := urlParamOrInvalid(r, "secret_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	binding, err := h.service.GetBinding(r.Context(), secretID, userID)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, binding)
}

func (h *Handler) updateBinding(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	secretID, err := urlParamOrInvalid(r, "secret_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	var req updateBindingRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writePolicyErr(w, err)
		return
	}
	if req.TemplateID == "" || req.TemplateVersion <= 0 {
		writePolicyErr(w, ErrPolicyInvalid)
		return
	}
	binding, err := h.service.UpdateBinding(r.Context(), secretID, userID, req.TemplateID, req.TemplateVersion, req.Override)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, binding)
}

func (h *Handler) grantAgentPermission(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	secretID, err := urlParamOrInvalid(r, "secret_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	var req grantAgentPermissionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writePolicyErr(w, err)
		return
	}
	if req.AgentID == "" {
		writePolicyErr(w, ErrPolicyInvalid)
		return
	}
	p, err := h.service.GrantPolicyReadToAgent(r.Context(), secretID, req.AgentID, userID)
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (h *Handler) revokeAgentPermission(w http.ResponseWriter, r *http.Request) {
	userID, ok := userIDOrUnauthorized(w, r)
	if !ok {
		return
	}
	secretID, err := urlParamOrInvalid(r, "secret_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	agentID, err := urlParamOrInvalid(r, "agent_id")
	if err != nil {
		writePolicyErr(w, err)
		return
	}
	if err := h.service.RevokePolicyReadFromAgent(r.Context(), secretID, agentID, userID); err != nil {
		writePolicyErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(payload) //nolint:errcheck
}
