# PRD — Cookie-Backed Session Auth (HEL-46)

**Status:** Draft
**Owner:** TBD
**Linear:** HEL-46
**Branch:** `feat/hel-46-cookie-sessions`
**Builds on:** PR #473 (cherry-picked as `7b969cc3 Fix cookie-backed session persistence`)

---

## 1. Problem

Three user-reported regressions, all rooted in the current localStorage-based JWT scheme:

1. **Frequent logouts.** Tokens live in `localStorage` and are lost on browser-storage clears, incognito sessions, or when 3rd-party storage policies evict them.
2. **Sessions are not shared across tabs.** Each tab reads its own snapshot of `localStorage`; refresh races between tabs invalidate sibling tabs because each generates its own refresh-token chain.
3. **No post-login redirect.** Deep-linking to a protected route bounces to `/login` and the user lands on the home page after sign-in instead of the original destination.

## 2. Goals

- Persist sessions across reloads, tabs, and short-term storage evictions.
- One source of truth for session state (cookies, set by the server).
- Preserve the originally requested URL across the login bounce.
- Maintain backwards compat with existing tokens during rollout (no forced logout).

## 3. Non-Goals

- Server-side session revocation lists. Refresh tokens remain stateless JWTs.
- Switching to OAuth/OIDC, sliding sessions, or device-binding.
- Cross-domain SSO (e.g. helpin.ai ↔ partner sites).

## 4. Approach

Move session tokens from `localStorage` to **httpOnly cookies** set by the server, with **SameSite=Lax** and **Secure** in non-localhost environments. Frontend issues credentialed fetches (`credentials: 'include'`); the server reads the cookie via `RequireAuth` middleware. WebSocket falls back to the cookie when no `?token=` query is supplied (legacy clients still work).

### 4.1 Cookies

| Cookie | Path | Max-Age | HttpOnly | Secure | SameSite |
|---|---|---|---|---|---|
| `helpin_access_token` | `/api` | token TTL | ✅ | conditional¹ | Lax² |
| `helpin_refresh_token` | `/api/auth/refresh` | refresh TTL | ✅ | conditional¹ | Lax² |

¹ `Secure` is set when `r.TLS != nil` OR `X-Forwarded-Proto == https` OR Host is not localhost/127.0.0.1/[::1]. Will be promoted to a config flag (`APP_SECURE_COOKIES=auto|true|false`) before merge.

