# 08 — REST API Specification

Base: `/api/v1`. JSON only. Auth: `Authorization: Bearer <access JWT>` or
`Bearer fbk_live_…` (API key, org-bound, `read`/`write` scopes). OpenAPI 3.1
generated from handler annotations (swaggo) → served at `/api/v1/openapi.json`;
frontend types generated from it (`make gen-client`).

## 1. Cross-Cutting Contracts

**Error envelope (single shape everywhere):**

```json
{"error":{"code":"plan_limit_exceeded","message":"Project limit reached",
          "details":{"limit":"max_projects","current":20,"max":20},
          "request_id":"018f..."}}
```

Codes: `validation_failed` (422, `details.fields{}`), `unauthorized` (401),
`forbidden` (403), `not_found` (404 — also used for cross-tenant probes),
`conflict` (409), `plan_limit_exceeded` (402), `rate_limited` (429 +
Retry-After), `internal` (500).

**Pagination:** cursor-based. Request `?limit=50&cursor=<opaque>`;
response `{"items":[...],"next_cursor":"..."} `. Cursor = base64(created_at,id)
keyset — no OFFSET (stable under concurrent inserts, no O(n) skip).

**Concurrency control:** mutable resources return `updated_at`; PATCH accepts
`If-Unmodified-Since`-style body field `expected_updated_at` → mismatch 409
`stale_write` (used by task detail form; board moves use rank CAS instead).

**Idempotency:** clients MAY send `Idempotency-Key` header on POST; server
stores response for 24 h (Redis) keyed by (key, route, org) and replays it.
Required by frontend for checkout and invite endpoints.

## 2. Auth — `/auth/*` (listed in 04 §5; no org context)

## 3. User Scope

```
GET    /me                                 profile + org memberships summary
PATCH  /me                                 name
POST   /me/avatar/upload-url               presigned PUT (mime/size validated)
POST   /me/avatar/confirm
GET    /me/notifications?org_id=&unread=   cross-checked against membership
POST   /me/notifications/{id}/read         POST /me/notifications/read-all
GET/PUT /me/notification-prefs
DELETE /me                                 409 sole_owner_of: [orgs] if blocked
GET    /users/lookup?email=                exact-match, only for invite UX, rate-limited
```

## 4. Organizations

```
POST   /orgs                               {name, slug}
GET    /orgs                               my orgs
GET    /orgs/{orgId}                       (membership required; includes logo_url when a logo is stored)
PATCH  /orgs/{orgId}                       name/logo (ADMIN+), slug (OWNER)
POST   /orgs/{orgId}/logo/upload-url       presigned PUT for the org logo (ADMIN+; confirm via PATCH logo_key, namespaced org-logos/{orgId}/)
DELETE /orgs/{orgId}                       soft-delete flow (OWNER)
POST   /orgs/{orgId}/restore               within grace (OWNER)
POST   /orgs/{orgId}/transfer-ownership    {user_id} (OWNER)

GET    /orgs/{orgId}/members               ?role=&q=  paginated
PATCH  /orgs/{orgId}/members/{userId}      {role}
DELETE /orgs/{orgId}/members/{userId}
DELETE /orgs/{orgId}/members/me            leave

GET    /orgs/{orgId}/invitations           pending
POST   /orgs/{orgId}/invitations           {email, role}  [Idempotency-Key]
DELETE /orgs/{orgId}/invitations/{id}      revoke
POST   /orgs/{orgId}/invitations/{id}/resend
POST   /invitations/accept                 {token}   (user scope, no org prefix)

GET/POST/PATCH/DELETE /orgs/{orgId}/labels[/{id}]
GET    /orgs/{orgId}/audit-log             ?actor=&action=&from=&to=  (ADMIN+)
GET    /orgs/{orgId}/audit-log/export      CSV ≤10k rows
GET    /orgs/{orgId}/tasks/search      ?q=&project_id=&assignee_id=&column_id=&priority=&label_id=
                                    FTS over tasks (FR-TASK-007); q may be empty
                                    when a filter is set (newest first), else 422
GET    /orgs/{orgId}/events                SSE (09 §1)
```

## 5. Projects & Board

