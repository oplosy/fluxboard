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
POST   /orgs/{orgId}/projects              {name,key,visibility,color}
GET    /orgs/{orgId}/project-templates     template catalog (FR-PROJ-007)
POST   /orgs/{orgId}/projects/from-template {template,name,key,visibility,color}  one tx, Idempotency-Key (ADR-035)
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
GET    /orgs/{orgId}/dashboard                       per-project open/done/overdue, per-assignee open, weekly completed; visible projects only (FR-AN-005)
GET    /orgs/{orgId}/dashboard/layout                caller's widget list [{id,type,project_ids}]; default when unsaved
PUT    /orgs/{orgId}/dashboard/layout                replace it; types project_status|overdue|throughput|workload, <=12 widgets, 400 otherwise
```

## 6. Tasks

```
GET    /orgs/{orgId}/projects/{key}/tasks            ?column=&assignee=&label=&priority=&q=&cursor=
POST   /orgs/{orgId}/projects/{key}/tasks            create (FR-TASK-001)
GET    /orgs/{orgId}/tasks/{taskId}                  full detail (subtasks, labels, activity head)
PATCH  /orgs/{orgId}/tasks/{taskId}                  field edits (+version) → 409 version_conflict; estimate: 0–1000 points, null clears, absent keeps (FR-TASK-015)
PATCH  /orgs/{orgId}/tasks/{taskId}/position         {column_id, rank, version?}  → 409 rank_conflict | version_conflict
                                                     → 403 human_approval_required (agent → done column)
