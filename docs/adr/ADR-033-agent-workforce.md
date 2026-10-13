# ADR-033 — Agents as first-class workers (team-shared, plan-capped, human-approved Done)

Date: 2026-10-13. Status: accepted (product owner decisions recorded below).
Implements FR-AGT-001..008. Builds on ADR-025 (governed AI) and ADR-034 (MCP server).

## Context

Fluxboard was built for human teams. The target use now includes one person
running many coding agents (e.g. 10) that connect over MCP and work a project
themselves: pick a task, do it, report, hand over. Today that cannot work:

- An API key is an org-level credential with a name. Work done through it is
  attributed to `apikey:<id>`; a task cannot be assigned to it, and ten agents
  are indistinguishable on the board.
- Nothing stops two agents from taking the same task or overwriting each
  other's update (no claim, no version check).
- Columns are free text, so an agent cannot know which column means "in
  progress" or "done".
- Seats are non-guest members and drive the Stripe quantity, so agents as
  members would be billed like people.

## Decision

1. **Agent principal.** A new kind of org principal, `agent`, with its own id,
   name, avatar and API key(s). An agent belongs to the org (team-shared, not
   personal): any ADMIN+ may create, pause, or delete it; it records who
   created it. It can be assigned tasks and @mentioned, appears in activity
   and audit as itself ("backend-2"), and acts with an org role chosen at
   creation: MEMBER (default) or GUEST, never OWNER/ADMIN. (The org roles are
   OWNER/ADMIN/MEMBER/GUEST; write access to a project comes from a project
   role such as CONTRIBUTOR, as for a person.)
2. **Not a seat; capped per plan.** Agents never count toward seats or the
   Stripe quantity. Each plan has `max_agents` (Free 1, Pro 10, Business
   unlimited = -1), enforced only on creation with the standard 402
   `plan_limit_exceeded` (`limit=max_agents`, current, max). Paused agents
   still count (pausing must not dodge the cap); deleting frees a slot. On a
   downgrade, existing agents keep working but no new one can be created until
   the org is under the cap. Platform admins can override per org (existing
   overrides mechanism).
3. **Claim with a lease.** `tasks.claim` atomically assigns an unclaimed task
   to the calling agent with a lease (default 30 min). The agent renews it with
   `tasks.heartbeat`; an expired lease auto-releases, so a crashed agent never
   locks work. A human assignee or an active lease blocks other claims (409
   with the current holder).
4. **Optimistic concurrency.** Tasks get a monotonically increasing `version`.
   Every agent write carries the version it read; a stale write is 409
   `version_conflict` with the current task, never a silent overwrite. Write
   tools also accept an idempotency key (24 h, existing Redis store).
5. **Column roles.** Each project maps its columns to fixed roles: `backlog`,
   `todo`, `in_progress`, `review`, `done` (defaults applied to the existing
   default columns). Agents move tasks by role, not by column name.
6. **Done is human-approved.** An agent can move a task as far as `review`
   (with a result summary). Only a human member may move it to `done`; an agent
   attempt is 403 `human_approval_required`. Rejecting a review sends it back
   to `in_progress` with a comment.

## Order of work (each its own PR)

1. MCP server on the standard protocol (ADR-034) — read tools + existing AI tools.
2. Agent principal + `max_agents` (decisions 1–2), web management page.
3. Claim/lease + versioning + idempotency (decisions 3–4).
4. Write tools + column roles + review gate (decisions 5–6).

## Implementation notes (step 2)

- **Identity.** An agent is backed by a `users` row with the same id, so every
  existing `users(id)` reference (task creator/assignee, comment author,
  activity actor, AI run caller, project membership) works unchanged and shows
  the agent's name. The row has no password, an unroutable RFC 2606 address
  (`agent-<id>@agents.fluxboard.invalid`) and no membership: it can never sign
  in, is never mailed (notifications are in-app only) and is never a seat.
  Org-side state lives in `agents` [T] (migrations 0034/0035).
- **Keys.** An agent holds up to 3 active read+write keys (rotation); its role,
  not the key scope, bounds what it may do. An agent key resolves to the
  agent's user id and role; a paused agent's key is 403 `agent_paused`. Org
  keys keep the synthetic `apikey:<id>` principal. No API key — an agent's
  included — may use user-scoped routes (`/me`, `GET|POST /orgs`, invitation
  accept): 403.
