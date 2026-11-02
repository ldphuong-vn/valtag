package billing

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/ldphuong-vn/valtag/server/internal/auth"
	"github.com/ldphuong-vn/valtag/server/internal/org"
	"github.com/ldphuong-vn/valtag/server/pkg/apierror"
)

// Handler serves billing HTTP endpoints.
type Handler struct {
	svc    *Service
	orgSvc *org.Service
}

// NewHandler creates a billing Handler.
func NewHandler(svc *Service, orgSvc *org.Service) *Handler {
	return &Handler{svc: svc, orgSvc: orgSvc}
}

type checkoutRequest struct {
	Plan       string `json:"plan"`
	SuccessURL string `json:"success_url"`
	CancelURL  string `json:"cancel_url"`
}

// CreateCheckout handles POST /billing/checkout-session.
func (h *Handler) CreateCheckout(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req checkoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.Plan != "pro" && req.Plan != "team" {
		apierror.BadRequest(w, "plan must be 'pro' or 'team'")
		return
	}
	if req.SuccessURL == "" || req.CancelURL == "" {
		apierror.BadRequest(w, "success_url and cancel_url are required")
		return
	}

	// Get user's owned org
	o, err := h.findOwnedOrg(r, userID)
	if err != nil {
		apierror.Forbidden(w, "only org owners can manage billing")
		return
	}

	url, err := h.svc.CreateCheckoutSession(r.Context(), o.ID, o.Name, "", req.Plan, req.SuccessURL, req.CancelURL)
	if err != nil {
		log.Printf("[billing] checkout error: %v", err)
		apierror.InternalError(w, "failed to create checkout session")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

type portalRequest struct {
	ReturnURL string `json:"return_url"`
}

// CreatePortal handles POST /billing/portal.
func (h *Handler) CreatePortal(w http.ResponseWriter, r *http.Request) {
	userID := auth.UserIDFromContext(r.Context())

	var req portalRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		apierror.BadRequest(w, "invalid request body")
		return
	}
	if req.ReturnURL == "" {
		apierror.BadRequest(w, "return_url is required")
		return
	}

	o, err := h.findOwnedOrg(r, userID)
	if err != nil {
		apierror.Forbidden(w, "only org owners can manage billing")
		return
	}

	url, err := h.svc.CreatePortalSession(r.Context(), o.ID, req.ReturnURL)
	if err != nil {
		log.Printf("[billing] portal error: %v", err)
		apierror.InternalError(w, "failed to create portal session")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"url": url})
}

// findOwnedOrg returns the first org the user owns.
func (h *Handler) findOwnedOrg(r *http.Request, userID string) (*org.Org, error) {
	orgs, err := h.orgSvc.ListByUser(r.Context(), userID)
	if err != nil {
		return nil, err
	}
	for _, o := range orgs {
		if o.OwnerID == userID {
			return &o, nil
		}
	}
	return nil, fmt.Errorf("no owned org found")
}
