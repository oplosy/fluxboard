package stripex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	stripe "github.com/stripe/stripe-go/v86"
	"github.com/stripe/stripe-go/v86/webhook"

	"github.com/mesutokul/fluxboard/backend/internal/domain"
	"github.com/mesutokul/fluxboard/backend/internal/domain/billing"
)

// Meter event names for the usage-billed dimensions (06 §5, ADR-029). The
// catalog seed (SeedCatalog) creates a Billing Meter for each; active members
// are billed as licensed seats, not by usage.
const (
	MeterStorageBytes = "fluxboard_storage_bytes" // aggregation "last": the latest daily gauge
	MeterAPICalls     = "fluxboard_api_calls"     // aggregation "sum": daily totals add up
)

// meterFor maps a usage dimension to its meter event name ("" = not billed by usage).
func meterFor(m billing.UsageMetric) string {
	switch m {
	case billing.MetricStorageBytes:
		return MeterStorageBytes
	case billing.MetricAPICalls:
		return MeterAPICalls
	default:
		return ""
	}
}

// webhookTolerance is the maximum signature age accepted (06 §4).
const webhookTolerance = 5 * time.Minute

// productPrefix + plan code is each paid plan's fixed Stripe product id. A
// subscription's plan is read from its seat price's product, so prices of an
// older catalog version (kept by existing customers) still map to their plan.
const productPrefix = "fluxboard_"

// ProductID returns the fixed Stripe product id of a plan.
func ProductID(code string) string { return productPrefix + code }

// planFromProduct maps a product id back to its plan ("" if not one of ours).
func planFromProduct(id string) billing.PlanCode {
	code := billing.PlanCode(strings.TrimPrefix(id, productPrefix))
	if !strings.HasPrefix(id, productPrefix) || !code.Valid() || code == billing.PlanFree {
		return ""
	}
	return code
}

// LiveGateway is the MODE=live adapter over stripe-go (pinned API version:
// stripe.APIVersion). Webhook payloads are only trusted for the event id, type,
// creation time and object id; the object itself is re-read from the API, so
// the event's API version (the account default for `stripe listen`) never
// matters and the mirror always gets the current state.
type LiveGateway struct {
	sc            *stripe.Client
	webhookSecret string
}

var _ billing.StripeGateway = (*LiveGateway)(nil)

// NewLive builds the live gateway. secretKey must be a Stripe secret or
// restricted key; webhookSecret the endpoint's signing secret (whsec_…).
func NewLive(secretKey, webhookSecret string, opts ...stripe.ClientOption) (*LiveGateway, error) {
	if !validSecretKey(secretKey) {
		return nil, errors.New("stripex: STRIPE_SECRET_KEY must be a Stripe secret (sk_…) or restricted (rk_…) key in MODE=live")
	}
	if !strings.HasPrefix(webhookSecret, "whsec_") {
		return nil, errors.New("stripex: STRIPE_WEBHOOK_SECRET must be the endpoint signing secret (whsec_…) in MODE=live")
	}
	return &LiveGateway{
		sc:            stripe.NewClient(secretKey, opts...),
		webhookSecret: webhookSecret,
	}, nil
}

func validSecretKey(k string) bool {
	for _, p := range []string{"sk_test_", "sk_live_", "rk_test_", "rk_live_"} {
		if strings.HasPrefix(k, p) && len(k) > len(p) {
			return true
		}
	}
	return false
}

// ---- outbound calls --------------------------------------------------------

// EnsureCustomer creates the org's customer (metadata org_id) once.
func (g *LiveGateway) EnsureCustomer(ctx context.Context, orgID, existing string) (string, error) {
	if existing != "" {
		return existing, nil
	}
	params := &stripe.CustomerCreateParams{Metadata: map[string]string{"org_id": orgID}}
	params.SetIdempotencyKey("cust:" + orgID)
	c, err := g.sc.V1Customers.Create(ctx, params)
	if err != nil {
		return "", fmt.Errorf("stripe: create customer: %w", err)
	}
	return c.ID, nil
}