- **Delete is soft.** `deleted_at` frees the slot and revokes the keys; history
  keeps the name.
- **Overrides.** Entitlement resolution now applies the platform-admin
  `entitlement_overrides` rows (any numeric limit, e.g. `max_agents`) on top
  of the plan; they were stored but not applied before.
- **MCP.** `ai.plan_apply` is agent-only: listed for and callable by an
  agent's key, hidden from org keys.

## Implementation notes (step 3)

- **Version.** `tasks.version` is bumped by a `BEFORE UPDATE` trigger whenever
  any column other than `rank`, `updated_at`, `version` and the claim columns
  changes, so every writer counts (REST, bulk, automations, agents) and a
  caller can never set it. A rank-only reorder and lease renewals do not bump
  it: an agent's edit should not fail because a card was dragged within its
  column. REST `PATCH /tasks/{id}` and `/position` take an optional `version`;
  the web task editors send it, so a person can no longer silently overwrite
  an agent's change (409 → the latest is shown).
- **Claim rules** (one conditional `UPDATE`, so two agents racing for a task
  cannot both win): a claim succeeds when no other agent holds a live lease
  and the assignee is empty, the caller, or the holder of a lapsed claim. A
  claim assigns the task to the agent. Heartbeat extends only a live claim
  the caller holds; release clears the claim and the agent's assignment.
  Lease: default 30 min, 1–240. A lapsed claim is inert; the
  `maintenance:claim_sweep` job (every minute) clears it and the assignment it
  made. Deleting an agent releases its claims at once.
- **Idempotency.** The MCP server adds key-based replay (Redis, 24 h) to write
  tools that are not idempotent by nature; tools idempotent by design (claim,
  heartbeat, `ai.plan_apply` with its own key) are left alone.

## Implementation notes (step 4)

- **Column roles.** `board_columns.role` (migration 0037) holds an explicit
  stage set by a project lead (`PUT …/columns/{id}/role`, one column per role
  per board, moved atomically). A column without one gets its role inferred
  from its name ("Todo", "In Progress", "Code Review", "Done", …), so existing
  boards needed no data change. New projects get five columns: Backlog, Todo,
  In Progress, **Review**, Done. "Done" for the dependency guard and velocity is
  now the done role, else the last column (as before).
- **Human-approved Done** is enforced in the usecases for every path that can
  put a task in the done column: drag/`PATCH …/position`, move by role, bulk
  move, task creation (incl. `ai.plan_apply`). The caller is an agent when it
  has an `agents` row (`agent.Lookup`).
- **Review.** `tasks.submit_for_review` moves the task to the review column
  (version-checked), posts the summary as a comment and ends the claim; the
  agent stays the assignee. A person decides with `POST /tasks/{id}/review`:
  `approve` → done; `changes` (comment required) → in progress (todo if the
  board has none), same assignee. Agents get 403 `human_approval_required`.
- **Write tools** (agent-only except `tasks.link`): `tasks.next` (claims by
  default: work it already holds, then work sent back to it, then the most
  urgent ready todo — unclaimed, unassigned or its own, every blocker done),
  `tasks.update` (needs the agent's live claim and the version it read; may
  move to backlog/todo/in_progress), `tasks.comment`, `tasks.create` (with
  checklist subtasks and blockers, never into done), `tasks.link`,
  `tasks.submit_for_review`.

## Rejected

- **Agents as members/seats:** bills an automated worker like a person and
  ties the agent count to Stripe quantity changes.
- **Agents as plain API keys with a label:** cannot be assigned, mentioned or
  shown on the board; attribution stays `apikey:<id>`.
- **Agent may close its own work:** no human checkpoint before work counts as
  done; the product owner chose human-approved Done.
- **Pessimistic locking without expiry:** a crashed agent would hold tasks
  forever.

## Consequences

- Migrations: an `agents` table (+ RLS), `plans.max_agents`, `tasks.version`,
  claim/lease columns, column-role mapping. Numbers assigned per PR.
- Tenancy/RBAC (05) gains the agent principal; the API-key path resolves to an
  agent principal instead of a synthetic `apikey:<id>` user when the key
  belongs to an agent.
- Billing (06 §7) gains the `max_agents` entitlement; seats are unchanged.
