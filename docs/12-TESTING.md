# 12 — Testing Strategy

## 1. Test Pyramid & Targets

This document is the **target** strategy. The Status column records what
exists today (checked 2026-10-12); "planned" rows are not implemented and must
not be cited as coverage.

| Layer | Tooling | Scope | Coverage target | Status |
|---|---|---|---|---|
| Unit (Go) | stdlib `testing` (no assertion library) | domain logic, rank math, token gen, entitlement derivation, JWT/Argon2 params | usecase ≥ 85%, billing pkg ≥ 95% | In place (`go test ./...`); coverage targets not measured in CI |
| Unit (web) | Vitest + Testing Library | API client, components | — | In place (`pnpm test`, 3 files) |
| Integration (Go) | `//go:build integration` suites against real services: CI service containers (Postgres 16, Redis 7, RustFS S3 — ADR-027), the compose stack locally | repositories against real PG (RLS active!), object store, webhook consumer, jobs | all repos + consumer | Partial: webhook repo idempotency (`postgres/`), object-store adapter (`minio/`), tenant-isolation suite (`cmd/api`, §6) |
| API contract | Go httptest against wired router | every endpoint: happy + authz + validation + error envelope | every route ≥ 1 authz-negative test | Partial: handler tests exist for some routes; no per-route authz-negative gate |
| E2E | Playwright | 6 golden journeys (below) | smoke in CI, full nightly | Planned — not in the repo; manual PowerShell smokes in `scripts/smoke/` |
| Load/abuse | k6 | board under concurrency, rate-limit behavior, SSE fan-out | profiles recorded in README | Planned — not in the repo |

Integration tests run against **RLS-enabled** schema with the `fluxboard_app`
role — testing with a superuser would silently bypass the exact layer under test.
The Postgres suite enforces this: it fails if the DSN role is a superuser or
`BYPASSRLS`. Every integration suite **fails rather than skips** when its
connection setting (`TEST_DATABASE_URL`, `TEST_S3_ENDPOINT`) is missing, so a
green run means the suite actually ran. testcontainers-go was the original plan;
CI uses service containers instead, and suites take their endpoints from env.

## 2. Golden E2E Journeys (Playwright)

1. Register → verify (Mailpit API fetch) → create org → invite member →
   second browser accepts → both see each other
2. Create project → create 5 tasks → drag between columns → second browser
   sees moves live (SSE) → comment with @mention → notification appears
3. Upgrade to Pro via Stripe Checkout (test card 4242…) → success page
   resolves after webhook → project limit raised
4. Payment failure path: `stripe trigger invoice.payment_failed` → past_due
   banner → portal link present
5. OAuth login (mocked Google via local OIDC stub container) → identity linked
6. Admin: impersonate tenant read-only → write attempt rejected → audit
   entries carry both identities

## 3. Unit-Test Focal Points

- LexoRank: between/append/prepend, exhaustion → rebalance trigger, property
  test (10k random moves keep total order)
- Entitlements: (plan × status) matrix → expected Entitlements struct (table test)
- Refresh rotation state machine: valid→rotate, rotated→family-revoke,
  expired→revoke, concurrent (2 goroutines, real PG row lock in integration tier)
- Cycle detection for task relations (path returned)
- Argon2id/JWT parameter regression guards (assert configured values)

## 4. Auth Security Suite (maps to 04 §6)

- Timing: unknown-email vs wrong-password login deltas statistically
  indistinguishable (n=200, Welch t-test tolerance)
- Response-body equality on forgot-password (known vs unknown email)
- Token single-use: parallel double-consume of reset token → exactly one succeeds
- Session revocation latency ≤ cache TTL (revoke → API call 401 within 60 s, in tests: immediate via cache-bust assertion)

## 5. Seed Data (`make seed`)

**Status: in place** (`backend/cmd/seed`). `make seed` first runs
`make stripe-seed` (upserts the free/pro/business `plans` rows; without them
entitlement-gated writes fail with `load free plan: not found`), then loads the
fixture on the owner pool. It refuses `APP_ENV=prod`.

