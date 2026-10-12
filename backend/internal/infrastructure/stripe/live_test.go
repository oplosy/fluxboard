package stripex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
)

const liveWebhookSecret = "whsec_live_test"

// fakePlans serves the plan catalog with seeded Stripe price ids.
type fakePlans struct{}

func (fakePlans) Get(_ context.Context, c billing.PlanCode) (*billing.Plan, error) {
	for _, p := range testPlans() {
		if p.Code == c {
			return &p, nil
		}
	}
	return nil, domain.ErrNotFound
}
func (fakePlans) List(context.Context) ([]billing.Plan, error) { return testPlans(), nil }

func testPlans() []billing.Plan {
	return []billing.Plan{
		{Code: billing.PlanFree},
		{Code: billing.PlanPro, SeatPriceID: "price_pro_seat"},
		{Code: billing.PlanBusiness, SeatPriceID: "price_biz_seat", Metered: true,
			MeteredStoragePriceID: "price_biz_storage", MeteredAPIPriceID: "price_biz_api"},
	}
}

// fakeStripe is a minimal Stripe API: canned objects by path, and a record of
// every request (method, path, form, idempotency key) for assertions.
type fakeStripe struct {
	mu       sync.Mutex
	objects  map[string]string // "GET /v1/…" or "POST /v1/…" → JSON body
	requests []recorded
}

type recorded struct {
	method, path, idemKey string
	form                  url.Values
}

func (f *fakeStripe) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	form, _ := url.ParseQuery(string(body))
	f.mu.Lock()
	f.requests = append(f.requests, recorded{r.Method, r.URL.Path, r.Header.Get("Idempotency-Key"), form})
	resp, ok := f.objects[r.Method+" "+r.URL.Path]
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	if !ok {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, `{"error":{"type":"invalid_request_error","code":"resource_missing","message":"no such object"}}`)
		return
	}
	fmt.Fprint(w, resp)
}

func (f *fakeStripe) last(method, path string) (recorded, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i := len(f.requests) - 1; i >= 0; i-- {
		if f.requests[i].method == method && f.requests[i].path == path {
			return f.requests[i], true
		}
	}
	return recorded{}, false
}

func newLiveAgainst(t *testing.T, f *fakeStripe) *LiveGateway {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	backend := stripe.GetBackendWithConfig(stripe.APIBackend, &stripe.BackendConfig{
		URL:               stripe.String(srv.URL),
		MaxNetworkRetries: stripe.Int64(0),
		LeveledLogger:     &stripe.LeveledLogger{Level: stripe.LevelNull},
	})
	g, err := NewLive("sk_test_fake", liveWebhookSecret, fakePlans{},
		stripe.WithBackends(&stripe.Backends{API: backend, Connect: backend, Uploads: backend, MeterEvents: backend}))
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// subscriptionJSON is a subscription in the pinned API shape: billing periods
// on the items, customer as an id.
func subscriptionJSON(id, status, seatPrice string, qty int64, metadata string, metered ...string) string {
	items := []string{fmt.Sprintf(`{"id":"si_seat","object":"subscription_item","quantity":%d,"current_period_end":1767225600,`+
		`"price":{"id":%q,"object":"price","recurring":{"interval":"month","usage_type":"licensed"}}}`, qty, seatPrice)}
	for i, p := range metered {
		items = append(items, fmt.Sprintf(`{"id":"si_m%d","object":"subscription_item","current_period_end":1767225600,`+
			`"price":{"id":%q,"object":"price","recurring":{"interval":"month","usage_type":"metered"}}}`, i, p))
	}
	return fmt.Sprintf(`{"id":%q,"object":"subscription","status":%q,"customer":"cus_1","cancel_at_period_end":false,`+
		`"metadata":%s,"items":{"object":"list","data":[%s]}}`, id, status, metadata, strings.Join(items, ","))
}

func signed(t *testing.T, payload string, at time.Time) (string, []byte) {
	t.Helper()
	sp := webhook.GenerateTestSignedPayload(&webhook.UnsignedPayload{
		Payload: []byte(payload), Secret: liveWebhookSecret, Timestamp: at,
	})
	return sp.Header, sp.Payload
}

func TestConstructEventCheckoutReadsSubscriptionFromAPI(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"GET /v1/checkout/sessions/cs_1": `{"id":"cs_1","object":"checkout.session","mode":"subscription","subscription":"sub_1","metadata":{"org_id":"org-1"}}`,
		"GET /v1/subscriptions/sub_1":    subscriptionJSON("sub_1", "active", "price_pro_seat", 4, `{"org_id":"org-1"}`),
	}}
	g := newLiveAgainst(t, f)
	// An older API version and a stale object body: only id/type/created/object id are used.
	header, body := signed(t, `{"id":"evt_1","object":"event","api_version":"2024-06-20","type":"checkout.session.completed",`+
		`"created":1767000000,"data":{"object":{"id":"cs_1","object":"checkout.session","status":"open"}}}`, time.Now())

	ev, err := g.ConstructEvent(body, header)
	if err != nil {
		t.Fatal(err)
	}
	if ev.ID != "evt_1" || ev.OrgID != "org-1" || !ev.Created.Equal(time.Unix(1767000000, 0)) {
		t.Fatalf("event = %+v", ev)
	}
	want := billing.StripeSubscription{SubscriptionID: "sub_1", CustomerID: "cus_1", Status: billing.StatusActive, PlanCode: billing.PlanPro}
	if ev.Sub == nil || ev.Sub.SubscriptionID != want.SubscriptionID || ev.Sub.CustomerID != want.CustomerID ||
		ev.Sub.Status != want.Status || ev.Sub.PlanCode != want.PlanCode {
		t.Fatalf("sub = %+v, want %+v", ev.Sub, want)
	}
	if ev.Sub.CurrentPeriodEnd == nil || ev.Sub.CurrentPeriodEnd.Unix() != 1767225600 {
		t.Fatalf("period end = %v, want the seat item's current_period_end", ev.Sub.CurrentPeriodEnd)
	}
}

