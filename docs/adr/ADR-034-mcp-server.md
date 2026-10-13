# ADR-034 — MCP server on the standard protocol (dual-era, Streamable HTTP)

Date: 2026-10-13. Status: accepted. Implements FR-AI-007 and FR-AGT-001.
Supersedes the ad-hoc `POST /orgs/{orgId}/mcp` endpoint of ADR-025.

## Context

FR-AI-007 asked for "`POST /api/v1/mcp` JSON-RPC". What shipped was a custom
`{tool, params}` envelope under `/orgs/{orgId}/mcp` with eight AI tools. No
MCP client (Claude Code, Cursor, …) can connect to it, read-scoped keys got
403 even for read tools (POST hit the write-scope guard), and an agent had no
tool to read a project or a task.

The MCP specification's current revision is **2026-07-28** (stateless: the
protocol version travels on every request in `_meta` and the
`MCP-Protocol-Version` header, no `initialize` handshake, `server/discover`
mandatory). Clients in the field still speak the handshake-based revisions
**2025-11-25 / 2025-06-18 / 2025-03-26**.

## Decision

1. **One endpoint:** `POST /api/v1/mcp`, Streamable HTTP, JSON responses
   (`application/json`; no SSE needed for request/response tools). `GET` and
   `DELETE` → 405. No sessions are minted (`Mcp-Session-Id` is ignored).
2. **Dual-era server.**
   - *Modern* (2026-07-28): a request whose `params._meta` carries
     `io.modelcontextprotocol/protocolVersion`. The server validates
     `MCP-Protocol-Version`, `Mcp-Method` and (for `tools/call`) `Mcp-Name`
     against the body (base64 sentinel decoded) → `400` + `-32020
     HeaderMismatch`; an unsupported version → `400` + `-32022` with
     `supported`/`requested`; missing required `_meta` fields → `400` +
     `-32602`; unknown method → `404` + `-32601`. Results carry
     `resultType: "complete"` and `_meta.io.modelcontextprotocol/serverInfo`.
     Methods: `server/discover`, `tools/list`, `tools/call`, `ping`.
   - *Legacy* (2025-11-25, 2025-06-18, 2025-03-26): `initialize` negotiates
     the version (the client's if supported, else the newest legacy one);
     later requests carry the version in `MCP-Protocol-Version` (absent ⇒
     2025-03-26). Methods: `initialize`, `ping`, `tools/list`, `tools/call`;
     notifications (`notifications/initialized`, …) → `202`.
3. **Auth = API key.** `Authorization: Bearer fbk_…`. The org comes from the
   key (no org id in the URL), so a client is configured with one URL and one
   header. Session tokens are not accepted on this endpoint. The usual chain
   still applies after auth: tenant context, plan rate limiter and usage
   metering. `Origin`, when present, must equal `WEB_ORIGIN` (DNS-rebinding
   guard) → 403.
4. **Tools carry their own scope.** Each tool is marked read or write.
   `tools/list` returns only what the key may call (read keys see read
   tools); a write tool called with a read key is a tool error. Tools declare
   `readOnlyHint` / `destructiveHint` / `idempotentHint` annotations.
5. **Errors.** Unknown tool or malformed arguments → JSON-RPC `-32602`.
   Business failures (validation, not found, forbidden, conflict, plan limit,
   rate limit) → a tool result with `isError: true` and a message the model
   can act on. Internal errors → `-32603` without detail.
6. **Results** carry both `structuredContent` and the same JSON as a text
   block (spec back-compat guidance).
7. **Initial tool set:** the eight governed-AI tools (`ai.*`, unchanged
   semantics, still metered/audited by aiuc) plus read tools for agents:
   `projects.list`, `projects.board`, `tasks.search`, `tasks.get`
   (with subtasks, comments, blockers). Write tools land with claim/version
   control (ADR-033 step 3–4).

## Rejected

- **Keeping `/orgs/{orgId}/mcp`:** not MCP; no consumer depends on it (the web
  app never called it). Removed rather than maintained twice.
- **Modern-only server:** today's Claude Code/Cursor builds may still open
  with `initialize`; a modern-only server would fail them.
- **SSE responses / `subscriptions/listen` now:** no tool needs streaming
  yet; events for agents are a later step (ADR-033 roadmap).
- **OAuth authorization server for MCP:** API keys already exist, are
  org-bound and revocable; OAuth (spec "Authorization") can follow.

## Consequences

- New package `internal/interface/http/mcp` (protocol + tool registry);
  `handlers/mcp.go` and its route are removed, its tests move with it.
- Client setup is one line, e.g.
  `claude mcp add --transport http fluxboard https://<host>/api/v1/mcp --header "Authorization: Bearer fbk_live_…"`.
