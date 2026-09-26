# Platform admin access for Helpin admin panel

**Status:** Historical v2 requirements; core gate implemented with differences below
**Author:** azhar
**Date:** 2026-04-28
**Target:** before deploying `apps/admin` to `ctrl.helpin.ai`

---

This PRD records the original platform-admin security boundary and rollout design.
Use the source notes below to understand today's gate; the original problem and
unchecked rollout items do not establish the current deployment state.

## Source review — 2026-09-18

- [RequirePlatformAdmin](../../server/internal/authorization/platform_admin.go)
  requires an access token with both platform-admin and MFA claims. The
  [router](../../server/internal/router/router.go) wraps admin routes with audit,
  authentication, optional authenticated rate limiting, then that gate. The old
  “any signed-in customer” route description is historical.
- [JWT issuance](../../server/internal/auth/jwt.go) distinguishes access/refresh
  use and carries MFA/admin claims. `RequireAuth` rejects non-access tokens.
  [Refresh](../../server/internal/service/auth.go) reloads the user and reissues
  admin status while preserving the MFA claim. The gate itself remains
  claim-only, so revocation does not invalidate an already issued access token.
- Legacy untyped refresh acceptance is based on token expiry exceeding
  `legacyRefreshFloor`; the inspected helper has no explicit deployment-date
  deadline. Do not describe the proposed bounded migration window as a current
  calendar cutoff enforced by that helper.
- Startup grants admin status to **existing** users matching configured emails;
  [the repository method](../../server/internal/repository/user.go) does not create
  accounts or revoke omitted addresses. Leaving a revoked email in the grant list
  can grant it again on a subsequent startup.
- [The admin login](../../apps/admin/src/pages/login-page.tsx) includes password,
  TOTP/recovery-code, and passkey flows. It did not adopt the proposed passkey-only
  Option B. The auth store validates admin/MFA status before keeping a session.
- [Audit middleware](../../server/internal/middleware/admin_audit.go) logs request
  metadata without bodies and can parse token claims independently after the
  inner handler. This is not a verified durable audit store or proof that every
  panic/unmatched route produces an audit event.
- This source review did not inspect live grants, deployed origins, passkey
  compatibility, or production audit retention, and did not rerun security tests.

## Original requirements

## 1. Problem

The `apps/admin` SPA exposes operational tooling (Webhook Events, Email Queue, Chat Playground, soon Git Webhooks) that should only be reachable by Helpin staff. Today:

- `users` has no platform-level role flag. The only authorization layer is workspace-scoped RBAC (`server/internal/authorization`).
- `/api/admin/*` is registered under standard JWT-protected routes (`server/internal/router/router.go:362–366`). Any signed-in customer can call them with their access token.
- The admin login page reuses `POST /api/auth/signin`. Once `ctrl.helpin.ai` is public, every customer can authenticate.
- Email Queue and Webhook Events expose **cross-tenant** data (raw inbound message snippets, recipient emails, payload bodies). This is a multi-workspace data leak waiting to happen.

We need a platform-admin layer before deploy.

## 2. Goals

1. Only users explicitly designated as Helpin platform admins can log in to `ctrl.helpin.ai`.
2. Only platform admins can call `/api/admin/*` endpoints regardless of client.
3. Granting and revoking admin status is controlled and auditable.
4. Admin sessions are higher-assurance than regular sessions (require MFA — TOTP and/or passkey-with-UV; see §4.4).
5. Both successful and denied admin requests are logged for audit.
6. Existing main-app users — including platform admins who have not yet enabled MFA — can continue to sign in to the main app and self-serve MFA setup.

## 3. Non-goals

- Sub-roles inside platform admin. Single boolean for v1.
- Per-IP allowlisting or SSO.
- Self-service admin invitation UI.
- Migrating workspace-scoped RBAC.

## 4. Proposed solution

### 4.1 Data model

```sql
ALTER TABLE users
ADD COLUMN IF NOT EXISTS is_platform_admin boolean NOT NULL DEFAULT false;

CREATE INDEX IF NOT EXISTS idx_users_is_platform_admin
  ON users(is_platform_admin)
  WHERE is_platform_admin = true;
```

