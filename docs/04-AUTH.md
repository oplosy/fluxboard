# 04 — Authentication Specification

Implements FR-AUTH-001…015. Standards references: RFC 6749 (OAuth2),
RFC 7636 (PKCE), RFC 6750 (Bearer), OWASP ASVS v4 §2/§3.

## 1. Credential Storage

Argon2id via `alexedwards/argon2id`:

```go
var Params = &argon2id.Params{
    Memory: 64 * 1024, Iterations: 3, Parallelism: 2,
    SaltLength: 16, KeyLength: 32,
}
// stored format: $argon2id$v=19$m=65536,t=3,p=2$<salt>$<hash>
```

Password checks on register/reset/change: length ≥ 12, ≤ 128 **characters**
(Unicode code points, not bytes); not in embedded top-10k breach list
(`embed.FS`, lowercase exact match); not equal to the email or its local
part. Each rejection has its own code (§7). Comparison via
`argon2id.ComparePasswordAndHash` (constant-time).

## 2. Token Model

| Token | Form | TTL | Transport | Storage |
|---|---|---|---|---|
| Access | JWT ES256 | 15 min | `Authorization: Bearer` | client memory only |
| Refresh | 32B random, base64url | 7 d sliding | httpOnly Secure SameSite=Strict cookie, `Path=/api/v1/auth` | DB: SHA-256 hash |
| Email verify | 32B random | 24 h | email link | DB: SHA-256 hash, single-use |
| Password reset | 32B random | 1 h | email link | DB: SHA-256 hash, single-use |
| Pending-2FA | JWT, `scope=pending_2fa` | 5 min | JSON body (OAuth: URL fragment) | stateless |
| Invitation | 32B random | 7 d | email link | DB: SHA-256 hash |
| API key | `fbk_live_` + 32B | until revoked | Bearer | DB: SHA-256 hash |

JWT claims (FR-AUTH-004):

```json
{"iss":"fluxboard","aud":"fluxboard-api","sub":"<user_uuid>",
 "sid":"<session_uuid>","iat":1720000000,"exp":1720000900}
```

ES256 keypair loaded from env/PEM files; `kid` header present; key rotation =
publish new key, keep old for verification until max access TTL passes
(15 min) — a two-element in-process JWKS.

Access token validation also checks the session: `sid` looked up in Redis
(`sess:{sid}` → status, 15 min TTL, backfilled from PG on miss). Revoked
session ⇒ 401 even if signature/exp valid. This bounds revocation latency to
one cache round-trip, not token expiry.

## 3. Session & Refresh Rotation (FR-AUTH-005/006)

Table `sessions` (see 07 §2): each login creates a session row = one refresh
token **family** (`family_id`). Refresh flow, single serializable transaction:

```
1. hash = SHA256(presented_token); SELECT ... FOR UPDATE by token_hash
2. not found                    → 401
3. revoked_at != null           → 401
4. rotated_at != null           → REUSE DETECTED:
       UPDATE sessions SET revoked_at=now(), revoke_reason='token_reuse'
       WHERE family_id = $family;
       audit(severity=security, action=auth.refresh_reuse); → 401
5. expires_at < now()           → mark revoked → 401
6. otherwise: SET rotated_at=now();
   INSERT new row (same family_id, new token_hash, expires_at=now()+7d);
   issue new access JWT (same sid);
   Set-Cookie new refresh token
```

Concurrency note: two parallel refreshes with the same token — the second
hits `rotated_at != null` inside the row lock and triggers family revocation.
Frontend prevents the benign case with a **single-flight refresh mutex**
(03 §4): all concurrent 401-retries await one refresh promise. Across tabs
the refresh cookie is shared, so refreshes are also serialized with the Web
Locks API (`navigator.locks`, lock `fluxboard-auth-refresh`): inside the lock
the browser always sends the newest cookie. The client refreshes ~60 s before
access-token expiry (and on tab focus after sleep) instead of waiting for a
401, and a refresh that fails on the network keeps the current token.