func TestConstructEventRejectsBadSignatures(t *testing.T) {
	g := newLiveAgainst(t, &fakeStripe{})
	payload := `{"id":"evt_1","object":"event","type":"invoice.paid","created":1,"data":{"object":{"id":"in_1"}}}`

	header, body := signed(t, payload, time.Now())
	tampered := []byte(strings.Replace(string(body), "in_1", "in_2", 1))
	if _, err := g.ConstructEvent(tampered, header); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("tampered body: err = %v, want ErrValidation", err)
	}
	staleHeader, staleBody := signed(t, payload, time.Now().Add(-10*time.Minute))
	if _, err := g.ConstructEvent(staleBody, staleHeader); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("signature older than 5 min: err = %v, want ErrValidation", err)
	}
	if _, err := g.ConstructEvent(body, ""); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("missing header: err = %v, want ErrValidation", err)
	}
}

func TestConstructEventRoutesByCustomerWhenMetadataMissing(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"GET /v1/subscriptions/sub_2": subscriptionJSON("sub_2", "past_due", "price_biz_seat", 9, `{}`, "price_biz_storage", "price_biz_api"),
		"GET /v1/customers/cus_1":     `{"id":"cus_1","object":"customer","metadata":{"org_id":"org-9"}}`,
	}}
	g := newLiveAgainst(t, f)
	header, body := signed(t, `{"id":"evt_2","object":"event","type":"customer.subscription.updated","created":1767000001,`+
		`"data":{"object":{"id":"sub_2","object":"subscription"}}}`, time.Now())

	ev, err := g.ConstructEvent(body, header)
	if err != nil {
		t.Fatal(err)
	}
	if ev.OrgID != "org-9" || ev.Sub.PlanCode != billing.PlanBusiness || ev.Sub.Status != billing.StatusPastDue {
		t.Fatalf("event = %+v sub = %+v", ev, ev.Sub)
	}
}

func TestConstructEventInvoice(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"GET /v1/invoices/in_1": `{"id":"in_1","object":"invoice","number":"F-0001","status":"open","amount_due":4900,"amount_paid":0,` +
			`"currency":"usd","invoice_pdf":"https://pay.stripe.test/in_1.pdf","period_start":1764547200,"period_end":1767225600,` +
			`"customer":"cus_1","parent":{"type":"subscription_details","subscription_details":{"metadata":{"org_id":"org-1"}}}}`,
	}}
	g := newLiveAgainst(t, f)
	header, body := signed(t, `{"id":"evt_3","object":"event","type":"invoice.payment_failed","created":1767000002,`+
		`"data":{"object":{"id":"in_1","object":"invoice"}}}`, time.Now())

	ev, err := g.ConstructEvent(body, header)
	if err != nil {
		t.Fatal(err)
	}
	inv := ev.Invoice
	if ev.OrgID != "org-1" || inv == nil || inv.InvoiceID != "in_1" || inv.AmountDue != 4900 || inv.Currency != "usd" ||
		inv.HostedPDFURL != "https://pay.stripe.test/in_1.pdf" || inv.PeriodEnd == nil || inv.PeriodEnd.Unix() != 1767225600 {
		t.Fatalf("event = %+v invoice = %+v", ev, inv)
	}
}