Migration: `server/internal/dbmigrate/sql/YYYYMMDDNNNN_add_user_is_platform_admin.sql`.

GORM (`server/internal/model/user.go`):

```go
IsPlatformAdmin bool `json:"is_platform_admin" gorm:"column:is_platform_admin;not null;default:false"`
```

Naming: `is_platform_admin`, distinct from workspace `admin` role.

### 4.2 Bootstrap and revocation semantics

`PLATFORM_ADMIN_EMAILS` (Doppler, comma-separated). On startup, after migrations, the server upserts `is_platform_admin = true` for matching emails (case-insensitive).

**The env var is grant-only.** Removing an email does not auto-revoke. Authoritative state is the DB column. Revocation requires an explicit DB write (or future admin endpoint).

Acceptance language reflects this: "a user's admin status is whatever `users.is_platform_admin` says, regardless of the env var's current contents."

### 4.3 Token-type discriminator (prerequisite)

**Currently broken:** `auth.Claims` (`server/internal/auth/jwt.go:10-15`) is identical for access and refresh tokens. `RequireAuth` (`server/internal/middleware/auth.go:31`) accepts any valid `Claims` JWT as a bearer token. A 24h–30d refresh token can today be used directly against any `/api/*` endpoint. This must be fixed before anything depending on JWT identity ships.

**Change:**

```go
type Claims struct {
    UserID   string `json:"user_id"`
    Email    string `json:"email"`
    TokenUse string `json:"tu"` // "access" | "refresh"
    jwt.RegisteredClaims
}
```

- `GenerateTokenPair` sets `TokenUse = "access"` on the access token, `"refresh"` on the refresh token.
- `RequireAuth` rejects any token whose `TokenUse != "access"` with 401.
- `RefreshToken` handler accepts `TokenUse == "refresh"`; during the bounded migration grace window it may also accept legacy-empty tokens as refresh tokens and immediately replace them with typed tokens.
- Backwards compat: tokens issued before the change have an empty `tu`. `RequireAuth` must reject empty/refresh tokens immediately; old access tokens naturally expire within 15 minutes. The refresh endpoint may accept an empty `tu` token as a legacy refresh token for one refresh-token TTL window (up to 30 days), minting a new typed pair. This preserves sessions without allowing legacy refresh tokens to keep acting as bearer access tokens.

This is a foundational fix. It is **not** specific to admin gating, but no admin gating is meaningful until it lands.

### 4.4 MFA model and admin signin

The admin panel must guarantee that a session reaching `/api/admin/*` was authenticated with MFA. Two signals exist today:

- TOTP via `POST /api/auth/2fa/verify-signin` (after `requires_2fa` from signin).
- Passkey signin where `credential.Flags.UserVerified` is true (`server/internal/service/passkey.go:234`).

The proposal treats passkey-with-UV and TOTP as MFA-satisfying for platform admins.
User verification alone does not establish that a credential is hardware-backed. A passkey **without** UV does not satisfy MFA.

#### `mfa_satisfied` claim

We add a JWT claim that is set true at token-issuance time only when:

- TOTP code was just verified (in `2fa/verify-signin`), or
- Passkey login completed with `UserVerified == true`. For WebAuthn, this means possession of the passkey plus local user verification (biometric, device PIN, or equivalent), so it satisfies the admin MFA requirement without an additional TOTP prompt.

```go
type Claims struct {
    UserID         string `json:"user_id"`
    Email          string `json:"email"`
    TokenUse       string `json:"tu"`
    MFASatisfied   bool   `json:"mfa,omitempty"`
    IsPlatformAdmin bool  `json:"pa,omitempty"`
    jwt.RegisteredClaims
}
```

Refresh token rotation preserves `mfa_satisfied` — once you've MFA'd, refreshes don't re-prompt.