// CreateCheckout opens a hosted Checkout session for the plan's seat price
// (× seats) plus, for metered plans, its usage prices (06 §3).
func (g *LiveGateway) CreateCheckout(ctx context.Context, p billing.CheckoutParams) (billing.CheckoutSession, error) {
	if p.Plan.SeatPriceID == "" {
		return billing.CheckoutSession{}, fmt.Errorf("stripe: plan %s has no seat price; run stripe-seed", p.Plan.Code)
	}
	if p.Seats < 1 {
		p.Seats = 1
	}
	items := []*stripe.CheckoutSessionCreateLineItemParams{
		{Price: stripe.String(p.Plan.SeatPriceID), Quantity: stripe.Int64(p.Seats)},
	}
	for _, price := range meteredPrices(p.Plan) {
		items = append(items, &stripe.CheckoutSessionCreateLineItemParams{Price: stripe.String(price)})
	}
	meta := map[string]string{"org_id": p.OrgID, "plan_code": string(p.Plan.Code)}
	params := &stripe.CheckoutSessionCreateParams{
		Customer:          stripe.String(p.CustomerID),
		Mode:              stripe.String(string(stripe.CheckoutSessionModeSubscription)),
		LineItems:         items,
		SuccessURL:        stripe.String(p.SuccessURL),
		CancelURL:         stripe.String(p.CancelURL),
		ClientReferenceID: stripe.String(p.OrgID),
		Metadata:          meta,
		SubscriptionData:  &stripe.CheckoutSessionCreateSubscriptionDataParams{Metadata: meta},
	}
	params.SetIdempotencyKey(p.IdempotencyKey)
	s, err := g.sc.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return billing.CheckoutSession{}, fmt.Errorf("stripe: create checkout session: %w", err)
	}
	return billing.CheckoutSession{URL: s.URL, CustomerID: p.CustomerID}, nil
}

// UpcomingInvoice previews the prorated invoice for moving to newPlan.
func (g *LiveGateway) UpcomingInvoice(ctx context.Context, subscriptionID string, newPlan billing.Plan, seats int64) (int64, string, error) {
	sub, err := g.retrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return 0, "", err
	}
	changes, err := planChange(sub, newPlan, seats)
	if err != nil {
		return 0, "", err
	}
	items := make([]*stripe.InvoiceCreatePreviewSubscriptionDetailsItemParams, 0, len(changes))
	for _, c := range changes {
		items = append(items, &stripe.InvoiceCreatePreviewSubscriptionDetailsItemParams{
			ID: c.ID, Price: c.Price, Quantity: c.Quantity, Deleted: c.Deleted,
		})
	}
	inv, err := g.sc.V1Invoices.CreatePreview(ctx, &stripe.InvoiceCreatePreviewParams{
		Subscription: stripe.String(subscriptionID),
		SubscriptionDetails: &stripe.InvoiceCreatePreviewSubscriptionDetailsParams{
			Items:             items,
			ProrationBehavior: stripe.String("create_prorations"),
		},
	})
	if err != nil {
		return 0, "", fmt.Errorf("stripe: preview invoice: %w", err)
	}
	return inv.AmountDue, string(inv.Currency), nil
}

// UpdateSubscription moves the subscription to newPlan with prorations.
func (g *LiveGateway) UpdateSubscription(ctx context.Context, subscriptionID string, newPlan billing.Plan, seats int64, idemKey string) error {
	sub, err := g.retrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	changes, err := planChange(sub, newPlan, seats)
	if err != nil {
		return err
	}
	items := make([]*stripe.SubscriptionUpdateItemParams, 0, len(changes))
	for _, c := range changes {
		items = append(items, &stripe.SubscriptionUpdateItemParams{
			ID: c.ID, Price: c.Price, Quantity: c.Quantity, Deleted: c.Deleted,
		})
	}
	params := &stripe.SubscriptionUpdateParams{
		Items:             items,
		ProrationBehavior: stripe.String("create_prorations"),
		Metadata:          map[string]string{"plan_code": string(newPlan.Code)},
	}
	params.SetIdempotencyKey(idemKey)
	if _, err := g.sc.V1Subscriptions.Update(ctx, subscriptionID, params); err != nil {
		return fmt.Errorf("stripe: update subscription: %w", err)
	}
	return nil
}

