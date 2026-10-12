//go:build stripe_sandbox

// Sandbox suite: drives LiveGateway against a real Stripe *test-mode* account
// (report B01). It seeds the catalog (idempotent), then walks a subscription
// through the lifecycle the app uses: customer, Checkout session, plan preview
// and change (seat count kept), usage meter events, cancel at period end and
// resume, Billing Portal, reconcile fetch, immediate cancel.
//
//	STRIPE_SECRET_KEY=sk_test_… go test -tags=stripe_sandbox -count=1 ./internal/infrastructure/stripe/
//
// (make stripe-sandbox-test). A missing or non-test key fails the suite; the
// customer it creates is deleted at the end. Webhook signature verification is
// covered offline in live_test.go with Stripe's own signing scheme.
package stripex

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
)

// catalogPlans is an in-memory plan repository over the seeded Stripe ids.
type catalogPlans []billing.Plan

func (c catalogPlans) Get(_ context.Context, code billing.PlanCode) (*billing.Plan, error) {
	for _, p := range c {
		if p.Code == code {
			return &p, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (c catalogPlans) List(context.Context) ([]billing.Plan, error) { return c, nil }

func TestSandboxLifecycle(t *testing.T) {
	key := os.Getenv("STRIPE_SECRET_KEY")
	if !strings.HasPrefix(key, "sk_test_") && !strings.HasPrefix(key, "rk_test_") {
		t.Fatal("STRIPE_SECRET_KEY must be a Stripe test-mode key (sk_test_…) for the sandbox suite")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	sc := stripe.NewClient(key)

	ids, err := SeedCatalog(ctx, sc, "usd", []CatalogPlan{
		{Code: "pro", Name: "Pro", SeatUnitAmount: 1200},
		{Code: "business", Name: "Business", SeatUnitAmount: 4900, Metered: true},
	})
	if err != nil {
		t.Fatalf("seed catalog: %v", err)
	}
	plans := catalogPlans{
		{Code: billing.PlanFree},
		{Code: billing.PlanPro, SeatPriceID: ids["pro"].SeatPriceID},
		{Code: billing.PlanBusiness, SeatPriceID: ids["business"].SeatPriceID, Metered: true,
			MeteredStoragePriceID: ids["business"].StoragePriceID, MeteredAPIPriceID: ids["business"].APIPriceID},
	}
	g, err := NewLive(key, "whsec_not_used_by_this_suite")
	if err != nil {
		t.Fatal(err)
	}

	orgID := "sandbox-" + time.Now().UTC().Format("20060102T150405")
	customerID, err := g.EnsureCustomer(ctx, orgID, "")
	if err != nil {
		t.Fatalf("ensure customer: %v", err)
	}
	t.Cleanup(func() {
		_, _ = sc.V1Customers.Delete(context.Background(), customerID, &stripe.CustomerDeleteParams{})
	})
	if again, err := g.EnsureCustomer(ctx, orgID, ""); err != nil || again != customerID {
		t.Fatalf("EnsureCustomer is not idempotent: %s vs %s (%v)", again, customerID, err)
	}

	pro, _ := plans.Get(ctx, billing.PlanPro)
	biz, _ := plans.Get(ctx, billing.PlanBusiness)

	co, err := g.CreateCheckout(ctx, billing.CheckoutParams{
		OrgID: orgID, Plan: *biz, Seats: 3, CustomerID: customerID, IdempotencyKey: "co:" + orgID,
		SuccessURL: "https://app.example.com/ok?session_id={CHECKOUT_SESSION_ID}", CancelURL: "https://app.example.com/no",
	})
	if err != nil || !strings.HasPrefix(co.URL, "https://checkout.stripe.com/") {
		t.Fatalf("checkout = %+v, %v", co, err)
	}

	// Completing Checkout needs a browser; create the same subscription directly
	// with Stripe's test card so the rest of the lifecycle can run.
	if _, err := sc.V1PaymentMethods.Attach(ctx, "pm_card_visa", &stripe.PaymentMethodAttachParams{Customer: stripe.String(customerID)}); err != nil {
		t.Fatalf("attach test card: %v", err)
	}
	pmList := sc.V1PaymentMethods.List(ctx, &stripe.PaymentMethodListParams{Customer: stripe.String(customerID)})
	var pmID string
	for pm, err := range pmList.All(ctx) {
		if err != nil {
			t.Fatal(err)
		}
		pmID = pm.ID
		break
	}
	sub, err := sc.V1Subscriptions.Create(ctx, &stripe.SubscriptionCreateParams{
		Customer:             stripe.String(customerID),
		DefaultPaymentMethod: stripe.String(pmID),
		Items:                []*stripe.SubscriptionCreateItemParams{{Price: stripe.String(pro.SeatPriceID), Quantity: stripe.Int64(3)}},
		Metadata:             map[string]string{"org_id": orgID, "plan_code": "pro"},
	})
	if err != nil {
		t.Fatalf("create subscription: %v", err)
	}

	fetch := func(stage string) *billing.StripeSubscription {
		t.Helper()
		s, err := g.FetchSubscription(ctx, sub.ID)
		if err != nil {
			t.Fatalf("%s: fetch: %v", stage, err)
		}
		return s
	}
	if s := fetch("created"); s.PlanCode != billing.PlanPro || s.Status != billing.StatusActive || s.CurrentPeriodEnd == nil ||
		s.CustomerID != customerID || s.Seats != 3 {
		t.Fatalf("created: %+v", s)
	}
	// Report B02: the first invoice is seats × seat price (3 × $12).
	first, err := sc.V1Invoices.Retrieve(ctx, sub.LatestInvoice.ID, &stripe.InvoiceRetrieveParams{})
	if err != nil {
		t.Fatal(err)
	}
	if first.AmountDue != 3*1200 || first.Currency != "usd" {
		t.Fatalf("first invoice = %d %s, want 3600 usd", first.AmountDue, first.Currency)
	}
	// Seat sync: a fourth member joins.
	if err := g.SetSeats(ctx, sub.ID, 4, "seats:"+orgID+":4"); err != nil {
		t.Fatalf("set seats: %v", err)
	}
	if s := fetch("seat added"); s.Seats != 4 {
		t.Fatalf("seats = %d, want 4", s.Seats)
	}

	amount, currency, err := g.UpcomingInvoice(ctx, sub.ID, *biz, 0)
	if err != nil || amount <= 0 || currency != "usd" {
		t.Fatalf("preview upgrade = %d %s %v, want a positive usd proration", amount, currency, err)
	}
	if err := g.UpdateSubscription(ctx, sub.ID, *biz, 0, "chg:"+orgID); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if s := fetch("upgraded"); s.PlanCode != billing.PlanBusiness {
		t.Fatalf("upgraded: plan %s, want business", s.PlanCode)
	}
	remote, err := sc.V1Subscriptions.Retrieve(ctx, sub.ID, &stripe.SubscriptionRetrieveParams{})
	if err != nil {
		t.Fatal(err)
	}
	var seats int64
	metered := 0
	for _, it := range remote.Items.Data {
		if it.Price.Recurring != nil && it.Price.Recurring.UsageType == stripe.PriceRecurringUsageTypeMetered {
			metered++
		} else {
			seats = it.Quantity
		}
	}
	if seats != 4 || metered != 2 {
		t.Fatalf("after upgrade: seats %d (want 4, kept), metered items %d (want 2)", seats, metered)
	}

	day := time.Now().UTC().Truncate(24 * time.Hour)
	for _, m := range []billing.UsageMetric{billing.MetricStorageBytes, billing.MetricAPICalls, billing.MetricActiveMembers} {
		if err := g.PushUsage(ctx, customerID, m, 1234, day); err != nil {
			t.Fatalf("push %s: %v", m, err)
		}
	}

	if err := g.CancelSubscription(ctx, sub.ID, true); err != nil {
		t.Fatalf("cancel at period end: %v", err)
	}
	if s := fetch("cancel scheduled"); !s.CancelAtPeriodEnd {
		t.Fatal("cancel_at_period_end not set")
	}
	if err := g.Resume(ctx, sub.ID); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if s := fetch("resumed"); s.CancelAtPeriodEnd {
		t.Fatal("cancel_at_period_end still set after resume")
	}

	portal, err := g.PortalSession(ctx, customerID, "https://app.example.com/billing")
	if err != nil {
		t.Errorf("portal: %v (save a Customer Portal configuration in the test-mode Dashboard once)", err)
	} else if !strings.HasPrefix(portal, "https://billing.stripe.com/") {
		t.Errorf("portal URL = %s", portal)
	}

	if err := g.CancelSubscription(ctx, sub.ID, false); err != nil {
		t.Fatalf("cancel now: %v", err)
	}
	if s := fetch("canceled"); s.Status != billing.StatusCanceled {
		t.Fatalf("canceled: status %s", s.Status)
	}
}
