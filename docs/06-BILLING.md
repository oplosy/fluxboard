# 06 — Billing & Payments (Stripe)

Implements FR-BILL-001…011. Library: `stripe-go/v86` (pinned API version
`2026-08-26.dahlia`, ADR-029). `STRIPE_MODE=stub` (default) is a no-network fake;
`STRIPE_MODE=live` is the real adapter (`internal/infrastructure/stripe/live.go`),
used with test-mode keys until the catalog is final (report B02). Local dev:
`stripe-cli` container forwards webhooks (`stripe listen --forward-to
api:8080/api/v1/webhooks/stripe`); §10 has the setup.

## 1. Stripe Object Mapping

| Local | Stripe | Notes |
|---|---|---|
| organization | Customer | created lazily on first paid action; `metadata.org_id` set — webhook consumer resolves org by metadata, never by email |
| subscription row | Subscription | one active per org; local row mirrors id, status, price ids, period end, cancel_at_period_end. The licensed item's quantity = billable seats (non-guest members), set at checkout and re-synced on membership changes and nightly (ADR-030) |
| plans table | Products/Prices | the single plan catalog (ADR-030), served by `GET /api/v1/plans`; seeded by `make stripe-seed` (MODE=live: Billing Meters, product `fluxboard_<plan>`, licensed seat price and metered prices found by lookup key `fluxboard_<plan>_<kind>_v1`); price IDs land in `plans` table, not env vars. The adapter maps a subscription back to its plan by the seat item's price id |
| usage_records | Billing Meter events | Business plan only; `storage_bytes` (meter `fluxboard_storage_bytes`, aggregation `last`) and `api_calls` (`fluxboard_api_calls`, `sum`); active members are billed as licensed seats |
| invoices (mirror) | Invoices | synced on `invoice.finalized` / `invoice.payment_succeeded` |

Card data: **never** touches our servers — Stripe Checkout (hosted) for
acquisition, Stripe Billing Portal for payment-method management. PCI scope:
SAQ A. There is no card form anywhere in our codebase.

## 2. Subscription State Machine (local `subscriptions.status`)

```
 (none/free) ──checkout.session.completed──► active
 active ──invoice.payment_failed──► past_due ──payment ok──► active
 past_due ──Stripe retries exhausted──► unpaid ──► entitlements = Free
 active ──cancel at period end──► active(cancel_at_period_end=true) ──period end──► canceled
 any ──customer.subscription.deleted──► canceled
```

Entitlement resolution (§7) derives from (plan, status): `active|trialing` ⇒
plan entitlements; `past_due` ⇒ plan entitlements + warning banner flag;
`unpaid|canceled|none` ⇒ Free.

## 3. Checkout (FR-BILL-002)

```go
params := &stripe.CheckoutSessionParams{
    Customer:   stripe.String(customerID),
    Mode:       stripe.String("subscription"),
    LineItems:  []*stripe.CheckoutSessionLineItemParams{
        {Price: stripe.String(plan.SeatPriceID), Quantity: stripe.Int64(seatCount)},
        // Business additionally appends metered items (no quantity):
        // storage overage price, api-calls price
    },
    SuccessURL: stripe.String(base + "/billing/checkout/success?session_id={CHECKOUT_SESSION_ID}"),
    CancelURL:  stripe.String(base + "/billing/checkout/cancelled"),
    SubscriptionData: &stripe.CheckoutSessionSubscriptionDataParams{
        Metadata: map[string]string{"org_id": orgID.String()},
    },
}
params.SetIdempotencyKey(fmt.Sprintf("co:%s:%s:%d", orgID, planCode, time.Now().Unix()/300))
```

Success page does NOT trust the redirect: it polls
`GET /orgs/{id}/billing/summary` until the webhook has flipped the state
(spinner "finalizing", timeout 60 s → "taking longer than usual" + support hint).

Plan switch (FR-BILL-003): `POST /billing/preview-change {plan}` calls Stripe
`invoices.create_preview` (the upcoming-invoice API is gone in current API
versions) for the proration figure; confirm calls Subscription Update with
`proration_behavior=create_prorations` under an idempotency key. The seat item
switches price and **keeps its quantity**; metered items are added or removed
to match the target plan.