`is_platform_admin` is included in the access-token claim for cheap middleware checks. It is **not** treated as authoritative from refresh-token claims. On every refresh, `AuthService.RefreshToken` reloads the user from the DB and recomputes `IsPlatformAdmin` from `users.is_platform_admin`; it may preserve `MFASatisfied` from the valid refresh token. This keeps admin revocation staleness to the access-token TTL.

Acceptable staleness window = access-token TTL (15 min). Sufficient given:

- Token-type fix prevents long-lived refresh tokens from acting as access tokens (§4.3).
- Refresh recomputes platform-admin status from the DB instead of preserving stale `pa` from the refresh token.
- Revocation is rare and the operator can flush an active session by writing a token blacklist later if we hit a real incident.

If a future audit finding requires immediate revocation, we revisit with a DB-lookup variant of the middleware. Documented as an open question (§8) for explicit acknowledgment.

#### Why we do not gate at `/api/auth/signin`

Earlier draft proposed returning 403 from signin if a platform admin lacked TOTP. **Rejected.** `signin` is shared with the main app (`server/internal/handler/auth.go:40`); blocking it would prevent platform admins from logging into the main app to enable TOTP — the very path the PRD relies on.

Enforcement moves to the admin gate (§4.5). A platform admin without MFA can still use the main app, and they simply cannot reach `/api/admin/*` until they enable MFA. The admin SPA's `/forbidden` page links to the main-app TOTP setup screen.

#### Admin SPA flow (concrete)

Currently the admin SPA treats `requires_2fa: true` as an error and refuses to proceed (`apps/admin/src/stores/authStore.ts:74` and `:90`). That is incompatible with this PRD. Two acceptable v1 options:

- **Option A — Admin TOTP screen:** add `/admin/2fa` with the same input UX the main app already uses, calling `POST /api/auth/2fa/verify-signin`. After success, run the post-signin admin check (see below).
- **Option B — Passkey-only admin login:** remove the password form from the admin login page. Only passkey signin (with UV) is allowed. This keeps the SPA narrow and dodges the duplicated TOTP UX.

**Recommendation: Option B for v1.** Justification: scope reduction (no TOTP UX to duplicate), stronger phishing resistance, aligns with where industry is moving. Trade-off: every platform admin must register a passkey in the main app first. Acceptable — passkey enrollment already exists.

If we go with B, the admin login page shows passkey-only with "Register a passkey in the main app" copy. The password fields are removed. Changes to `apps/admin/src/pages/login-page.tsx` and `authStore.ts` are scoped.

#### WebAuthn relying-party configuration for Option B

Passkey-only admin login only works if passkeys are valid on both the main app and `ctrl.helpin.ai`.

Current WebAuthn config derives the relying-party ID and allowed origins from app config unless `WEBAUTHN_RP_ID` / `WEBAUTHN_RP_ORIGIN` are set. For production admin login:

- Set `WEBAUTHN_RP_ID=helpin.ai` so credentials are scoped to the registrable domain and usable on both `app.helpin.ai` and `ctrl.helpin.ai`.
- Set `WEBAUTHN_RP_ORIGIN` to a comma-separated list including the main app origin and admin origin, e.g. `https://app.helpin.ai,https://ctrl.helpin.ai`.
- Verify stage equivalents before rollout.
- Confirm existing passkeys were registered under a compatible RP ID. If existing credentials were registered under a narrower host-only RP ID, affected admins must re-register passkeys after the RP config change.

#### Post-signin admin check

After the SPA receives an access token (whether from passkey or TOTP-verify path):

