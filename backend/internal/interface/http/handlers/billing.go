// This file is the org-scoped billing surface (docs/06-BILLING.md, docs/08 §4).
// All routes run under TenantGuard with the billing object gate (ADMIN+); the
// handlers trust the resolved TenantContext and delegate to billinguc. Money is
// integer minor units; card data never transits here (Checkout/Portal only).
package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	mw "github.com/mesutokul/fluxboard/backend/internal/interface/http/middleware"
	"github.com/mesutokul/fluxboard/backend/internal/interface/http/response"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/billinguc"
)

// BillingHandlers serves the billing/subscription surface.
type BillingHandlers struct {
	svc    *billinguc.Service
	logger *slog.Logger
}

// NewBillingHandlers builds BillingHandlers.
func NewBillingHandlers(svc *billinguc.Service, logger *slog.Logger) *BillingHandlers {
	if logger == nil {
		logger = slog.Default()
	}
	return &BillingHandlers{svc: svc, logger: logger}
}

// ---- DTOs -----------------------------------------------------------------

type entitlementsResp struct {
	Plan               string `json:"plan"`
	Status             string `json:"status"`
	MaxMembers         int    `json:"max_members"`
	MaxProjects        int    `json:"max_projects"`
	MaxAgents          int    `json:"max_agents"`
	MaxStorageBytes    int64  `json:"max_storage_bytes"`
	APIRatePerMin      int    `json:"api_rate_per_min"`
	AuditRetentionDays int    `json:"audit_retention_days"`
	Metered            bool   `json:"metered"`
}

func toEntitlementsResp(e billing.Entitlements) entitlementsResp {
	return entitlementsResp{
		Plan: string(e.Plan), Status: string(e.Status),
		MaxMembers: e.MaxMembers, MaxProjects: e.MaxProjects, MaxAgents: e.MaxAgents,
		MaxStorageBytes: e.MaxStorageBytes, APIRatePerMin: e.APIRatePerMin,
		AuditRetentionDays: e.AuditRetentionDays, Metered: e.Metered,
	}
}

type summaryResp struct {
	Plan              string           `json:"plan"`
	Status            string           `json:"status"`
	CurrentPeriodEnd  *time.Time       `json:"current_period_end,omitempty"`
	CancelAtPeriodEnd bool             `json:"cancel_at_period_end"`
	PastDueWarning    bool             `json:"past_due_warning"`
	HasSubscription   bool             `json:"has_subscription"`
	Entitlements      entitlementsResp `json:"entitlements"`
}

type invoiceResp struct {
	ID              string     `json:"id"`
	StripeInvoiceID string     `json:"stripe_invoice_id"`
	Number          string     `json:"number"`
	Status          string     `json:"status"`
	AmountDue       int64      `json:"amount_due"`
	AmountPaid      int64      `json:"amount_paid"`
	Currency        string     `json:"currency"`
	HostedPDFURL    string     `json:"hosted_pdf_url,omitempty"`
	PeriodStart     *time.Time `json:"period_start,omitempty"`
	PeriodEnd       *time.Time `json:"period_end,omitempty"`
	CreatedAt       time.Time  `json:"created_at"`
}

func toInvoiceResp(i billing.Invoice) invoiceResp {
	return invoiceResp{
		ID: i.ID, StripeInvoiceID: i.StripeInvoiceID, Number: i.Number, Status: i.Status,
		AmountDue: i.AmountDue, AmountPaid: i.AmountPaid, Currency: i.Currency,
		HostedPDFURL: i.HostedPDFURL, PeriodStart: i.PeriodStart, PeriodEnd: i.PeriodEnd,
		CreatedAt: i.CreatedAt,
	}
}

// ---- Read -----------------------------------------------------------------

// Summary returns the billing page snapshot (FR-BILL-001).
func (h *BillingHandlers) Summary(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	v, err := h.svc.Summary(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, summaryResp{
		Plan: string(v.Plan), Status: string(v.Status),
		CurrentPeriodEnd: v.CurrentPeriodEnd, CancelAtPeriodEnd: v.CancelAtPeriodEnd,
		PastDueWarning: v.PastDueWarning, HasSubscription: v.HasSubscription,
		Entitlements: toEntitlementsResp(v.Entitlements),
	})
}

