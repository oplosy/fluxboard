# 12 — Testing Strategy

## 1. Test Pyramid & Targets

This document is the **target** strategy. The Status column records what
exists today (checked 2026-10-12); "planned" rows are not implemented and must
not be cited as coverage.

| Layer | Tooling | Scope | Coverage target | Status |
|---|---|---|---|---|
| Unit (Go) | stdlib `testing` (no assertion library) | domain logic, rank math, token gen, entitlement derivation, JWT/Argon2 params | usecase ≥ 85%, billing pkg ≥ 95% | In place (`go test ./...`); coverage targets not measured in CI |
| Unit (web) | Vitest + Testing Library | API client, components | — | In place (`pnpm test`, 3 files) |
| Integration (Go) | `//go:build integration` suites against real services: CI service containers (Postgres 16, Redis 7, RustFS S3 — ADR-027), the compose stack locally | repositories against real PG (RLS active!), object store, webhook consumer, jobs | all repos + consumer | Partial: webhook repo idempotency (`postgres/`) and object-store adapter (`minio/`) only |
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

**Status: planned.** `make seed` is currently a stub that only prints a
message; the fixture below is the target. Until it exists, a fresh database has
no `plans` rows, so entitlement-gated writes (e.g. project create) fail with
`load free plan: not found`.

Deterministic fixture (fixed UUIDs, fixed clock):
- Users: `owner@demo.dev`, `admin@…`, `member@…`, `guest@…`, `outsider@…` (all password `Demo-Pass-2026!`), `padmin@…` (platform admin, TOTP seeded)
- Orgs: `acme` (Business, active, 20 members, 6 projects, 400 tasks, usage history 30d), `beta-co` (Free, 2 members), `pastdue-inc` (Pro, past_due)
- One project per state: active/archived/private; tasks distributed across columns with realistic timestamps (feeds analytics rollup verification)

## 6. Tenant Isolation Suite (Phase-2 gate — must be exhaustive, generated)

**Status: planned.** No generated suite exists yet. Isolation is currently
backed by RLS plus per-usecase tests (e.g. the AI private-project tests in
`aiuc`); the matrix below is the target.

Test generator iterates **every tenant-scoped endpoint** (route table
introspection) × principal in {member-of-A, admin-of-A, outsider} attempting
org-B resources: expected 404/403 per matrix. Plus:
- Raw repository call with GUC unset → 0 rows (fail-closed proof)
- INSERT with mismatched org_id under RLS `WITH CHECK` → error
- Search/FTS endpoint never returns foreign-org hits (seeded marker strings)
- Presigned download for foreign-org attachment id → 404 before presign

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

Still target: add the tenant-isolation and billing suites as required checks
once they exist; Playwright smoke (journeys 1–3) on PR, full set nightly. Flake
policy: quarantine label + issue, never retry-until-green as a merge strategy.