1. Decode claims locally for fast UX (don't trust them — backend re-validates).
2. If `is_platform_admin !== true` or `mfa_satisfied !== true`, clear local storage, stop token refresh, route to `/forbidden`.
3. Otherwise, persist session, route to `/`.

The `/auth/me` response includes `is_platform_admin` so the SPA can also re-check on app reload.

### 4.5 Admin gate (backend)

```go
// server/internal/authorization/platform_admin.go
func RequirePlatformAdmin(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        c := middleware.ClaimsFrom(r.Context())
        if c == nil || c.TokenUse != "access" {
            writeError(w, http.StatusUnauthorized, "auth required")
            return
        }
        if !c.IsPlatformAdmin || !c.MFASatisfied {
            writeError(w, http.StatusForbidden, "platform admin access required")
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

Applied to a dedicated `/admin` route group, moved out of the generic protected route group so audit logging can wrap auth failures too:

```go
r.Route("/admin", func(r chi.Router) {
    r.Use(adminAuditLogger)         // see §4.7; outermost for /admin
    r.Use(middleware.RequireAuth(jwtManager))
    r.Use(RequirePlatformAdmin)
    r.Get("/webhook-events", ...)
    r.Get("/webhook-events/{id}", ...)
    r.Get("/email-queue", ...)
})
```

Both `is_platform_admin` and `mfa_satisfied` checks happen here. No DB lookup — JWT-claim only. Acceptable staleness given §4.3.

### 4.6 Typed auth errors and status mapping

Today every signin failure becomes 401 (`server/internal/handler/auth.go:40`). The SPA cannot distinguish "wrong password" from "policy failure," nor will future flows (rate limiting, lockouts) be cleanly representable.

**Change:**

- Define sentinel errors in `service/auth.go`: `ErrInvalidCredentials`, `ErrTwoFAUnavailable`, etc.
- Handler uses `errors.Is` and maps:

| Error | Status | Body |
|---|---|---|
| `ErrInvalidCredentials` | 401 | `{ "error": "invalid credentials", "code": "invalid_credentials" }` |
| `ErrTwoFAUnavailable` | 503 | `{ "error": "...", "code": "two_factor_unavailable" }` |
| (validation, e.g. missing email) | 400 | `{ "error": "...", "code": "bad_request" }` |
| (unexpected) | 500 | generic |

Add `code` to the JSON body so the SPA can branch on `code`, not on prose. The admin SPA only needs the `code` field; the main app can add it later.

This is small but unblocks future error-handling work and is mandatory for the admin SPA's UX (e.g. "Register a passkey in the main app first" must distinguish "no passkey enrolled" from "wrong creds").

### 4.7 Audit logging

Audit middleware (`server/internal/middleware/admin_audit.go`) wraps the `/admin` group **outside** `RequirePlatformAdmin` so it sees both authorized and denied requests:

```go
func adminAuditLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        rw := &statusRecorder{ResponseWriter: w, status: 200}
        start := time.Now()
        next.ServeHTTP(rw, r)
        c := middleware.ClaimsFrom(r.Context()) // may be nil
        slog.InfoContext(r.Context(), "admin_request",
            "user_id", c.GetUserID(),     // empty if unauthenticated
            "user_email", c.GetEmail(),
            "method", r.Method,
            "path", r.URL.Path,
            "status", rw.status,
            "duration_ms", time.Since(start).Milliseconds(),
        )
    })
}
```

Logged fields: `user_id`, `user_email`, `method`, `path`, `status`, `duration_ms`, `request_id` (from existing context). **Bodies are never logged** — payloads contain raw webhook content and PII.

Order of middleware on `/admin` group:
1. `adminAuditLogger`
2. `RequireAuth`
3. `RequirePlatformAdmin`

This way: 401 from missing/invalid token, 403 from non-admin, 403 from no-MFA, and 200/500 from authorized handlers are all logged by the same audit middleware. Missing/invalid-token attempts have empty `user_id` / `user_email`.

### 4.8 SPA route guard summary

- `requirePlatformAdmin` (router `beforeLoad`):
  - `user == null` → redirect `/login`.
  - `user != null && (!user.is_platform_admin || !user.mfa_satisfied_in_token)` → clear local auth state and redirect `/forbidden`.
  - else → continue.
- `/forbidden` page: explains why, links to main app for MFA setup, has a sign-out button.

## 5. Out of scope (explicit)

- IP allowlist via Cloudflare Access (can layer later).
- Cloudflare Access in front of SPA.
- SSO.
- Per-action authorization within admin (e.g. read-only vs read-write).
- Token blacklist for immediate revocation. Acceptable staleness = access-token TTL.

## 6. Migration and rollout plan

Three-PR rollout, each independently safe and reversible:

1. **PR 1 — Token-type fix (§4.3).** Add `TokenUse` claim, set on issuance, validate on `RequireAuth` and refresh handler. `RequireAuth` rejects non-access and legacy-empty tokens after a short access-token drain window; the refresh endpoint may accept legacy-empty tokens as refresh tokens for one refresh-token TTL and mint typed replacements.
2. **PR 2 — Schema + bootstrap (§4.1, §4.2, §4.6 errors).** Migration, model field, env-var seed, typed auth errors. No behavior change for non-admin users; admins not yet flagged in DB stay non-admin.
3. **PR 3 — Admin gate + SPA changes (§4.4–4.8).** `mfa_satisfied` claim, `is_platform_admin` claim, `RequirePlatformAdmin` middleware, audit middleware, SPA passkey-only login, `/forbidden` page, post-signin admin check.

Cloudflare Pages deploy of `ctrl.helpin.ai` only after PR 3 is in prod and a verification run confirms a non-admin user gets 403 on `/api/admin/email-queue` and the SPA refuses to keep their session.

## 7. Risks and mitigations

| Risk | Mitigation |
|---|---|
| Lockout — no admin in prod | `PLATFORM_ADMIN_EMAILS` set in Doppler before PR 2 deploy. Manual `UPDATE users SET is_platform_admin = true WHERE email = ...` is the break-glass path. |
| JWT claim staleness — admin revoked but token still valid | Acceptable lag = access-token TTL (15 min). Token-type fix prevents refresh-token misuse. Future blacklist documented in §5. |
| Admins without passkey can't reach panel after PR 3 | Pre-deploy comms: enroll a passkey in the main app. `/forbidden` page links to passkey enrollment. |
| Passkeys fail on `ctrl.helpin.ai` because of RP/origin mismatch | Configure `WEBAUTHN_RP_ID=helpin.ai` and allowed origins for main/admin hosts before PR 3; verify in stage. Re-register admin passkeys if existing credentials used a host-only RP ID. |
| Token-type rollout invalidates active sessions | Old access tokens drain within 15 minutes; legacy refresh tokens are accepted only by the refresh endpoint during the grace window and are exchanged for typed tokens. |
| Audit log volume floods stdout | `/admin/*` traffic is low; current slog handler is fine. Reassess if we add high-volume admin endpoints. |
| Customer hits `ctrl.helpin.ai` and gets confused | Clear copy on login + `/forbidden` pages. |

## 8. Open questions

1. **Confirm: passkey-with-UV counts as MFA for platform admins?** If yes, Option B (passkey-only) is viable. If no, we must build admin-side TOTP UX and `mfa_satisfied` is set only after explicit TOTP verification.
2. **Confirm: 15-minute access-token staleness is acceptable for admin revocation?** Or do we need DB-lookup-on-every-request for `RequirePlatformAdmin`?
3. **Confirm: `PLATFORM_ADMIN_EMAILS` is grant-only, never a revocation source?** v2 says yes; revocation requires explicit DB action.
4. **Confirm: typed error `code` field is added to all auth responses, not just admin-relevant ones?** This is a small breaking-shape change for any client reading the body.
5. **Should the admin SPA show a "session expired — re-authenticate" UX after token expiry, or just bounce to login?** v1 leans bounce; can refine later.
6. **Are current production passkeys registered under an RP ID compatible with `helpin.ai`?** If not, admins must re-register after the WebAuthn config update.

## 9. Acceptance criteria

- A user whose `users.is_platform_admin = false` cannot pass the admin SPA login screen and gets 403 on direct calls to `/api/admin/*`.
- A user whose `users.is_platform_admin = true` but who has not completed MFA (no TOTP, no passkey-with-UV in this session) cannot reach `/api/admin/*`. They can still sign into and use the main app.
- A platform admin who logs in via passkey-with-UV can access `/api/admin/*` for the lifetime of their access token.
- Refresh tokens and legacy untyped tokens cannot authenticate against `/api/admin/*` or any other `RequireAuth`-protected endpoint after the PR 1 access-token drain window. Legacy untyped tokens may only be accepted by `/api/auth/refresh` during the bounded refresh grace window.
- Every `/api/admin/*` request — authorized or denied — produces an audit log entry with `user_id`, `path`, `method`, `status`, `duration_ms`. No request or response bodies are logged.
- Migration is idempotent.
- `PLATFORM_ADMIN_EMAILS` adds users on startup. Removing an email from the env does **not** revoke; revocation requires DB write.
- Auth handler returns distinct status codes for invalid credentials (401) vs MFA unavailability (503) vs validation errors (400), each with a stable `code` field.
- WebAuthn RP ID/origins are configured so passkeys registered for Helpin work on both the main app and `ctrl.helpin.ai`, or affected admins have re-registered credentials.

## 10. Implementation checklist (informational)

**PR 1 — Token-type fix**
- [ ] Add `TokenUse` to `auth.Claims`
- [ ] Set `tu="access"` / `tu="refresh"` in `GenerateTokenPair`
- [ ] `RequireAuth` rejects refresh and legacy-empty tokens after the short access-token drain window
- [ ] `RefreshToken` handler accepts typed refresh tokens, plus legacy-empty tokens only during the bounded refresh grace window
- [ ] Tests: refresh token cannot call `/api/auth/me` etc.

**PR 2 — Schema + bootstrap + typed errors**
- [ ] Migration `add_user_is_platform_admin.sql`
- [ ] `User.IsPlatformAdmin`
- [ ] `PLATFORM_ADMIN_EMAILS` parsing in `config.go`
- [ ] Startup seed in `cmd/api/main.go` (post-migration)
- [ ] Sentinel errors in `service/auth.go`, mapping in handler
- [ ] Add `code` field to error responses

**PR 3 — Admin gate + SPA**
- [ ] Add `MFASatisfied`, `IsPlatformAdmin` to access-token claims
- [ ] Refresh path preserves `MFASatisfied` but recomputes `IsPlatformAdmin` from the DB
- [ ] Set `mfa_satisfied=true` in TOTP verify path
- [ ] Set `mfa_satisfied=true` in passkey path **only** when `UserVerified` is true
- [ ] `authorization.RequirePlatformAdmin`
- [ ] `middleware.adminAuditLogger`
- [ ] Move `/admin` to a dedicated group ordered `adminAuditLogger` → `RequireAuth` → `RequirePlatformAdmin`
- [ ] `/auth/me` returns `is_platform_admin`
- [ ] Configure `WEBAUTHN_RP_ID` and `WEBAUTHN_RP_ORIGIN` for main/admin hosts in stage and prod
- [ ] Admin SPA: passkey-only login (Option B)
- [ ] Admin SPA: `/forbidden` page
- [ ] Admin SPA: post-signin admin check + local auth cleanup on failure
- [ ] Doppler: `PLATFORM_ADMIN_EMAILS` set in stage and prod
- [ ] Verify: stage smoke test before prod rollout

## Revision history

- **v2 (2026-04-28):** Incorporated security review.
  - Added §4.3 token-type discriminator as prerequisite (refresh tokens currently accepted as bearer).
  - Removed signin-time 2FA gate; replaced with `mfa_satisfied` claim enforced at admin gate (§4.4).
  - Documented passkey-with-UV as acceptable MFA; recommended Option B (passkey-only admin login) over duplicating TOTP UX in admin SPA.
  - Added §4.6 typed auth errors with `code` field.
  - Audit middleware (§4.7) ordered to capture both authorized and denied requests, including missing/invalid-token attempts.
  - Reframed §4.2 revocation semantics as DB-authoritative, env grant-only.
  - Clarified refresh recomputes platform-admin status from DB and added WebAuthn RP/origin rollout requirements for passkey-only admin login.
  - Restructured rollout into three PRs with bounded risk per step.
- **v1 (2026-04-28):** Initial draft.