// SetSeats sets the licensed seat item's quantity with prorations.
func (g *LiveGateway) SetSeats(ctx context.Context, subscriptionID string, seats int64, idemKey string) error {
	if seats < 1 {
		return fmt.Errorf("stripe: seat count must be at least 1, got %d", seats)
	}
	sub, err := g.retrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return err
	}
	seat := seatItem(sub)
	if seat == nil {
		return fmt.Errorf("stripe: subscription %s has no licensed seat item", subscriptionID)
	}
	if seat.Quantity == seats {
		return nil
	}
	params := &stripe.SubscriptionUpdateParams{
		Items: []*stripe.SubscriptionUpdateItemParams{
			{ID: stripe.String(seat.ID), Quantity: stripe.Int64(seats)},
		},
		ProrationBehavior: stripe.String("create_prorations"),
	}
	params.SetIdempotencyKey(idemKey)
	if _, err := g.sc.V1Subscriptions.Update(ctx, subscriptionID, params); err != nil {
		return fmt.Errorf("stripe: set seats: %w", err)
	}
	return nil
}

// CancelSubscription cancels at period end (reversible with Resume) or now.
func (g *LiveGateway) CancelSubscription(ctx context.Context, subscriptionID string, atPeriodEnd bool) error {
	if atPeriodEnd {
		return g.setCancelAtPeriodEnd(ctx, subscriptionID, true)
	}
	if _, err := g.sc.V1Subscriptions.Cancel(ctx, subscriptionID, &stripe.SubscriptionCancelParams{}); err != nil {
		return fmt.Errorf("stripe: cancel subscription: %w", err)
	}
	return nil
}

// Resume clears a scheduled at-period-end cancellation.
func (g *LiveGateway) Resume(ctx context.Context, subscriptionID string) error {
	return g.setCancelAtPeriodEnd(ctx, subscriptionID, false)
}

func (g *LiveGateway) setCancelAtPeriodEnd(ctx context.Context, subscriptionID string, v bool) error {
	_, err := g.sc.V1Subscriptions.Update(ctx, subscriptionID, &stripe.SubscriptionUpdateParams{
		CancelAtPeriodEnd: stripe.Bool(v),
	})
	if err != nil {
		return fmt.Errorf("stripe: set cancel_at_period_end=%t: %w", v, err)
	}
	return nil
}

// PortalSession opens the hosted Billing Portal (payment methods, invoices).
func (g *LiveGateway) PortalSession(ctx context.Context, customerID, returnURL string) (string, error) {
	s, err := g.sc.V1BillingPortalSessions.Create(ctx, &stripe.BillingPortalSessionCreateParams{
		Customer:  stripe.String(customerID),
		ReturnURL: stripe.String(returnURL),
	})
	if err != nil {
		return "", fmt.Errorf("stripe: create portal session: %w", err)
	}
	return s.URL, nil
}

// PushUsage sends one meter event per customer/metric/day. The identifier
// makes a re-push of the same day a duplicate Stripe ignores (06 §5).
func (g *LiveGateway) PushUsage(ctx context.Context, customerID string, metric billing.UsageMetric, quantity int64, ts time.Time) error {
	event := meterFor(metric)
	if event == "" {
		return nil
	}
	if quantity < 0 {
		return fmt.Errorf("stripe: negative usage %d for %s", quantity, metric)
	}
	day := ts.UTC().Format("2006-01-02")
	_, err := g.sc.V1BillingMeterEvents.Create(ctx, &stripe.BillingMeterEventCreateParams{
		EventName:  stripe.String(event),
		Identifier: stripe.String(fmt.Sprintf("%s:%s:%s", customerID, metric, day)),
		Timestamp:  stripe.Int64(ts.UTC().Unix()),
		Payload: map[string]string{
			"stripe_customer_id": customerID,
			"value":              strconv.FormatInt(quantity, 10),
		},
	})
	if err != nil {
		return fmt.Errorf("stripe: meter event %s %s: %w", event, day, err)
	}
	return nil
}

