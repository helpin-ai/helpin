# Widget messenger security implementation proposal

> Historical JWT proposal, source-compared on 2026-09-17. This page records
> a possible widget security implementation for contributors. Current widget
> identity verification uses a different HMAC-proof contract; the milestones
> below are not an installation guide or evidence of shipped JWT support.

## Current identity contract

The [verifier](../../server/internal/service/support_widget_identity.go) accepts
`identity_verification` version `v1` with issued/expiry timestamps and a hexadecimal
HMAC-SHA256 signature. It signs a canonical newline-delimited value containing
the widget key, normalized email, external user ID, company ID, and timestamps,
using the installation secret. It permits at most 15 minutes of validity and
one minute of future issued-time skew. This is not an HS256 JWT with `sub`, `aud`,
`iss`, and a token refresh callback.

The [model](../../server/internal/model/support_inbox.go) exposes per-installation
`off`, `report_only`, and `enforced` verification modes. Empty mode defaults to
report-only in the verifier. Invalid/missing proofs remain untrusted in report-only
mode and produce a generic identity-required error in enforced mode. The verifier
alone does not establish the proposed identity-event audit table or rotation UI.

The [SDK identity client](../../packages/sdk-js/src/core/client.ts) and
[widget transport](../../packages/sdk-js/src/core/widget.ts) carry the structured
proof. Source search found no current `identityToken`, `getIdentityToken`, or
`messenger_security_*` contract in the inspected backend/shared/SDK sources.
The installation model has `SecretKey`, not the proposed encrypted current/previous
secret-ID fields. Do not assume that the rotation, at-rest encryption, issuer
auditing, or unified `ApplyWidgetIdentity` milestones are implemented.

The original JWT rollout, sample payloads, callback recommendations, badge/UI
requirements, and release matrix below remain proposed work. Existing identity
and CRM matching behavior must be read from current services rather than inferred
from this checklist. No identity secrets, installation modes, external clients,
or live sessions were changed or tested during this documentation comparison.

## Original JWT implementation proposal

Status: Draft engineering plan
Last updated: 2026-05-02

## Objective

Implement signed identity verification for the Helpin support widget without breaking existing anonymous or unsigned widget installs.

The first production release should support monitor mode:

- JWT accepted and verified if present.
- Sessions/conversations record verified vs unverified identity.
- Existing installs keep working.
- Enforcement remains disabled until explicitly configured.

## Milestone 1: Schema And Types

Backend model changes:

- `support_widget_sessions.external_user_id`
- `support_widget_sessions.identity_verified`
- `support_widget_sessions.identity_verified_at`
- `support_widget_sessions.identity_source`
- `support_widget_sessions.identity_token_subject`
- `support_widget_sessions.identity_token_issuer`
- `support_widget_sessions.identity_token_audience`
- `support_conversations.external_user_id`
- `support_conversations.identity_verified`
- `support_conversations.identity_verified_at`
- `support_conversations.identity_source`

Audit table:

- `support_widget_identity_events`

Settings:

- `messenger_security_enabled`
- `messenger_security_enforced`
- `messenger_security_secret_current_id`
- `messenger_security_secret_previous_id`
- `messenger_security_secret_current_encrypted`
- `messenger_security_secret_previous_encrypted`
- `messenger_security_previous_expires_at`
- `messenger_security_max_token_ttl_seconds`

Settings live per support widget installation. Secrets must be encrypted at rest with `server/internal/crypto`.

Migration rules:

- Existing sessions default to `identity_source = 'anonymous'` or `legacy_unsigned` where customer email exists.
- Existing conversations default similarly.
- No existing row becomes verified by migration.

## Milestone 2: Verification Service

Create a focused package/service under `server/internal/widgetsecurity`:

```go
type VerifiedWidgetIdentity struct {
    Subject string
    Email string
    Name string
    FirstName string
    LastName string
    Company VerifiedWidgetCompany
    Audience string
    Issuer string
    IssuedAt time.Time
    ExpiresAt time.Time
}

type WidgetIdentityVerifier struct {
    installationRepo *repository.SupportInboxInstallationRepository
    clock func() time.Time
}
```

Verification requirements:

- Reject `alg=none`.
- Require `HS256`.
- Verify signature with widget security secret.
- Require `sub`.
- Require matching `aud`.
- `aud` is the widget key in v1.
- Require `exp`.
- Validate `iat`.
- Enforce max TTL.
- Accept optional `iss` and store/audit it when present.
- Return stable reason codes.
- Accept replay within token TTL in v1; do not build `jti` replay cache unless required later.

Tests:

- valid token
- expired token
- missing subject
- invalid audience
- invalid signature
- wrong algorithm
- TTL too long

## Milestone 3: Unified Identity Application