func TestConstructEventUnknownPriceIsRetryable(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"GET /v1/subscriptions/sub_3": subscriptionJSON("sub_3", "active", "price_unknown", 1, `{"org_id":"org-1"}`),
	}}
	g := newLiveAgainst(t, f)
	header, body := signed(t, `{"id":"evt_4","object":"event","type":"customer.subscription.updated","created":1,`+
		`"data":{"object":{"id":"sub_3","object":"subscription"}}}`, time.Now())

	_, err := g.ConstructEvent(body, header)
	if err == nil || errors.Is(err, domain.ErrValidation) {
		t.Fatalf("unknown seat price: err = %v, want a retryable (non-validation) error", err)
	}
}

func TestUpdateSubscriptionKeepsSeatsAndDropsUsagePrices(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"GET /v1/subscriptions/sub_1":  subscriptionJSON("sub_1", "active", "price_biz_seat", 7, `{"org_id":"org-1"}`, "price_biz_storage", "price_biz_api"),
		"POST /v1/subscriptions/sub_1": subscriptionJSON("sub_1", "active", "price_pro_seat", 7, `{"org_id":"org-1"}`),
	}}
	g := newLiveAgainst(t, f)
	pro, _ := fakePlans{}.Get(context.Background(), billing.PlanPro)

	if err := g.UpdateSubscription(context.Background(), "sub_1", *pro, 0, "chg:org-1:pro:1"); err != nil {
		t.Fatal(err)
	}
	req, ok := f.last(http.MethodPost, "/v1/subscriptions/sub_1")
	if !ok {
		t.Fatal("no update request")
	}
	form := req.form
	if req.idemKey != "chg:org-1:pro:1" || form.Get("proration_behavior") != "create_prorations" || form.Get("metadata[plan_code]") != "pro" {
		t.Fatalf("update = idem %q form %v", req.idemKey, form)
	}
	got := map[string]string{}
	for i := range 3 {
		p := fmt.Sprintf("items[%d]", i)
		got[form.Get(p+"[id]")] = form.Get(p+"[price]") + "|" + form.Get(p+"[quantity]") + "|" + form.Get(p+"[deleted]")
	}
	want := map[string]string{"si_seat": "price_pro_seat|7|", "si_m0": "||true", "si_m1": "||true"}
	for id, w := range want {
		if got[id] != w {
			t.Errorf("item %s = %q, want %q (all: %v)", id, got[id], w, got)
		}
	}
}

func TestCreateCheckoutAddsUsagePricesForMeteredPlans(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"POST /v1/checkout/sessions": `{"id":"cs_9","object":"checkout.session","url":"https://checkout.stripe.test/cs_9"}`,
	}}
	g := newLiveAgainst(t, f)
	biz, _ := fakePlans{}.Get(context.Background(), billing.PlanBusiness)

	s, err := g.CreateCheckout(context.Background(), billing.CheckoutParams{
		OrgID: "org-1", Plan: *biz, Seats: 5, CustomerID: "cus_1", IdempotencyKey: "co:org-1:business:9",
		SuccessURL: "https://app.test/ok", CancelURL: "https://app.test/no",
	})
	if err != nil {
		t.Fatal(err)
	}
	req, _ := f.last(http.MethodPost, "/v1/checkout/sessions")
	form := req.form
	if s.URL != "https://checkout.stripe.test/cs_9" || req.idemKey != "co:org-1:business:9" ||
		form.Get("mode") != "subscription" || form.Get("customer") != "cus_1" ||
		form.Get("line_items[0][price]") != "price_biz_seat" || form.Get("line_items[0][quantity]") != "5" ||
		form.Get("line_items[1][price]") != "price_biz_storage" || form.Get("line_items[2][price]") != "price_biz_api" ||
		form.Get("line_items[1][quantity]") != "" || form.Get("subscription_data[metadata][org_id]") != "org-1" {
		t.Fatalf("checkout = %+v idem %q form %v", s, req.idemKey, form)
	}
}