```
GET    /orgs/{orgId}/projects              ?archived=
POST   /orgs/{orgId}/projects              {name,key,visibility,color,template?}
GET    /orgs/{orgId}/projects/{key}
PATCH  /orgs/{orgId}/projects/{key}        (LEAD)
POST   /orgs/{orgId}/projects/{key}/archive | /unarchive
DELETE /orgs/{orgId}/projects/{key}        soft (LEAD; ADMIN+ for hard)

GET/POST /orgs/{orgId}/projects/{key}/members
PATCH/DELETE /orgs/{orgId}/projects/{key}/members/{userId}

GET    /orgs/{orgId}/projects/{key}/columns
POST   /orgs/{orgId}/projects/{key}/columns          {name, position, wip_limit?}
PATCH  /orgs/{orgId}/projects/{key}/columns/{id}
DELETE /orgs/{orgId}/projects/{key}/columns/{id}     ?migrate_to=<columnId> required if non-empty

GET    /orgs/{orgId}/projects/{key}/board            columns + tasks (card projection), one query
GET    /orgs/{orgId}/projects/{key}/analytics        rollup data (FR-AN-001)
```

## 6. Tasks

```
GET    /orgs/{orgId}/projects/{key}/tasks            ?column=&assignee=&label=&priority=&q=&cursor=
POST   /orgs/{orgId}/projects/{key}/tasks            create (FR-TASK-001)
GET    /orgs/{orgId}/tasks/{taskId}                  full detail (subtasks, labels, activity head)
PATCH  /orgs/{orgId}/tasks/{taskId}                  field edits (+expected_updated_at)
PATCH  /orgs/{orgId}/tasks/{taskId}/position         {column_id, rank}  → 409 rank_conflict
POST   /orgs/{orgId}/tasks/bulk                      {task_ids[], op: move|assign|label, ...}
DELETE /orgs/{orgId}/tasks/{taskId}                  → trash
POST   /orgs/{orgId}/tasks/{taskId}/restore
GET    /orgs/{orgId}/trash                            POST /orgs/{orgId}/trash/{id}/purge

GET/POST /orgs/{orgId}/tasks/{taskId}/subtasks       PATCH/DELETE .../subtasks/{id}
GET/POST /orgs/{orgId}/tasks/{taskId}/comments       PATCH/DELETE .../comments/{id}
GET      /orgs/{orgId}/tasks/{taskId}/activity       paginated

POST   /orgs/{orgId}/tasks/{taskId}/relations        {blocked_id} → 422 cycle_detected + path
DELETE /orgs/{orgId}/tasks/{taskId}/relations/{blockedId}

POST   /orgs/{orgId}/tasks/{taskId}/attachments/upload-url   {filename,mime,size} → presigned PUT
POST   /orgs/{orgId}/tasks/{taskId}/attachments/{id}/confirm
GET    /orgs/{orgId}/attachments/{id}/download-url            presigned GET (5 min)
DELETE /orgs/{orgId}/attachments/{id}
```

## 7. Billing

```
GET    /orgs/{orgId}/billing/summary        plan, status, seats, period_end, pm brand/last4
POST   /orgs/{orgId}/billing/checkout       {plan} → {url}   [Idempotency-Key]
POST   /orgs/{orgId}/billing/preview-change {plan} → proration figures
POST   /orgs/{orgId}/billing/change-plan    {plan}
POST   /orgs/{orgId}/billing/cancel         at period end
POST   /orgs/{orgId}/billing/resume
POST   /orgs/{orgId}/billing/portal         → {url}  (Stripe Billing Portal session)
GET    /orgs/{orgId}/billing/invoices       mirror list
GET    /orgs/{orgId}/billing/usage          ?from=&to=  usage_records series
```

## 8. API Keys

```
GET    /orgs/{orgId}/api-keys
POST   /orgs/{orgId}/api-keys              {name, scopes[]} → plaintext ONCE
DELETE /orgs/{orgId}/api-keys/{id}
```

API-key requests: org inferred from key (no `{orgId}` mismatch allowed — key
bound to a different org than the path → 403), scope `write` required for all
non-GET, rate limit headers per FR-API-002. Session-auth requests share the
same limiter keyed `rl:api:{orgId}:{userId}`.

## 8a. Agents (ADR-033, FR-AGT-002/003)

```
GET    /orgs/{orgId}/agents                         → {agents[], limit:{current,max}}   (MEMBER+)
POST   /orgs/{orgId}/agents                         {name, role?} → {agent, key_id, secret ONCE}   (ADMIN+)
PATCH  /orgs/{orgId}/agents/{agentId}               {name?, role?, status?: active|paused}
DELETE /orgs/{orgId}/agents/{agentId}               204 (soft delete; revokes keys)
POST   /orgs/{orgId}/agents/{agentId}/keys          → {agent, key_id, secret ONCE}
DELETE /orgs/{orgId}/agents/{agentId}/keys/{keyId}  204
```

- `role` is `MEMBER` (default) or `GUEST`; anything else → 422
  `agent_role_invalid`. Names: 1–40 letters, digits, space, `.`, `-`, `_`
  (422 `agent_name_invalid`), unique per org case-insensitively (409
  `agent_name_taken`). At most 3 active keys (409 `agent_key_limit`).