Add one service method used by all widget identity paths:

```go
ApplyWidgetIdentity(ctx, input ApplyWidgetIdentityInput) (*ApplyWidgetIdentityResult, error)
```

Inputs:

- workspace ID
- widget key / installation ID
- session token optional
- anonymous ID optional
- identity token optional
- legacy unsigned payload optional
- mode: monitor or enforce

Rules:

- Valid JWT wins over all unsigned fields.
- Invalid JWT in monitor mode records event and falls back to anonymous/unverified.
- Invalid JWT in enforce mode rejects trusted identity mutation.
- Unsigned email/name can still be captured as unverified lead identity in monitor mode.
- Existing verified identity cannot be overwritten by unsigned identity.
- CRM contact matching uses external ID first from the first release.
- Legacy conversations with matching token email can be shown as unverified history, but must not be marked verified by migration or email match alone.

Contact matching order:

1. `workspace_id + external_user_id`
2. `workspace_id + email`
3. create contact

Company matching order:

1. `workspace_id + company.id`
2. `workspace_id + company.domain`
3. `workspace_id + company.name`
4. create company

## Milestone 4: WebSocket Wiring

Update widget WebSocket messages:

- `session:create`
- `session:restore`
- `session:upgrade`

Payload addition:

```json
{
  "identity_token": "eyJ..."
}
```

Response addition:

```json
{
  "identity_verified": true,
  "identity_source": "jwt",
  "external_user_id": "user_123"
}
```

Test both valid and invalid token paths.

## Milestone 5: HTTP Fallback Wiring

Update `/widget/identify` to accept:

```json
{
  "widget_key": "wk_live_xxx",
  "anonymous_id": "anon_123",
  "identity_token": "eyJ..."
}
```

Keep legacy payload working in monitor mode:

```json
{
  "api_key": "wk_live_xxx",
  "anonymous_id": "anon_123",
  "email": "jane@example.com"
}
```

Mark legacy identity as unverified.

## Milestone 6: SDK Changes

Add public config fields:

```ts
type WidgetSettings = {
  widgetKey?: string;
  key?: string;
  host?: string;
  user?: WidgetUser;
  identityToken?: string;
  getIdentityToken?: () => Promise<string | null>;
};
```

Add identify shape:

```ts
helpin("identify", {
  identityToken: token,
});
```

SDK sends token through:

- WebSocket `session:create`
- WebSocket `session:restore`
- WebSocket `session:upgrade`
- HTTP identify fallback

For long-lived SPAs, `getIdentityToken` is the preferred authenticated-app integration. A one-time `identityToken` is acceptable only for simple page-load integrations.

Logout:

- `helpin("shutdown")` clears session and local verified identity state.

## Milestone 7: Inbox Indicators

Add internal identity badges:

- `Verified by app`
- `Unverified lead`
- `Anonymous visitor`

Badge should appear in:

- conversation detail sidebar
- conversation row metadata if space allows
- message thread customer context

## Milestone 8: Admin UI

Settings -> Chat Widget -> Security:

- show status
- show secret controls
- show current and previous key IDs during rotation overlap
- show integration snippets
- show token validation logs
- show enforcement toggle

Enforcement toggle requirements:

- Warn about breaking browser-only identify.
- Recommend at least one valid token observed recently.
- Confirm dialog before enabling.

## Milestone 9: E2E And Release

Test matrix:

```txt
anonymous marketing boot                  pass
legacy unsigned identify monitor mode     pass, unverified
valid JWT session create                  pass, verified
valid JWT session upgrade                 pass, verified
invalid JWT monitor mode                  pass, unverified + audit
invalid JWT enforce mode                  rejected identity
logout                                    clears session
CRM external ID match                     pass
legacy matching email history             visible as unverified history
secret rotation current + previous         both validate during overlap
```

Release plan:

1. Ship backend hidden.
2. Ship SDK support.
3. Enable monitor mode for internal workspace.
4. Enable monitor mode for early customer.
5. Add admin UI.
6. Enable enforcement only for opted-in customer.

## Suggested PR Breakdown

1. Schema and model fields.
2. JWT verifier and tests.
3. Unified identity application service.
4. WebSocket/HTTP wiring.
5. SDK token support.
6. Inbox identity indicators.
7. Admin security settings.
8. Integration docs and examples.

## Verification Commands

Backend:

```bash
cd server
GOCACHE=/tmp/go-build go test ./internal/service ./internal/repository ./internal/handler ./internal/websocket
```

SDK:

```bash
pnpm --dir packages/sdk-js test
pnpm --dir packages/widget-core test
pnpm --dir packages/sdk-js build
```

Frontend:

```bash
pnpm --dir frontend test
pnpm --dir frontend build
```