// ListInvoices returns the org's invoice mirror, newest-first (FR-BILL-008).
func (h *BillingHandlers) ListInvoices(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	invs, err := h.svc.ListInvoices(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	out := make([]invoiceResp, 0, len(invs))
	for i := range invs {
		out = append(out, toInvoiceResp(invs[i]))
	}
	response.JSON(w, http.StatusOK, map[string]any{"invoices": out})
}

// ---- Plan catalog (public) -------------------------------------------------

type catalogPlanResp struct {
	Code               string `json:"code"`
	Name               string `json:"name"`
	SeatPrice          int64  `json:"seat_price"`  // minor units, per billable seat per interval
	MaxMembers         int    `json:"max_members"` // -1 = unlimited
	MaxProjects        int    `json:"max_projects"`
	MaxAgents          int    `json:"max_agents"` // agents are not seats (ADR-033)
	MaxStorageBytes    int64  `json:"max_storage_bytes"`
	APIRatePerMin      int    `json:"api_rate_per_min"`
	AuditRetentionDays int    `json:"audit_retention_days"`
	Metered            bool   `json:"metered"` // storage + API calls are measured (usage prices: $0 for now)
}

type catalogResp struct {
	Version  string            `json:"version"`
	Currency string            `json:"currency"`
	Interval string            `json:"interval"`
	Pricing  string            `json:"pricing"` // "per_seat": billable (non-guest) members
	Plans    []catalogPlanResp `json:"plans"`
}

// Catalog serves the plan catalog to the marketing site and the app (report
// B02): public, no tenant. Prices and limits come from the plans table only.
func (h *BillingHandlers) Catalog(w http.ResponseWriter, r *http.Request) {
	plans, err := h.svc.Catalog(r.Context())
	if err != nil {
		response.Error(w, err)
		return
	}
	out := catalogResp{
		Version: billing.CatalogVersion, Currency: billing.CatalogCurrency,
		Interval: billing.CatalogInterval, Pricing: "per_seat",
		Plans: make([]catalogPlanResp, 0, len(plans)),
	}
	for _, p := range plans {
		out.Plans = append(out.Plans, catalogPlanResp{
			Code: string(p.Code), Name: p.Name, SeatPrice: p.MonthlyPrice,
			MaxMembers: p.MaxMembers, MaxProjects: p.MaxProjects, MaxAgents: p.MaxAgents,
			MaxStorageBytes: p.MaxStorageBytes, APIRatePerMin: p.APIRatePerMin, AuditRetentionDays: p.AuditRetentionDays, Metered: p.Metered,
		})
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	response.JSON(w, http.StatusOK, out)
}

// ---- Subscription lifecycle -----------------------------------------------

// checkoutReq names the plan; the seat count is the org's billable members,
// counted server-side (report B02).
type checkoutReq struct {
	Plan string `json:"plan"`
}

type checkoutResp struct {
	URL string `json:"url"`
}

// Checkout starts a hosted Stripe Checkout for a paid plan (FR-BILL-002).
func (h *BillingHandlers) Checkout(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req checkoutReq
	if !decodeJSON(w, r, &req) {
		return
	}
	sess, err := h.svc.Checkout(r.Context(), tc.OrgID, billing.PlanCode(req.Plan))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, checkoutResp{URL: sess.URL})
}

type planReq struct {
	Plan string `json:"plan"`
}

type previewResp struct {
	Amount   int64  `json:"amount"`
	Currency string `json:"currency"`
}

// PreviewChange returns the prorated amount for switching plans (FR-BILL-003).
func (h *BillingHandlers) PreviewChange(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req planReq
	if !decodeJSON(w, r, &req) {
		return
	}
	amount, currency, err := h.svc.PreviewChange(r.Context(), tc.OrgID, billing.PlanCode(req.Plan))
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, previewResp{Amount: amount, Currency: currency})
}

// ApplyChange switches the org to a new paid plan (FR-BILL-003).
func (h *BillingHandlers) ApplyChange(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req planReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.ApplyChange(r.Context(), tc.OrgID, billing.PlanCode(req.Plan)); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type cancelReq struct {
	AtPeriodEnd bool `json:"at_period_end"`
}

// Cancel schedules or applies a subscription cancellation (FR-BILL-004).
func (h *BillingHandlers) Cancel(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	var req cancelReq
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := h.svc.Cancel(r.Context(), tc.OrgID, req.AtPeriodEnd); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Resume clears a scheduled at-period-end cancellation (FR-BILL-004).
func (h *BillingHandlers) Resume(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	if err := h.svc.Resume(r.Context(), tc.OrgID); err != nil {
		response.Error(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type portalResp struct {
	URL string `json:"url"`
}

// Portal returns a Stripe Billing Portal URL for payment-method management
// (FR-BILL-006).
func (h *BillingHandlers) Portal(w http.ResponseWriter, r *http.Request) {
	tc, ok := mw.TenantFrom(r.Context())
	if !ok {
		response.Error(w, domain.ErrForbidden)
		return
	}
	url, err := h.svc.Portal(r.Context(), tc.OrgID)
	if err != nil {
		response.Error(w, err)
		return
	}
	response.JSON(w, http.StatusOK, portalResp{URL: url})
}