// FetchSubscription reads the subscription for the nightly reconcile (06 §8).
func (g *LiveGateway) FetchSubscription(ctx context.Context, subscriptionID string) (*billing.StripeSubscription, error) {
	sub, err := g.retrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return nil, err
	}
	return normalizeSubscription(sub)
}

func (g *LiveGateway) retrieveSubscription(ctx context.Context, id string) (*stripe.Subscription, error) {
	sub, err := g.sc.V1Subscriptions.Retrieve(ctx, id, &stripe.SubscriptionRetrieveParams{})
	if err != nil {
		return nil, fmt.Errorf("stripe: retrieve subscription %s: %w", id, err)
	}
	return sub, nil
}

// ---- webhooks --------------------------------------------------------------

// ConstructEvent verifies the Stripe-Signature header (5 min tolerance) and
// normalizes the event by re-reading its object from the API. A bad signature
// is domain.ErrValidation (400); an API failure is a plain error (500, so
// Stripe retries).
func (g *LiveGateway) ConstructEvent(payload []byte, sigHeader string) (billing.StripeEvent, error) {
	ev, err := webhook.ConstructEventWithOptions(payload, sigHeader, g.webhookSecret, webhook.ConstructEventOptions{
		Tolerance:                webhookTolerance,
		IgnoreAPIVersionMismatch: true, // only id/type/created/object id are read from the payload
	})
	if err != nil {
		return billing.StripeEvent{}, fmt.Errorf("%w: stripe signature: %v", domain.ErrValidation, err)
	}
	out := billing.StripeEvent{ID: ev.ID, Type: string(ev.Type), Created: time.Unix(ev.Created, 0).UTC()}

	objectID, err := eventObjectID(ev)
	if err != nil {
		return billing.StripeEvent{}, fmt.Errorf("%w: %v", domain.ErrValidation, err)
	}
	// Webhook delivery has no request context; bound the API reads.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	switch out.Type {
	case "checkout.session.completed":
		s, err := g.sc.V1CheckoutSessions.Retrieve(ctx, objectID, &stripe.CheckoutSessionRetrieveParams{})
		if err != nil {
			return billing.StripeEvent{}, fmt.Errorf("stripe: retrieve checkout session: %w", err)
		}
		if s.Mode != stripe.CheckoutSessionModeSubscription || s.Subscription == nil || s.Subscription.ID == "" {
			out.OrgID = s.Metadata["org_id"]
			return out, nil // not a subscription checkout: recorded, no effect
		}
		return g.withSubscription(ctx, out, s.Subscription.ID)

	case "customer.subscription.updated", "customer.subscription.deleted":
		return g.withSubscription(ctx, out, objectID)

	case "invoice.finalized", "invoice.payment_succeeded", "invoice.payment_failed":
		inv, err := g.sc.V1Invoices.Retrieve(ctx, objectID, &stripe.InvoiceRetrieveParams{})
		if err != nil {
			return billing.StripeEvent{}, fmt.Errorf("stripe: retrieve invoice: %w", err)
		}
		orgID, err := g.invoiceOrg(ctx, inv)
		if err != nil {
			return billing.StripeEvent{}, err
		}
		out.OrgID = orgID
		out.Invoice = normalizeInvoice(inv)
		return out, nil

	default:
		// Unhandled types are recorded (handled=false); route by metadata when present.
		out.OrgID = objectMetadataOrg(ev)
		return out, nil
	}
}

func (g *LiveGateway) withSubscription(ctx context.Context, out billing.StripeEvent, subscriptionID string) (billing.StripeEvent, error) {
	sub, err := g.retrieveSubscription(ctx, subscriptionID)
	if err != nil {
		return billing.StripeEvent{}, err
	}
	ss, err := normalizeSubscription(sub)
	if err != nil {
		return billing.StripeEvent{}, err
	}
	orgID, err := g.subscriptionOrg(ctx, sub)
	if err != nil {
		return billing.StripeEvent{}, err
	}
	out.OrgID = orgID
	out.Sub = ss
	return out, nil
}