- Create over the plan's `max_agents` → 402 `plan_limit_exceeded`
  (`limit=max_agents`); paused agents count.
- An agent key authenticates as the agent (its user id, its role) on org
  routes and `/mcp`; a paused agent → 403 `agent_paused`. API keys are refused
  (403) on user-scoped routes: `/me*`, `GET|POST /orgs`, `/orgs/by-slug`,
  `/invitations/accept`.
- Agent keys are not listed by `GET /api-keys`.

## 8b. Governed AI — `/orgs/{orgId}/ai/*` (ADR-025, FR-AI-001..009)

```
POST   /orgs/{orgId}/ai/parse             {input} → items[]
POST   /orgs/{orgId}/ai/plan              {brief}  [Idempotency-Key] → draft
POST   /orgs/{orgId}/ai/plan/apply        {project_id, column_id, items[]}  [Idempotency-Key] → tasks (201, replay 200)
POST   /orgs/{orgId}/ai/chat              {message, history[]} → turn
GET    /orgs/{orgId}/projects/{projectId}/ai/digest   live-data report
GET    /orgs/{orgId}/ai/risks?project_id= open risks
POST   /orgs/{orgId}/ai/risks/scan        {project_id} (BUSINESS+)
POST   /orgs/{orgId}/ai/risks/dismiss     {task_id} (204)
```

Every AI call: surface flag → monthly meter (402 `ai_actions`) → provider →
`ai_runs` ledger → `ai.run` audit.

## 8c. MCP server — `POST /mcp` (ADR-034, FR-AI-007, FR-AGT-001)

```
POST   /mcp        JSON-RPC 2.0, Streamable HTTP, application/json responses
GET|DELETE /mcp    405 (no standalone stream, no sessions)
```

- **Auth:** `Authorization: Bearer fbk_live_…` only; the key's org is the
  tenant (no org id in the URL). Session tokens → 401. Plan rate limit and
  usage metering apply as on org routes. A browser `Origin` other than
  `WEB_ORIGIN` → 403.
- **Modern (2026-07-28):** `_meta.io.modelcontextprotocol/protocolVersion` +
  `clientCapabilities` on every request; `MCP-Protocol-Version`, `Mcp-Method`
  and (tools/call) `Mcp-Name` headers must match the body (400 `-32020`).
  Unsupported version → 400 `-32022` `{supported, requested}`; unknown method
  → 404 `-32601`. Methods: `server/discover`, `tools/list`, `tools/call`, `ping`.
- **Legacy (2025-11-25 / 2025-06-18 / 2025-03-26):** `initialize` negotiates;
  later requests send `MCP-Protocol-Version` (absent ⇒ 2025-03-26);
  notifications → 202. Methods: `initialize`, `ping`, `tools/list`, `tools/call`.
- **Tools:** read — `projects.list`, `projects.board`, `tasks.search`,
  `tasks.get`, `ai.parse`, `ai.plan_draft`, `ai.digest`, `ai.chat`,
  `ai.risks_scan`, `ai.risks_list`; write (needs a write-scoped key; hidden
  from read keys) — `ai.risks_dismiss`; agent-only (an agent's key, hidden
  from org keys) — `ai.plan_apply`. Business failures come back as `isError: true` tool
  results; unknown tools / malformed arguments as `-32602`; internal errors as
  `-32603` with no detail.
- **Client setup:** `claude mcp add --transport http fluxboard https://<host>/api/v1/mcp --header "Authorization: Bearer fbk_live_…"`.

## 9. Platform Admin — `/admin/*` (separate router; platform_role=admin + TOTP enforced)

```
GET  /admin/stats                          GET  /admin/tenants?q=&plan=&status=
GET  /admin/tenants/{orgId}                PATCH /admin/tenants/{orgId}/entitlement-overrides
POST /admin/tenants/{orgId}/impersonate    → short-lived read-only token (FR-ADM-003)
GET  /admin/users?q=                       POST /admin/users/{id}/lock | /unlock
GET  /admin/webhook-events?status=&type=   POST /admin/webhook-events/{eventId}/retry
GET  /admin/audit-log                      GET/PUT /admin/flags/{orgId}
GET  /admin/jobs/stats                     (Asynq queue introspection)
```

## 10. Webhooks & System

```
POST /api/v1/webhooks/stripe    signature-gated (06 §4); NOT under /admin or session auth
GET  /healthz                   200 static
GET  /readyz                    checks: PG ping, Redis ping, migrations current → 200/503
GET  /metrics                   Prometheus (bound to internal interface)
```