Session marker: the web origin keeps a non-secret `fluxboard_session=1` cookie
(SameSite=Lax, 7 d, renewed on refresh) while it holds an access token, and
clears it when the token is dropped. At boot the client only calls
`/auth/refresh` when the marker exists or the page needs a session (`/app`,
`/account`, `/admin`, `/verify-email`, `/invite/…`), so signed-out visitors on
public pages make no doomed refresh (no 401). Guards on protected pages still
attempt one restore before redirecting to `/login`, so a browser with a valid
refresh cookie but no marker is never signed out by mistake. Logout first
settles any in-flight restore, so it always revokes the real session and a
late refresh cannot sign the user back in.

## 4. Login Flows

**Password login:** rate-limit gate (FR-AUTH-011, Redis key
`rl:login:{sha1(email)}:{ip}`) → user lookup (unknown email: run Argon2id
against a static dummy hash — uniform timing) → verify → if TOTP enabled
return `{status:"2fa_required", pending_token}` → else create session,
return access token + set refresh cookie. Failures return identical
`invalid_credentials` regardless of cause (unknown email, wrong password,
locked account, OAuth-only account). A tripped throttle returns 429
`rate_limited` with `Retry-After` and `details.retry_after` (seconds; the
header is not readable cross-origin).

The client IP used by every per-IP throttle comes from `RemoteAddr`, or from
`X-Forwarded-For` only for the `TRUSTED_PROXY_HOPS` entries our own proxies
append (the N-th from the right). The leftmost XFF entry is client-controlled
and never trusted, so it cannot be rotated to dodge the login throttle.

**TOTP step (FR-AUTH-013):** `POST /auth/2fa/verify {pending_token, code}` —
validates JWT scope, checks TOTP (±1 window, 30s step) or a recovery-code
hash (consumed on use) → creates session. Spaces/dashes in codes are ignored.
Per-IP throttled. Bad code → 401 `invalid_code`; expired/stale pending token
→ 401 `pending_2fa_expired` (client restarts at /login).

**OAuth: Google + GitHub (FR-AUTH-009, ADR-032):**
`GET /auth/oauth/providers` lists the configured providers (the UI only shows
those buttons). `GET /auth/oauth/{provider}/start` generates `state` (32B) +
PKCE `code_verifier` (43–128 chars), stores
`oauth:{state} → {verifier, redirect_after, provider}` in Redis (10 min),
pins `state` to the browser in an httpOnly `SameSite=Lax` cookie
(`fluxboard_oauth_state`, `Path=/api/v1/auth/oauth`), and redirects with
`code_challenge=S256`. Callback: the query `state` must equal the cookie
(constant-time; blocks login CSRF with a lured callback URL), must exist in
Redis (delete on read) and belong to the same provider; exchange
code+verifier; require a provider-verified email (Google `email_verified`,
GitHub verified primary address from `/user/emails`), then:

```
identity (provider,sub) exists          → login that user
else email matches existing VERIFIED user → link identity, login
else email matches UNVERIFIED user      → oauth_verify_first (no silent takeover)
else                                    → create user (email_verified=true), login
```

A locked account is refused (`account_locked`). An account with TOTP on gets a
pending-2FA token instead of a session — OAuth never skips the second factor.
On success the API sets the refresh cookie and redirects to
`{WEB_ORIGIN}/oauth/callback?next=<path>`; the page mints its access token via
`/auth/refresh`. **No access token ever travels in a URL.** The 2FA case
redirects to `/oauth/callback?next=…#pending_token=…` (fragment, never sent to
servers; moved to sessionStorage and stripped from history). Every failure —
including the user cancelling at the provider — redirects to
`/login?oauth_error=<code>` instead of rendering JSON. Provider HTTP calls
time out after 10 s.

## 5. Endpoint Summary (details in 08-API-SPEC.md)

```
POST /auth/register            POST /auth/login            POST /auth/2fa/verify
POST /auth/refresh             POST /auth/logout           POST /auth/logout-all
GET  /auth/oauth/providers     GET  /auth/oauth/{google|github}/start
GET  /auth/oauth/{google|github}/callback
POST /auth/verify-email/request        POST /auth/verify-email/confirm
POST /auth/verify-email/resend (auth; to the caller's own address)
GET  /auth/me                  (auth; id/email/name/email_verified/has_password —
                                reachable before verification, unlike GET /me)
POST /auth/password/forgot     POST /auth/password/reset   POST /auth/password/change
POST /auth/password/reset/validate  {token} → 204 | 422 token_invalid (does not consume)
GET  /auth/sessions            DELETE /auth/sessions/{id}
POST /auth/2fa/enroll          POST /auth/2fa/activate     DELETE /auth/2fa
```