// subscriptionOrg routes by subscription metadata, falling back to the
// customer's metadata (a subscription created outside our Checkout). Never by email.
func (g *LiveGateway) subscriptionOrg(ctx context.Context, sub *stripe.Subscription) (string, error) {
	if id := sub.Metadata["org_id"]; id != "" {
		return id, nil
	}
	if sub.Customer == nil {
		return "", nil
	}
	return g.customerOrg(ctx, sub.Customer.ID)
}

func (g *LiveGateway) invoiceOrg(ctx context.Context, inv *stripe.Invoice) (string, error) {
	if inv.Parent != nil && inv.Parent.SubscriptionDetails != nil {
		if id := inv.Parent.SubscriptionDetails.Metadata["org_id"]; id != "" {
			return id, nil
		}
	}
	if inv.Customer == nil {
		return "", nil
	}
	return g.customerOrg(ctx, inv.Customer.ID)
}

func (g *LiveGateway) customerOrg(ctx context.Context, customerID string) (string, error) {
	if customerID == "" {
		return "", nil
	}
	c, err := g.sc.V1Customers.Retrieve(ctx, customerID, &stripe.CustomerRetrieveParams{})
	if err != nil {
		return "", fmt.Errorf("stripe: retrieve customer %s: %w", customerID, err)
	}
	return c.Metadata["org_id"], nil
}

// normalizeSubscription maps a Stripe subscription to the domain view; the plan
// comes from the seat price's product (fixed per plan, any catalog version).
func normalizeSubscription(sub *stripe.Subscription) (*billing.StripeSubscription, error) {
	seat := seatItem(sub)
	if seat == nil {
		return nil, fmt.Errorf("stripe: subscription %s has no licensed seat item", sub.ID)
	}
	var plan billing.PlanCode
	if seat.Price.Product != nil {
		plan = planFromProduct(seat.Price.Product.ID)
	}
	if plan == "" {
		return nil, fmt.Errorf("stripe: subscription %s seat price %s is not on a Fluxboard plan product", sub.ID, seat.Price.ID)
	}
	out := &billing.StripeSubscription{
		SubscriptionID:    sub.ID,
		Status:            mapStatus(sub.Status),
		PlanCode:          plan,
		CancelAtPeriodEnd: sub.CancelAtPeriodEnd,
		Seats:             seat.Quantity,
	}
	if sub.Customer != nil {
		out.CustomerID = sub.Customer.ID
	}
	// Billing periods live on the subscription items in this API version.
	if seat.CurrentPeriodEnd > 0 {
		t := time.Unix(seat.CurrentPeriodEnd, 0).UTC()
		out.CurrentPeriodEnd = &t
	}
	return out, nil
}

func normalizeInvoice(inv *stripe.Invoice) *billing.StripeInvoice {
	out := &billing.StripeInvoice{
		InvoiceID:    inv.ID,
		Number:       inv.Number,
		Status:       string(inv.Status),
		AmountDue:    inv.AmountDue,
		AmountPaid:   inv.AmountPaid,
		Currency:     string(inv.Currency),
		HostedPDFURL: inv.InvoicePDF,
	}
	if out.HostedPDFURL == "" {
		out.HostedPDFURL = inv.HostedInvoiceURL
	}
	if inv.PeriodStart > 0 {
		t := time.Unix(inv.PeriodStart, 0).UTC()
		out.PeriodStart = &t
	}
	if inv.PeriodEnd > 0 {
		t := time.Unix(inv.PeriodEnd, 0).UTC()
		out.PeriodEnd = &t
	}
	return out
}

// mapStatus folds Stripe's subscription statuses into the local state machine
// (06 §2): incomplete (first payment pending) grants nothing yet, an expired
// incomplete checkout is canceled, and a paused subscription loses paid
// entitlements like unpaid.
func mapStatus(s stripe.SubscriptionStatus) billing.SubStatus {
	switch s {
	case stripe.SubscriptionStatusActive:
		return billing.StatusActive
	case stripe.SubscriptionStatusTrialing:
		return billing.StatusTrialing
	case stripe.SubscriptionStatusPastDue:
		return billing.StatusPastDue
	case stripe.SubscriptionStatusUnpaid, stripe.SubscriptionStatusPaused:
		return billing.StatusUnpaid
	case stripe.SubscriptionStatusCanceled, stripe.SubscriptionStatusIncompleteExpired:
		return billing.StatusCanceled
	default: // incomplete
		return billing.StatusNone
	}
}