func TestPushUsageSendsMeterEvents(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"POST /v1/billing/meter_events": `{"object":"billing.meter_event","event_name":"fluxboard_storage_bytes"}`,
	}}
	g := newLiveAgainst(t, f)
	day := time.Date(2026, 10, 11, 0, 0, 0, 0, time.UTC)

	if err := g.PushUsage(context.Background(), "cus_1", billing.MetricStorageBytes, 1<<30, day); err != nil {
		t.Fatal(err)
	}
	req, ok := f.last(http.MethodPost, "/v1/billing/meter_events")
	if !ok || req.form.Get("event_name") != MeterStorageBytes || req.form.Get("identifier") != "cus_1:storage_bytes:2026-10-11" ||
		req.form.Get("payload[stripe_customer_id]") != "cus_1" || req.form.Get("payload[value]") != "1073741824" ||
		req.form.Get("timestamp") != fmt.Sprint(day.Unix()) {
		t.Fatalf("meter event = %v", req.form)
	}

	n := len(f.requests)
	if err := g.PushUsage(context.Background(), "cus_1", billing.MetricActiveMembers, 12, day); err != nil {
		t.Fatal(err)
	}
	if len(f.requests) != n {
		t.Error("active members are billed as seats; no meter event expected")
	}
}

func TestUpcomingInvoicePreviewsTheChange(t *testing.T) {
	f := &fakeStripe{objects: map[string]string{
		"GET /v1/subscriptions/sub_1":      subscriptionJSON("sub_1", "active", "price_pro_seat", 3, `{"org_id":"org-1"}`),
		"POST /v1/invoices/create_preview": `{"object":"invoice","amount_due":11100,"currency":"usd"}`,
	}}
	g := newLiveAgainst(t, f)
	biz, _ := fakePlans{}.Get(context.Background(), billing.PlanBusiness)

	amount, currency, err := g.UpcomingInvoice(context.Background(), "sub_1", *biz, 0)
	if err != nil || amount != 11100 || currency != "usd" {
		t.Fatalf("preview = %d %s %v", amount, currency, err)
	}
	req, _ := f.last(http.MethodPost, "/v1/invoices/create_preview")
	form := req.form
	if form.Get("subscription") != "sub_1" || form.Get("subscription_details[proration_behavior]") != "create_prorations" ||
		form.Get("subscription_details[items][0][price]") != "price_biz_seat" ||
		form.Get("subscription_details[items][0][quantity]") != "3" ||
		form.Get("subscription_details[items][1][price]") != "price_biz_storage" ||
		form.Get("subscription_details[items][2][price]") != "price_biz_api" {
		t.Fatalf("preview form = %v", form)
	}
}

func TestMapStatus(t *testing.T) {
	cases := map[stripe.SubscriptionStatus]billing.SubStatus{
		stripe.SubscriptionStatusActive:            billing.StatusActive,
		stripe.SubscriptionStatusTrialing:          billing.StatusTrialing,
		stripe.SubscriptionStatusPastDue:           billing.StatusPastDue,
		stripe.SubscriptionStatusUnpaid:            billing.StatusUnpaid,
		stripe.SubscriptionStatusPaused:            billing.StatusUnpaid,
		stripe.SubscriptionStatusCanceled:          billing.StatusCanceled,
		stripe.SubscriptionStatusIncompleteExpired: billing.StatusCanceled,
		stripe.SubscriptionStatusIncomplete:        billing.StatusNone,
	}
	for in, want := range cases {
		if got := mapStatus(in); got != want {
			t.Errorf("mapStatus(%s) = %s, want %s", in, got, want)
		}
	}
}

func TestPlanChangeRejectsAmbiguousSubscriptions(t *testing.T) {
	var sub stripe.Subscription
	raw := subscriptionJSON("sub_1", "active", "price_pro_seat", 2, `{}`)
	if err := json.Unmarshal([]byte(raw), &sub); err != nil {
		t.Fatal(err)
	}
	sub.Items.Data = append(sub.Items.Data, sub.Items.Data[0]) // two licensed items
	if _, err := planChange(&sub, testPlans()[1], 0); err == nil {
		t.Error("two licensed items: want an error")
	}
	if _, err := planChange(&sub, billing.Plan{Code: billing.PlanPro}, 0); err == nil {
		t.Error("target plan without a seat price: want an error")
	}
}
