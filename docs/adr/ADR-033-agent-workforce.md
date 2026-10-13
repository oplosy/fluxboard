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
   creation (CONTRIBUTOR by default, never OWNER/ADMIN).
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