Flow details:

- **Register** validates email (RFC 5322 addr-spec, dotted domain, no CR/LF,
  ≤ 254), name (≤ 100 chars) and the password policy; a taken email is a 409
  `email_taken` on the `email` field (ADR-032). A mail delivery failure does
  not fail registration (the verify screen can resend).
- **Verify email** consumes the token, marks the user verified and retires the
  user's other outstanding verification links.
- **Forgot password** is silent for unknown/locked addresses; OAuth-only
  accounts *do* get a link, so they can add a password.
- **Reset password** checks the policy *before* consuming the single-use token
  (a rejected password never burns the link), then marks the email verified,
  retires other reset links and revokes every session.
- **Change password**: wrong current password is a 422 on `current_password`
  (not 401 — the session is fine), reusing the current password is rejected,
  and an OAuth-only account gets 403 `password_not_set`.
- Mail: multipart (text + HTML button) with Date/Message-ID, RFC 2047 subject,
  quoted-printable bodies, and a 15 s deadline on the SMTP conversation.

`/auth/refresh` CSRF posture: SameSite=Strict cookie + required header
`X-Requested-With: fetch` (server rejects otherwise) + `Origin` allowlist.

## 6. Security Requirements Checklist (tested in 12 §4)

- [x] Timing-uniform login for unknown vs wrong-password vs locked (dummy hash)
- [x] No user enumeration on login, forgot-password, invite. **Register deliberately reports `email_taken`** (ADR-032); it is per-IP throttled (20 / 15 min)
- [x] All single-use tokens stored hashed; consumed transactionally (`UPDATE … WHERE used_at IS NULL RETURNING`); a used link retires its siblings
- [x] Password change/reset revokes sessions per FR-AUTH-010/012
- [x] Refresh reuse revokes family + security audit entry
- [x] Cookie flags: HttpOnly, Secure, SameSite=Strict, scoped Path
- [x] `WWW-Authenticate: Bearer` on 401; no token material ever logged or put in a URL (OAuth included)
- [x] Argon2id params asserted by a unit test (regression guard against silent downgrade)
- [x] OAuth state bound to the browser (cookie) and to the provider; OAuth login honours TOTP and account lock
- [x] Per-IP throttles key on a trusted-proxy-aware client IP (forged `X-Forwarded-For` ignored)
- [x] 2FA verify per-IP throttled

## 7. Error Codes (contract with the web client)

Envelope per 08 §1. Field errors also carry `details.fields = {<field>: <message>}`.
Clients branch on `code`, never on `message`.

| Code | Status | Field | When |
|---|---|---|---|
| `invalid_credentials` | 401 | — | login failed (any cause) |
| `rate_limited` | 429 | — | throttle tripped; `details.retry_after` seconds |
| `email_invalid` | 422 | email | not a valid address |
| `email_taken` | 409 | email | register with an existing email |
| `name_too_long` | 422 | name | > 100 characters |
| `password_too_short` / `password_too_long` | 422 | password / new_password | outside 12–128 characters |
| `password_breached` | 422 | password / new_password | in the breach list |
| `password_matches_email` | 422 | password / new_password | equals the email or its local part |
| `password_unchanged` | 422 | new_password | new = current |
| `current_password_incorrect` | 422 | current_password | change-password check failed |
| `password_not_set` | 403 | — | change-password on an OAuth-only account |
| `token_invalid` | 422 | — | verify/reset link missing, expired, used or retired |
| `invalid_code` | 401 / 422 | code | wrong TOTP/recovery code (login step / account settings) |
| `pending_2fa_expired` | 401 | — | pending-2FA token expired or stale |
| `account_locked` | 403 | — | OAuth login to a locked account |
| `oauth_unavailable`, `oauth_cancelled`, `oauth_failed`, `oauth_state_invalid`, `oauth_email_unverified`, `oauth_verify_first` | redirect | — | `/login?oauth_error=<code>` |