// eventObjectID reads data.object.id from the raw event.
func eventObjectID(ev stripe.Event) (string, error) {
	if ev.Data == nil || len(ev.Data.Raw) == 0 {
		return "", errors.New("stripe event has no data.object")
	}
	var obj struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(ev.Data.Raw, &obj); err != nil || obj.ID == "" {
		return "", errors.New("stripe event data.object has no id")
	}
	return obj.ID, nil
}

// objectMetadataOrg reads data.object.metadata.org_id when the object has one.
func objectMetadataOrg(ev stripe.Event) string {
	if ev.Data == nil {
		return ""
	}
	var obj struct {
		Metadata map[string]string `json:"metadata"`
	}
	if json.Unmarshal(ev.Data.Raw, &obj) != nil {
		return ""
	}
	return obj.Metadata["org_id"]
}

// ---- plan mapping ----------------------------------------------------------

// isMetered reports whether a subscription item is billed by usage.
func isMetered(it *stripe.SubscriptionItem) bool {
	return it.Price.Recurring != nil && it.Price.Recurring.UsageType == stripe.PriceRecurringUsageTypeMetered
}

// seatItem returns the subscription's licensed (quantity-based) item: the seats.
func seatItem(sub *stripe.Subscription) *stripe.SubscriptionItem {
	if sub.Items == nil {
		return nil
	}
	for _, it := range sub.Items.Data {
		if it != nil && it.Price != nil && !isMetered(it) {
			return it
		}
	}
	return nil
}

// meteredPrices returns the usage prices a metered plan adds to a subscription.
func meteredPrices(p billing.Plan) []string {
	if !p.Metered {
		return nil
	}
	var out []string
	for _, id := range []string{p.MeteredStoragePriceID, p.MeteredAPIPriceID} {
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

// itemChange is one subscription item edit (shared by preview and update).
type itemChange struct {
	ID       *string
	Price    *string
	Quantity *int64
	Deleted  *bool
}

// planChange computes the item edits that move sub to newPlan: the seat item
// switches price (keeping its quantity when seats <= 0), usage prices the new
// plan needs are added, and usage prices it does not bill are removed.
func planChange(sub *stripe.Subscription, newPlan billing.Plan, seats int64) ([]itemChange, error) {
	if newPlan.SeatPriceID == "" {
		return nil, fmt.Errorf("stripe: plan %s has no seat price; run stripe-seed", newPlan.Code)
	}
	if sub.Items == nil {
		return nil, fmt.Errorf("stripe: subscription %s has no items", sub.ID)
	}
	want := map[string]bool{}
	for _, id := range meteredPrices(newPlan) {
		want[id] = true
	}
	var (
		changes []itemChange
		seat    *stripe.SubscriptionItem
		have    = map[string]bool{}
	)
	for _, it := range sub.Items.Data {
		if it == nil || it.Price == nil {
			continue
		}
		// The licensed item (quantity-based) is the seat; usage-based items are metered.
		if isMetered(it) {
			have[it.Price.ID] = true
			if !want[it.Price.ID] {
				changes = append(changes, itemChange{ID: stripe.String(it.ID), Deleted: stripe.Bool(true)})
			}
			continue
		}
		if seat != nil {
			return nil, fmt.Errorf("stripe: subscription %s has more than one licensed item", sub.ID)
		}
		seat = it
	}
	if seat == nil {
		return nil, fmt.Errorf("stripe: subscription %s has no licensed seat item", sub.ID)
	}
	qty := seats
	if qty <= 0 {
		qty = seat.Quantity
	}
	if qty < 1 {
		qty = 1
	}
	changes = append(changes, itemChange{
		ID: stripe.String(seat.ID), Price: stripe.String(newPlan.SeatPriceID), Quantity: stripe.Int64(qty),
	})
	for _, id := range meteredPrices(newPlan) {
		if !have[id] {
			changes = append(changes, itemChange{Price: stripe.String(id)})
		}
	}
	return changes, nil
}
