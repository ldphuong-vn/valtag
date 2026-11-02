package scanner

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/internal/rbac"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Handler handles HTTP routes for the scanner package.
type Handler struct {
	service *Service
	db      *pgxpool.Pool
}

// NewHandler creates a new scanner Handler.
func NewHandler(service *Service, db *pgxpool.Pool) *Handler {
	return &Handler{service: service, db: db}
}

// Routes registers all scanner routes.
func (h *Handler) Routes() chi.Router {
	r := chi.NewRouter()
	h.registerRoutes(r)
	return r
}

// RegisterRoutes registers scanner routes onto an existing router group.
func (h *Handler) RegisterRoutes(r chi.Router) {
	h.registerRoutes(r)
}

func (h *Handler) registerRoutes(r chi.Router) {
	r.With(rbac.Middleware(h.db, "project_id", rbac.ResourceScans, rbac.ActionWrite)).
		Post("/projects/{project_id}/scans", h.CreateScan)
	r.With(rbac.Middleware(h.db, "project_id", rbac.ResourceScans, rbac.ActionRead)).
		Get("/projects/{project_id}/scans", h.ListScans)
	r.Get("/scans/{scan_id}/findings", h.ListFindings)
	r.Post("/scans/{scan_id}/findings/{finding_id}/import", h.ImportFinding)
	r.Post("/scans/{scan_id}/findings/{finding_id}/dismiss", h.DismissFinding)
}

type createScanRequest struct {
	ScanPath      string        `json:"scan_path"`
	FindingsCount int           `json:"findings_count"`
	Findings      []ScanFinding `json:"findings"`
}

func (h *Handler) CreateScan(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")
	userID := auth.UserIDFromContext(r.Context())

	var req createScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.ScanPath == "" {
		apierror.BadRequest(w, "scan_path is required")
		return
	}

	scan, err := h.service.CreateScan(r.Context(), projectID, userID, req.ScanPath, req.Findings)
	if err != nil {
		log.Printf("Failed to create scan: %v", err)
		apierror.InternalError(w, "failed to create scan")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(scan) //nolint:errcheck
}

func (h *Handler) ListScans(w http.ResponseWriter, r *http.Request) {
	projectID := chi.URLParam(r, "project_id")

	scans, err := h.service.ListScans(r.Context(), projectID)
	if err != nil {
		log.Printf("Failed to list scans: %v", err)
		apierror.InternalError(w, "failed to list scans")
		return
	}
	if scans == nil {
		scans = []ScanResult{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"scans": scans}) //nolint:errcheck
}

func (h *Handler) ListFindings(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scan_id")
	userID := auth.UserIDFromContext(r.Context())
	if !h.checkScanAccess(r.Context(), w, scanID, userID, rbac.ActionRead) {
		return
	}

	findings, err := h.service.ListFindings(r.Context(), scanID)
	if err != nil {
		log.Printf("Failed to list findings: %v", err)
		apierror.InternalError(w, "failed to list findings")
		return
	}
	if findings == nil {
		findings = []ScanFinding{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"findings": findings}) //nolint:errcheck
}

type importFindingRequest struct {
	SecretID string `json:"secret_id"`
}

func (h *Handler) ImportFinding(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scan_id")
	findingID := chi.URLParam(r, "finding_id")
	userID := auth.UserIDFromContext(r.Context())
	if !h.checkScanAccess(r.Context(), w, scanID, userID, rbac.ActionWrite) {
		return
	}

	var req importFindingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.SecretID == "" {
		apierror.BadRequest(w, "secret_id is required")
		return
	}

	if err := h.service.ImportFinding(r.Context(), findingID, req.SecretID); err != nil {
		log.Printf("Failed to import finding: %v", err)
		apierror.InternalError(w, "failed to import finding")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "imported"}) //nolint:errcheck
}

func (h *Handler) DismissFinding(w http.ResponseWriter, r *http.Request) {
	scanID := chi.URLParam(r, "scan_id")
	findingID := chi.URLParam(r, "finding_id")
	userID := auth.UserIDFromContext(r.Context())
	if !h.checkScanAccess(r.Context(), w, scanID, userID, rbac.ActionWrite) {
		return
	}

	if err := h.service.DismissFinding(r.Context(), findingID); err != nil {
		log.Printf("Failed to dismiss finding: %v", err)
		apierror.InternalError(w, "failed to dismiss finding")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "dismissed"}) //nolint:errcheck
}

// checkScanAccess verifies the user has the required permission on the scan's project.
func (h *Handler) checkScanAccess(ctx context.Context, w http.ResponseWriter, scanID, userID string, action rbac.Action) bool {
	projectID, err := h.service.GetScanProjectID(ctx, scanID)
	if err != nil {
		apierror.NotFound(w, "scan not found")
		return false
	}
	var role string
	err = h.db.QueryRow(ctx, `SELECT role FROM project_memberships WHERE project_id = $1 AND user_id = $2`, projectID, userID).Scan(&role)
	if err != nil {
		apierror.Forbidden(w, "access denied")
		return false
	}
	if !rbac.Can(rbac.RoleFromProjectMembership(role), rbac.ResourceScans, action) {
		apierror.Forbidden(w, "insufficient permissions")
		return false
	}
	return true
}