## 4. Webhook Consumer (the reliability core)

Endpoint `POST /api/v1/webhooks/stripe` — outside session-auth middleware;
gated solely by signature verification:

```go
event, err := webhook.ConstructEventWithOptions(payload, sigHeader, secret,
    webhook.ConstructEventOptions{Tolerance: 5 * time.Minute, IgnoreAPIVersionMismatch: true})
if err != nil { w.WriteHeader(400); return }
```

Only the event's id, type, `created` and `data.object.id` are read from the
payload. The adapter re-reads the object (checkout session → subscription,
subscription, invoice) from the API at the pinned version, so an endpoint or
`stripe listen` running on another API version cannot change the parsed shape,
and the mirror gets the current state. An API failure during that read is a 500
(Stripe retries); a bad signature is a 400. Orgs are resolved from
`metadata.org_id` (subscription, invoice parent, then customer), never by email.

Processing contract:

```
BEGIN;
  INSERT INTO processed_stripe_events (event_id, type, payload, processed_at)
  VALUES ($1,$2,$3,now())
  ON CONFLICT (event_id) DO NOTHING;
  -- rows affected == 0  → duplicate → COMMIT; respond 200 (no side effects)

  -- staleness guard (out-of-order delivery):
  -- if event.created <= subscriptions.last_stripe_event_at → record & skip mutation

  <side effects: mutate subscription mirror / invoices / entitlement cache bust,
   enqueue notification emails — ALL inside this same transaction;
   Asynq enqueue via transactional outbox table drained by worker>
COMMIT;  → respond 200
error   → ROLLBACK → respond 500 (Stripe retries with backoff up to 72h)
```

Properties this guarantees, and the interview language for them:
- **At-least-once delivery × idempotent consumer = exactly-once effect.**
- Unique-violation-as-dedup is race-safe under concurrent duplicate delivery
  (two replicas processing the same event: one commits, one conflicts).
- 200 only after commit ⇒ a crash mid-processing yields a Stripe retry, not a
  lost event.
- Handler is fast (<1 s target); anything slow (emails) goes through the
  outbox → Asynq.

Handled events and side effects:

| Event | Side effect |
|---|---|
| `checkout.session.completed` | upsert subscription mirror (active), set plan, bust entitlement cache, welcome email |
| `customer.subscription.updated` | sync status/price/period/cancel flag; on plan change bust cache |
| `customer.subscription.deleted` | status=canceled, entitlements→Free, cache bust, email OWNER |
| `invoice.payment_succeeded` | upsert invoice mirror(paid); if was past_due → active + "resolved" email |
| `invoice.payment_failed` | status=past_due, dunning email w/ Billing-Portal link, in-app notification OWNER/ADMIN |
| `invoice.finalized` | upsert invoice mirror (number, pdf url, amount, period) |
| anything else | record in processed_stripe_events with `handled=false` (visible in admin browser) |

## 5. Usage Metering Pipeline (FR-BILL-007)

Dimensions: `active_members` (daily HyperLogLog `usage:{org}:am:{yyyymmdd}`,
PFADD on any authenticated request), `storage_bytes` (Redis gauge, adjusted
on attachment confirm/delete, nightly reconciliation against SUM(size)),
`api_calls` (INCR `usage:{org}:api:{yyyymmdd}` in rate-limit middleware).

```
Redis (real-time, lossy-tolerant)
  └─ hourly Asynq job: UPSERT usage_records(org_id, metric, period_date, value)
       ON CONFLICT (org_id, metric, period_date) DO UPDATE  ← idempotent, re-runnable
       └─ daily 02:00 UTC job: for Business orgs, one Billing Meter event per
            customer/metric/day {stripe_customer_id, value}, identifier
            "<customer>:<metric>:<day>" -- Stripe drops a repeated identifier,
            and pushed days are marked, so re-running the job is harmless
```

The legacy usage-records API this section first described was removed by
Stripe (`2025-03-31.basil`); meters replaced it (ADR-029). Usage prices are $0
until the catalog sets real amounts (report B02), so usage is recorded and
shown on invoices without being charged yet.

