package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
	"github.com/mesutokul/fluxboard/backend/internal/usecase/billinguc"
)

type catalogPlans []billing.Plan

func (c catalogPlans) Get(_ context.Context, code billing.PlanCode) (*billing.Plan, error) {
	for _, p := range c {
		if p.Code == code {
			return &p, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (c catalogPlans) List(context.Context) ([]billing.Plan, error) {
	return append([]billing.Plan(nil), c...), nil
}

// GET /plans serves the catalog from the plans table, cheapest first, with the
// versioned per-seat pricing contract (report B02).
func TestCatalogServesThePlansTable(t *testing.T) {
	plans := catalogPlans{ // table order differs from price order on purpose
		{Code: billing.PlanBusiness, Name: "Business", MonthlyPrice: 4900, MaxMembers: -1, MaxProjects: -1, MaxStorageBytes: 500 << 30, Metered: true},
		{Code: billing.PlanFree, Name: "Free", MonthlyPrice: 0, MaxMembers: 5, MaxProjects: 3, MaxStorageBytes: 2 << 30},
		{Code: billing.PlanPro, Name: "Pro", MonthlyPrice: 1200, MaxMembers: 25, MaxProjects: 50, MaxStorageBytes: 50 << 30},
	}
	h := NewBillingHandlers(billinguc.New(billinguc.Deps{Plans: plans}), slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	h.Catalog(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/api/v1/plans", nil))

	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "public, max-age=300" {
		t.Fatalf("status %d cache %q", rec.Code, rec.Header().Get("Cache-Control"))
	}
	var got catalogResp
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Version != billing.CatalogVersion || got.Currency != "usd" || got.Interval != "month" || got.Pricing != "per_seat" {
		t.Fatalf("contract = %+v", got)
	}
	codes := []string{}
	for _, p := range got.Plans {
		codes = append(codes, p.Code)
	}
	if len(codes) != 3 || codes[0] != "free" || codes[1] != "pro" || codes[2] != "business" {
		t.Fatalf("order = %v, want free, pro, business", codes)
	}
	if pro := got.Plans[1]; pro.SeatPrice != 1200 || pro.MaxMembers != 25 || pro.MaxProjects != 50 || pro.MaxStorageBytes != 50<<30 || pro.Metered {
		t.Fatalf("pro = %+v", pro)
	}
	if biz := got.Plans[2]; biz.SeatPrice != 4900 || biz.MaxMembers != -1 || !biz.Metered {
		t.Fatalf("business = %+v", biz)
	}
}