Deterministic fixture (fixed UUIDv5 IDs; every row derived from a clock that
defaults to today, UTC midnight — pin it with `make seed SEED_ARGS="-clock
2026-01-15"` for byte-identical data):
- Users (all `email_verified`, password `Demo-Pass-2026!`): `owner@demo.dev`,
  `admin@…`, `member@…`, `guest@…`, `outsider@…` (in no org — §6's outsider),
  `user01…user16@…`, `beta.owner@…`, `beta.member@…`, and `padmin@…`
  (platform admin). When `TOTP_ENC_KEY` is set,
  padmin has 2FA enabled with the fixed dev secret `JBSWY3DPEHPK3PXP` (the admin
  console requires 2FA); without the key padmin is seeded without 2FA.
- Orgs: `acme` (Business/active, 20 members, 6 projects, 400 tasks, 30 days of
  usage history), `beta-co` (Free — no subscription row — `beta.owner@` and
  `beta.member@`, none of them in acme), `pastdue-inc` (Pro/past_due, owned by `owner@`, so the org
  switcher has two orgs).
- acme projects: `WEB`, `API`, `MOB`, `OPS` (active, org-visible), `OLD`
  (archived), `SEC` (private: `user01` LEAD, `user02` CONTRIBUTOR; `member@`
  is deliberately not a member). `guest@` is VIEWER on `WEB` only.
- Tasks: created over the last 60 days, older ones mostly Done; ~80% assigned to
  project contributors, ~60% with a due date (so some open tasks are overdue),
  priorities and labels spread across the range.
- Isolation markers: task descriptions contain `secaudit-private-marker`
  (acme `SEC`), `betaco-marker` (beta-co) and `pastdue-marker` (pastdue-inc) for
  the search/visibility checks of §6.

Re-running resets the seed orgs: they are deleted (all their tenant rows
cascade) and rebuilt; seed users are upserted by fixed ID. A user or org that
only shares a seed email or slug under a different ID is never touched — the
command fails and names it instead.

## 6. Tenant Isolation Suite (Phase-2 gate — must be exhaustive, generated)

**Status: in place** — `backend/cmd/api/isolation_integration_test.go`
(`integration` tag; part of the required `test-backend` CI job). It builds the
real API wiring (`newApp`) in-process against Postgres (app role, RLS on),
Redis and RustFS, loads the §5 fixture (org A = `acme`, org B = `beta-co`) plus
one org-B row of every resource a route can name, and logs in through the real
login endpoint.

Test generator iterates **every tenant-scoped endpoint** (route table
introspection via `chi.Walk`, currently 121 routes) × principal in
{member-of-A, admin-of-A, outsider} attempting org-B resources:
- **Via org B's path** (`/orgs/{B}/…`): every response must be 403/404 — the
  tenant guard stops non-members before any handler runs.
- **Via org A's path with org B's resource IDs** (76 routes carry resource
  params): no 2xx/3xx, no 5xx, no 401 (an unauthenticated probe proves
  nothing), and no org-B marker in any body.
- **IDs in bodies and queries** (bulk actions, AI risk scan/dismiss/list,
  search `project_id`) get the same treatment.
- **Org B's rows are digested** in every table with an `org_id` before and after
  the probes and must be identical.
- A route parameter with no org-B resource **fails the suite** — adding a route
  with a new parameter means adding that resource to `insertOrgBResources`.

Plus:
- Raw query as the app role with no `app.current_tenant` → no rows from any RLS
  table (fail-closed proof: 0 rows on a fresh connection; on a pooled one the
  leftover `''` makes the policy's `::uuid` cast error, also returning nothing).
  Tenant A sees none of org B's rows; tenant B sees all of its own.
- INSERT with mismatched org_id under RLS `WITH CHECK` → error (42501)
- Search/FTS endpoint never returns foreign-org or hidden-private-project hits
  (seeded marker strings), with positive controls proving the markers match
- Presigned download for foreign-org attachment id → 404 before presign (the
  rightful owner gets a `download_url`, proving the store is wired)

Positive controls (A's member reads A, B's owner reads B's seeded IDs) run
first, so a broken login or fixture cannot make the negative checks pass
vacuously. The first run found `GET /projects/{id}/analytics` answering 200 for
a foreign project (and exposing private-project stats inside a tenant); fixed in
`analyticsuc`.

## 7. Billing Reliability Suite (Phase-4 gate)

- Same `checkout.session.completed` payload delivered 5× concurrently
  (goroutines against consumer) → exactly one subscription flip, one welcome
  email outbox row
- Out-of-order: `subscription.updated(created=T2)` then stale `T1` → mirror
  remains T2
- Signature: tampered body / stale timestamp / wrong secret → 400, nothing recorded
- Consumer crash simulation: side-effect tx forced to fail → 500 → redelivery
  → succeeds → single effect
- Proration preview vs applied invoice consistency (Stripe test clock advance)
- Metering: rerun `usage:push_stripe` for same day → Stripe quantity unchanged
- Reconciliation: mutate mirror manually → job heals + drift counter increments

## 8. Jobs & Realtime Suite (maps 09 §4)

Worker kill/rerun idempotency per task type; outbox backlog drain without
email duplicates (Mailpit count assertion); SSE `Last-Event-ID` replay
correctness; `resync` on over-aged cursor; 500-connection fan-out k6 profile.

## 9. CI Wiring

Today (`.github/workflows/ci.yml`): jobs `lint`, `test-backend`, `build`, `web`,
`security` on every PR to `main`. `test-backend` runs `go test ./...`, then
bootstraps the roles and migrations and runs `go test -tags=integration ./...`
against the service containers. `make test` = unit; `make test-integration` =
the same integration suites against the local compose stack (`make up` +
`make migrate` first).

**Branch protection on `main`** (since 2026-10-12): all five jobs are required
status checks; changes land only through a PR (no approval count, so a solo
maintainer can merge their own PR); the rules apply to admins too; force pushes
and branch deletion are blocked. Branches do not have to be up to date with
`main` before merging (AGENTS.md still asks for a rebase before opening the PR).

The tenant-isolation suite runs inside the required `test-backend` job. Still
target: the billing suite (§7) as a required check once it exists; Playwright
smoke (journeys 1–3) on PR, full set nightly. Flake
policy: quarantine label + issue, never retry-until-green as a merge strategy.