Consistency model to state honestly (docs + interview): metering is
**eventually consistent by design**; a Redis loss between flushes costs at
most one hour of API-call counts; storage is self-healing via nightly
reconciliation; billing-side truth is what was pushed with `set`.

## 6. Dunning (FR-BILL-006)

Stripe Smart Retries enabled (dashboard config, documented in README).
Timeline: fail#1 → past_due (email + banner) → Stripe retries (3, 5, 7 days)
→ each `invoice.payment_failed` refreshes the notice → final failure →
`customer.subscription.updated(status=unpaid)` → entitlements drop to Free
(data retained, over-limit resources read-only). Payment fix path: Billing
Portal session (`POST /billing/portal` → redirect URL).

## 7. Entitlement Middleware (FR-BILL-009)

```go
type Entitlements struct {
    MaxMembers, MaxProjects int  // -1 unlimited
    MaxAgents               int  // agents are not seats (ADR-033)
    MaxStorageBytes         int64
    APIRatePerMin           int
    AuditRetentionDays      int
    Metered                 bool
}
```

Resolution: Redis `ent:{orgID}` (TTL 60 s) → miss: derive from subscription
mirror → apply platform-admin `entitlement_overrides` (FR-ADM-002; integer
value, -1 = unlimited) → cache. An override change takes effect within the
60 s TTL. Webhook consumer and plan-change usecase DEL the key
(write-through invalidation), so the 60 s TTL is only a safety net.
Write endpoints declare checks:

```go
r.With(mw.RequireEntitlement(mw.CheckProjectQuota)).Post("/projects", h.Create)
// exceeded → 402 {"error":{"code":"plan_limit_exceeded","limit":"max_projects",
//                  "current":20,"max":20}}
```

Frontend maps 402 + `limit` to the contextual upgrade modal (02 §9).

## 8. Reconciliation Job (drift defense, ADR-010)

Nightly: list Stripe subscriptions (paginated) for all known customers,
diff against local mirror on (status, price, period_end). Drift → log at
ERROR, Prometheus counter `billing_reconciliation_drift_total`, auto-heal
local mirror toward Stripe (Stripe is the source of truth for subscription
state), surface in admin panel. This catches missed webhooks (endpoint down
> 72 h) and manual dashboard changes.

## 9. Testing Hooks (full plan 12 §7)

- `stripe trigger invoice.payment_failed` etc. via CLI in integration tests
- Replay test: deliver the same event payload 5× concurrently → exactly one
  side-effect row set
- Out-of-order test: deliver `subscription.updated(created=T2)` then a stale
  `(created=T1)` → mirror stays at T2 state
- Clock-skew signature test: tampered payload / stale timestamp → 400

## 10. Stripe test-mode setup and checks (ADR-029)

1. In the Stripe Dashboard (test mode) copy the secret key, and save the
   Customer Portal settings once (Settings → Billing → Customer portal); the
   portal session call fails until a configuration exists.
2. In `.env`: `STRIPE_MODE=live`, `STRIPE_SECRET_KEY=sk_test_…`.
3. `make stripe-seed`: creates the meters, products and prices and writes their
   ids into `plans` (refuses live-mode keys until B02).
4. Webhooks: `docker compose -f deploy/docker-compose.yml --profile stripe up -d stripe-cli`,
   take the `whsec_…` it prints (`docker compose … logs stripe-cli`) into
   `STRIPE_WEBHOOK_SECRET`, restart api and worker.
5. `make stripe-sandbox-test`: runs the adapter against the test account
   (catalog, customer, Checkout session, preview, plan change keeping seats,
   meter events, cancel/resume, portal, reconcile fetch, cancel). A browser
   Checkout with card `4242 4242 4242 4242` then exercises the webhook path:
   the billing page flips to the new plan once `checkout.session.completed` lands.

Offline coverage without an account: `live_test.go` (fake Stripe server:
request shapes, idempotency keys, signature tolerance, org routing) and a run of
every adapter call against `stripe/stripe-mock` (schema validation) during
development.