POST   /orgs/{orgId}/tasks/{taskId}/review           {decision: approve|changes, comment?} (people only; comment required for changes)
PUT    /orgs/{orgId}/projects/{projectId}/columns/{columnId}/role  {role: backlog|todo|in_progress|review|done|""} (LEAD)
GET    /orgs/{orgId}/projects/{projectId}/epics                epics with progress {total, done, points, done_points} (VIEWER; FR-PROJ-008, FR-TASK-015)
POST   /orgs/{orgId}/projects/{projectId}/epics                {name, color?: #rrggbb} (LEAD)
PATCH  /orgs/{orgId}/projects/{projectId}/epics/{epicId}       {name, color?} (LEAD)    DELETE same path keeps its tasks (LEAD)
POST   /orgs/{orgId}/projects/{projectId}/epics/assign         {task_ids, epic_id|null} (CONTRIBUTOR)
GET    /orgs/{orgId}/projects/{projectId}/workload?from=YYYY-MM-DD&to=YYYY-MM-DD  {from, to, rows:[{user_id|null, open_tasks, open_points, scheduled_tasks, scheduled_points, scheduled_unestimated, overdue_tasks, logged_minutes}]} UTC days, to inclusive, ≤ 92 days (VIEWER; FR-AN-004)
GET    /orgs/{orgId}/integrations/github                      {configured, webhook_path, repos: ["owner/name"], move_on_merge_project_ids} (ADMIN; the secret is never returned; FR-TASK-016)
POST   /orgs/{orgId}/integrations/github/secret               creates or rotates the webhook secret, returns {secret} ONCE (64 hex); 403 when TOTP_ENC_KEY is unset (ADMIN)
PUT    /orgs/{orgId}/integrations/github                      {repos: ["owner/name"] (max 50, empty = any), move_on_merge_project_ids}; 404 before a secret exists, 422 for a project outside the org (ADMIN)    DELETE same path removes the integration (ADMIN)
GET    /orgs/{orgId}/tasks/{taskId}/pull-requests             {pull_requests:[{repo, number, title, state: open|closed|merged, url, updated_at}]} (VIEWER; FR-TASK-016)
GET    /orgs/{orgId}/projects/{projectId}/releases             releases with {release_date: YYYY-MM-DD|null, released_at|null, total, done, points, done_points} (VIEWER; FR-PROJ-010)
POST   /orgs/{orgId}/projects/{projectId}/releases             {name, description?, release_date?: YYYY-MM-DD} (LEAD)
PATCH  /orgs/{orgId}/projects/{projectId}/releases/{releaseId} {name, description, release_date|null, released: bool} released=true stamps released_at once, false reopens (LEAD)    DELETE same path keeps its tasks (LEAD)
POST   /orgs/{orgId}/projects/{projectId}/releases/assign      {task_ids, release_id|null} (CONTRIBUTOR)
GET    /orgs/{orgId}/projects/{projectId}/wiki?q=               pages {id, title, version, created_by, updated_by, created_at, updated_at} by title, no bodies; q = title substring (VIEWER; FR-PROJ-013)
POST   /orgs/{orgId}/projects/{projectId}/wiki               {title, body?} -> page with body, version 1 and tasks (CONTRIBUTOR); body <= 50000 chars, title <= 200
GET    /orgs/{orgId}/projects/{projectId}/wiki/{wikiId}      page with body and tasks: [{number, title}] = live tasks of this project named as KEY-N in the body (VIEWER)
PATCH  /orgs/{orgId}/projects/{projectId}/wiki/{wikiId}      {title, body, version} 409 when version is not the stored one (CONTRIBUTOR)    DELETE same path (project LEAD or author)
GET    /orgs/{orgId}/projects/{projectId}/wiki/mentions?task_number=N  pages naming task N, no bodies (VIEWER)
GET    /orgs/{orgId}/goals                                     {goals:[{id, title, description, owner_id, target_date: YYYY-MM-DD|null, status: on_track|at_risk|done, total, done, points, done_points, created_at}]}; progress counts distinct tasks linked directly or through a linked epic, only from projects the caller can see (any org member; FR-PROJ-014)
POST   /orgs/{orgId}/goals                                     {title, description?, target_date?: YYYY-MM-DD} caller becomes owner (MEMBER+); returns the detail below
GET    /orgs/{orgId}/goals/{goalId}                            goal + items:[{kind: epic|task, id, project_id, project_key, number, title, total, done}] (linked work of projects the caller cannot see is omitted)
PATCH  /orgs/{orgId}/goals/{goalId}                            {title, description, target_date|null} (owner or ADMIN+)    DELETE same path keeps the linked work (owner or ADMIN+)
PUT    /orgs/{orgId}/goals/{goalId}/epics/{epicId}             link an epic, idempotent (owner or ADMIN+; epic's project must be visible, else 404)    DELETE same path unlinks (404 if not linked)
PUT    /orgs/{orgId}/goals/{goalId}/tasks/{taskId}             link a task, same rules    DELETE same path unlinks
GET    /orgs/{orgId}/projects/{projectId}/field-values         every custom value of the project's live tasks: {values:[{task_id, field_id, name, type, text?, number?, date?}]} (VIEWER; FR-PROJ-012)
GET    /orgs/{orgId}/projects/{projectId}/views                shared views + the caller's personal ones: {id, name, shared, mine, filters:{text, priority, assignee, sprint, epic, group_by}} (VIEWER; FR-PROJ-011)
POST   /orgs/{orgId}/projects/{projectId}/views                {name, shared, filters} personal: VIEWER, shared: LEAD
PATCH  /orgs/{orgId}/projects/{projectId}/views/{viewId}       {name, shared, filters} own personal view or (LEAD) a shared one; someone else's personal view → 404    DELETE same path, same rules
GET    /orgs/{orgId}/projects/{projectId}/sprints              sprints oldest-first; completed ones carry {completed_total, completed_done, completed_points_total, completed_points_done} (VIEWER; FR-AN-003)
GET    /orgs/{orgId}/projects/{projectId}/sprints/{sprintId}/burndown  {start, end|null, total_points, total_tasks, days:[{date, remaining_points, remaining_tasks}]} UTC days; planned sprint → 409 (VIEWER; FR-AN-003)
POST   /orgs/{orgId}/tasks/bulk                      {task_ids[], op: move|assign|label, ...}
DELETE /orgs/{orgId}/tasks/{taskId}                  → trash
POST   /orgs/{orgId}/tasks/{taskId}/restore
GET    /orgs/{orgId}/trash                            POST /orgs/{orgId}/trash/{id}/purge

GET/POST /orgs/{orgId}/tasks/{taskId}/subtasks       PATCH/DELETE .../subtasks/{id}
GET/POST /orgs/{orgId}/tasks/{taskId}/comments       PATCH/DELETE .../comments/{id}
GET      /orgs/{orgId}/tasks/{taskId}/activity       paginated
GET      /orgs/{orgId}/tasks/{taskId}/watchers       PUT/DELETE .../watchers/me (self only, VIEWER+; FR-TASK-014)

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
GET    /orgs/{orgId}/chat-channels          {channels:[{id, provider: slack|teams, name, url_hint, categories[], enabled, created_at}]} (ADMIN; FR-NTF-005, ADR-040) — the webhook URL is never returned
POST   /orgs/{orgId}/chat-channels          {provider, name, url, categories[]} url must be https on hooks.slack.com (Slack) or *.webhook.office.com / outlook.office.com / *.logic.azure.com (Teams) else 422; max 10 per org → 201 {channel}
PATCH  /orgs/{orgId}/chat-channels/{channelId}  {name, categories[], enabled} (the URL is replaced by delete + create)    DELETE same path removes the channel and its delivery log
POST   /orgs/{orgId}/chat-channels/{channelId}/test   queues a test message → 202 {delivery}
GET    /orgs/{orgId}/chat-deliveries        newest 30 {id, channel_id, channel_name, category, title, status: pending|sent|failed, attempts, last_error, created_at, sent_at}
```

API-key requests: org inferred from key (no `{orgId}` mismatch allowed — key
bound to a different org than the path → 403), scope `write` required for all
non-GET, rate limit headers per FR-API-002. Session-auth requests share the
same limiter keyed `rl:api:{orgId}:{userId}`.

### Task versions and claims (ADR-033, FR-AGT-004/005)

Every task carries `version`, bumped by the database on each content change
(not on a rank-only reorder, nor on claim/lease bookkeeping). A write that
sends `version` is applied only if the task is still at it; otherwise 409:

```json
{"error":{"code":"version_conflict","message":"…","details":{"current":{…task…}}}}
```

Without `version` the write is unconditional, as before.

Board columns carry `role` (ADR-033 decision 5): the stage set by a lead, else
inferred from the column name; omitted when none applies. An agent can never
put a task into the done column (move, bulk move, create): 403
`human_approval_required`. Task and board-card
payloads also carry `claimed_by` + `lease_expires_at` while an agent's claim
is live (omitted otherwise: a lapsed claim blocks nobody).

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
  from org keys) — `ai.plan_apply`, `tasks.claim` (assigns the task to the
  agent with a lease, default 30 min, 1–240; renewing its own claim is fine;
  another agent's live claim or a person's assignment → tool error naming the
  holder), `tasks.heartbeat` (extends a live claim; a lapsed one must be
  claimed again), `tasks.release` (ends the claim, unassigns if still the
  assignee). Task summaries carry `version`, `claimed_by`, `lease_expires_at`.
  Agent write tools: `tasks.next` (find and, by default, claim the next ready
  task), `tasks.update` (needs the agent's live claim and `version`; fields
  and/or `column` backlog|todo|in_progress), `tasks.comment`, `tasks.create`
  (`subtasks`, `blocked_by`, never into done), `tasks.submit_for_review`
  (`version`, `summary`: → review column, summary comment, claim ends);
  `tasks.link` is also open to org write keys. `projects.board` columns carry
  `role`.
  Write tools that are not idempotent by nature accept `idempotency_key` (or
  the `Idempotency-Key` header): the first successful result is replayed for
  24 h; the same key with other arguments is refused. Business failures come back as `isError: true` tool
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
POST /api/v1/integrations/github/{orgSlugOrId}/webhook    GitHub `pull_request` webhook (FR-TASK-016, ADR-039); NOT under session auth, per-IP throttled
  Auth: `X-Hub-Signature-256` = `sha256=` + HMAC-SHA256(secret, raw body), verified in constant time BEFORE the body is parsed. Unknown org, no integration and bad signature are all 401.
  Headers: `X-GitHub-Event` (`ping` → 200 pong; `pull_request`; anything else 422), `X-GitHub-Delivery` (required, deduped per org: replays answer 200 `duplicate`, nothing written). Body max 1 MiB (413).
  `pull_request` actions opened/edited/closed/reopened; others and repos outside the allow-list answer 200 `ignored`. Task keys `WEB-12` (case-insensitive, max 20) in the title and head branch link the PR to matching live tasks of that org only. A merged PR (`closed` + `merged`) moves linked tasks of projects with move-on-merge to the done column through the automation move path (dependency guard applies; a refused move is logged, the delivery still succeeds).
  200 → {status: processed|duplicate|ignored|pong}
GET  /healthz                   200 static
GET  /readyz                    checks: PG ping, Redis ping, migrations current → 200/503
GET  /metrics                   Prometheus (bound to internal interface)
```