² Initial implementation uses `SameSite=Lax`. We will tighten the **access** cookie to `SameSite=Strict` and keep the **refresh** cookie at `Lax` (it's only POSTed by our own app). See §6 CSRF.

### 4.2 Backend changes

- `server/internal/handler/auth_cookies.go` — `setAuthCookies`, `clearAuthCookies`, `secureCookie`, `tokenTTL`, `maxAgeSeconds` helpers.
- `server/internal/middleware/auth.go` — `RequireAuth` accepts `Authorization: Bearer …` *or* `helpin_access_token` cookie.
- `server/internal/handler/auth.go` — signin / signup / refresh / 2FA endpoints call `setAuthCookies` before writing the JSON response (response body keeps the tokens for backwards-compat during transition).
- `POST /api/auth/signout` — new endpoint that calls `clearAuthCookies`.
- `server/internal/websocket/handler.go` — falls back to `helpin_access_token` cookie if the `?token=` query is absent.
- `service/auth.go` refresh — auto-promotes `rememberMe` when an existing access token still has a remaining TTL longer than `RefreshTokenTTL`. This prevents another tab's refresh from downgrading sibling tabs to a short-lived session. **Open question:** should this only apply when the original claims had `rememberMe=true`?
- `auth_test.go` — adds `TestRequireAuth_ValidTokenCookie` covering cookie-based auth.

### 4.3 Frontend changes

- All fetch calls (`frontend/src/lib/api.ts`, every `services/*.ts`, `apps/admin/src/lib/api.ts`) add `credentials: 'include'`.
- Auth bootstrap (`stores/authStore.ts → initialize`) no longer requires a localStorage token; calls `/auth/me` directly and trusts the cookie.
- `persistAuthSession` removes access/refresh tokens from localStorage; only `remember_me` is kept (used by Profile/MFA flows to pass through preference).
- `tryRefreshToken` sends `credentials: 'include'` and an empty body when no refresh token is in localStorage.
- `request()` retry on 401 now allows `/auth/me` to attempt refresh (was blanket-skipped under `path.startsWith('/auth/')`).
- Visibility-refresh fires unconditionally (cookie may exist even with no localStorage state).

### 4.4 Post-login redirect

New file `frontend/src/lib/authRedirect.ts`:

- `storeRedirectAfterLogin(path?)` — saves the current path to `sessionStorage['helpin_redirect_after_login']` when redirecting to `/login`.
- `consumeRedirectAfterLogin()` — reads and clears it after a successful login; returns `null` if absent or external.
- `isInternalRedirect()` — guards against open-redirect (`//evil.com`, external schemes, `/login` itself).

Wired in:
- `routes/_authenticated.tsx` — calls `storeRedirectAfterLogin()` before throwing the redirect to `/login`.
- `pages/Login.tsx`, `pages/JoinWorkspace.tsx`, MFA gate — calls `consumeRedirectAfterLogin()` after successful sign-in.
- `lib/api.ts` — calls `storeRedirectAfterLogin()` when a refresh fails and forces a redirect to `/login`.

`sessionStorage` (per-tab) is used deliberately so that closing the original tab discards the pending redirect — opening the login page in a fresh tab will not hijack a prior tab's destination.

## 5. Multi-tab semantics

| Scenario | Before | After |
|---|---|---|
| Sign in tab A, open tab B | Tab B has no token, signed out | Tab B reads cookie, signed in |
| Tab A refreshes silently | Tab B sees stale token, eventually 401s | Tab B uses the same cookie, no break |
| Sign out in tab A | Other tabs keep working until next 401 | Cookie cleared globally; next request re-auths |
| Browser closes & reopens | Token still in localStorage; user re-enters | Cookie max-age determines persistence; rememberMe controls long-lived refresh cookie |

## 6. CSRF strategy *(must address before merge)*

Cookie-bearing fetches with `credentials: 'include'` introduce CSRF surface. `SameSite=Lax` mitigates most cases but leaves top-level form-POST and link-driven navigation. We will add **two** layers:

1. **Cookie-level:** access cookie → `SameSite=Strict`. The access cookie is only ever sent on first-party requests; this is the primary mitigation. Refresh cookie stays `Lax` so a sign-in flow that lands on `/login?next=…` still works.
2. **Header-level:** `Origin` (with `Referer` fallback) check on `POST/PUT/PATCH/DELETE` in a new middleware. Allowed origins are read from existing `apiCORS` config so the source of truth is shared.

A double-submit CSRF token was considered but rejected: `SameSite=Strict + Origin check` already blocks the realistic attack paths and avoids new client-side plumbing.

## 7. Rollout

The cherry-picked commit already keeps the JSON response body containing tokens for backwards-compat. Plan:

1. **Phase 1 (this PR):** Cookie write + read paths land. Frontend stops persisting tokens to localStorage but the server continues to issue them in the response. Existing sessions migrate on first request via the refresh flow.
2. **Phase 2 (follow-up):** Stop returning `access_token`/`refresh_token` in JSON bodies once frontend is fully cookie-only. Remove dead `localStorage.getItem('access_token')` reads in `api.ts` and `authService` retry paths.
3. **Phase 3 (follow-up):** Remove `?token=` WebSocket query param after legacy admin clients are confirmed migrated.

## 8. Open follow-up items (tracked in this PR)

| # | Item | Why |
|---|---|---|
| 1 | Tighten access cookie to `SameSite=Strict`, add `Origin`-check middleware | CSRF defense (§6) |
| 2 | Remove dead `localStorage.getItem('access_token')` reads from `api.ts` + admin `api.ts` | Cookie does the work; reads always return `null` after login |
| 3 | Make `secureCookie` driven by `APP_SECURE_COOKIES` config flag | Explicit > heuristic |
| 4 | Validate cookie scope for cross-subdomain deploys (e.g. `helpin.ai` ↔ `api.helpin.ai`) | Cookie is host-only; WS fallback `?token=` covers it but worth confirming |
| 5 | Tighten `rememberMe` auto-promotion to only fire when original claims had `rememberMe=true` | Avoids silent persistence upgrades |
| 6 | Tests: refresh-via-cookie, signout-clears-cookie, secureCookie matrix, post-login redirect | Coverage |

## 9. Test plan

**Backend:**
- `go test ./server/internal/middleware -run TestRequireAuth_ValidTokenCookie` (added)
- New tests for `setAuthCookies`/`clearAuthCookies` matrix (Secure on/off, MaxAge).
- New test that signin response sets both cookies and that `/auth/me` works with cookies only (no Authorization header).

**Frontend (Vitest):**
- Unit test for `authRedirect.isInternalRedirect` rejecting `//`, external URLs, and `/login` paths.
- Unit test for `consumeRedirectAfterLogin` cleanup semantics.

**Manual:**
- [ ] Sign in tab A → open tab B → expect signed-in state without re-auth.
- [ ] Hit a deep link signed-out → log in → land on the deep link.
- [ ] Sign out tab A → tab B's next request 401s and bounces to `/login`.
- [ ] Reload during background tab → still signed in.
- [ ] CSRF: cross-site form POST to `/api/workspaces/…` from a different origin → blocked (after §6 lands).
- [ ] WebSocket: connect with no `?token=` query → cookie auth succeeds.
- [ ] WebSocket: legacy `?token=` query still works.

## 10. Risks

- **Cookie domain mismatch in production** could leave WS on legacy `?token=` path silently. Mitigation: add a startup log when WS request lacks the cookie *and* falls back to query param, so we can monitor it post-deploy.
- **3rd-party iframe embedding** (widget, etc.) could break if cookies are now `SameSite=Strict`. Widget runs on a different code path and does not use these cookies — verify before merging Phase 2.
- **Existing localStorage tokens in users' browsers** are no longer read for bootstrap. They will be stranded but harmless; cleared by `persistAuthSession` on next login.

## 11. References

- Original PR: https://github.com/helpin-ai/helpin/pull/473
- Cherry-picked commit: `7b969cc3`
- Linear ticket: HEL-46
