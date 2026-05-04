# PRD: Widget Messenger Security With Signed Identity Tokens

Status: Draft
Owner: Support / Widget / SDK
Last updated: 2026-05-02

## 1. Executive Summary

Helpin should add Messenger Security for the support widget using server-generated signed identity tokens. The feature verifies that a logged-in visitor is really the user/account claimed by the customer's application before Helpin trusts that identity in support conversations, CRM records, routing, or AI workflows.

The current widget identify flow accepts browser-provided `email`, `name`, and company attributes. That is useful for lead capture, but it is not identity verification. For authenticated SaaS app installs, this leaves a spoofing gap: any browser can claim another user's email if it can call the widget SDK.

The recommended product direction is an Intercom-style JWT Messenger Security model:

- Customer backend signs a short-lived JWT using a Helpin widget secret.
- Customer frontend passes that token to the Helpin widget.
- Helpin verifies the token before trusting `user_id`, `email`, `name`, and company/account claims.
- Existing anonymous marketing-site chat continues to work.
- Existing unsigned identify flows continue in monitor mode, but are marked unverified.
- Workspaces can later enable enforcement once their secure integration is live.

Key product decisions for v1:

- `aud` is the public `widget_key`. This gives the best customer developer experience because it matches the value already used in snippets.
- Messenger Security settings are per support widget installation, not workspace-wide.
- Rotation supports two active secrets: current and previous, with a defined overlap window.
- Token replay within the token TTL is accepted in v1. Short TTL is the replay control.
- `iss` is optional but stored/audited when present. It is not required in v1.
- The verifier should live in a small backend package such as `server/internal/widgetsecurity`.
- Messenger Security secrets must be encrypted at rest using the existing `server/internal/crypto` package.

## 2. Why Now

This is becoming more urgent because Helpin is being installed inside SaaS applications, not only on marketing pages. In authenticated product surfaces, support identity is operationally sensitive:

- Support teams may act on billing, account, permissions, or data-access requests.
- CRM contact/company records are created and updated from widget identity.
- Conversation history continuity depends on identifying the right person.
- AI support workflows may use customer/account context.
- Larger SaaS customers will reasonably ask whether one app user can spoof another user's email.

For anonymous marketing traffic, unsigned lead capture is acceptable. For logged-in SaaS app traffic, verified identity should be the standard integration path.

## 3. Current State

Today, the browser can provide identity using public values:

- widget key / API key
- anonymous browser ID
- email
- name
- first name / last name
- company traits

Backend then uses this identity to:

- upgrade widget sessions
- backfill conversations by anonymous ID
- match or create CRM contacts
- match or create CRM companies
- show customer identity in the support inbox

The widget key is public by design. Therefore, browser-provided identity must not be treated as verified.

## 4. Problem

Helpin currently conflates three different concepts:

- Anonymous browser identity: durable cookie/session identity for a browser.
- Captured lead identity: visitor-provided email/name, not trusted.
- Verified application identity: authenticated app user confirmed by customer backend.

Without a signed identity proof, Helpin cannot know whether `jane@example.com` was provided by:

- Jane after logging into the customer's app.
- A random visitor typing Jane's email.
- A malicious user intentionally impersonating Jane.

This creates security and data-quality risks:

- Cross-user impersonation in chat.
- Incorrect CRM contact/company association.
- Polluted customer records.
- Support actions based on spoofed identity.
- AI workflows using untrusted user/account traits.
- Unsafe restoration of historical conversations if identity continuity is email-based.

## 5. Goals

1. Verify logged-in widget users using server-generated signed tokens.
2. Preserve anonymous marketing-site chat.
3. Keep current installs working during rollout.
4. Clearly distinguish verified and unverified identities in backend state.
5. Prefer stable external user IDs over email for trusted identity.
6. Support account/company claims for B2B SaaS installs.
7. Provide a clean customer integration path.
8. Support monitor mode before enforcement.
9. Add audit logs and diagnostics for token issues.
10. Make future AI/account workflows safer by using protected identity claims.

## 6. Non-Goals

- Requiring every visitor to log in.
- Verifying email ownership directly.
- Replacing customer authentication.
- Building SSO.
- Replacing all CRM merge logic.
- Eliminating pre-chat forms.
- Preventing account takeover if the customer's own app account is compromised.

## 7. Recommended Approach

Use short-lived JWTs signed with HS256.

This is operationally similar to HMAC because HS256 is HMAC-SHA256, but JWT is a better container for Helpin because it supports structured claims, expiry, audience, and future protected attributes.

### 7.1 Why JWT Over Bare HMAC

Crisp-style HMAC signs a single email:

```txt
signature = HMAC_SHA256(secret, email)
```

That is simple and useful for "is this email verified?".

Helpin needs more:

- stable user ID
- email
- display name
- account/company ID
- account/company name
- token expiry
- intended widget audience
- issued-at timestamp
- future protected traits

JWT gives us this shape:

```json
{
  "sub": "user_123",
  "email": "jane@example.com",
  "name": "Jane Doe",
  "company": {
    "id": "acct_456",
    "name": "Acme Inc",
    "domain": "acme.com"
  },
  "aud": "wk_live_abc",
  "iat": 1777560000,
  "exp": 1777560600
}
```

The signature still uses HMAC-SHA256, but the payload is safer and more extensible.

## 8. User Stories

### 8.1 Anonymous Marketing Visitor

As a visitor on a marketing site, I can open chat and ask a question without logging in.

Acceptance:

- Widget boots with only `widgetKey`.
- Visitor gets anonymous session.
- Pre-chat email is allowed but marked unverified.
- No verified conversation history is loaded based only on email.

### 8.2 Logged-In SaaS User

As a logged-in app user, I can open chat and Helpin support sees my verified identity.

Acceptance:

- Customer frontend fetches an identity token from customer backend.
- Widget sends token to Helpin.
- Helpin verifies signature and claims.
- Session/conversation are marked verified.
- CRM contact is matched by external user ID first.

### 8.3 Support Teammate

As a support teammate, I can distinguish verified app users from unverified leads.

Acceptance:

- Inbox shows identity verification state.
- Verified identity includes user/account context.
- Unverified pre-chat identity is visibly not trusted.

### 8.4 Customer Admin

As a customer admin, I can configure and rotate widget identity secrets.

Acceptance:

- Admin can create/rotate secret.
- Admin can view integration snippets.
- Admin can see recent token errors.
- Admin can enable enforcement after successful monitor-mode traffic.

## 9. Terminology

| Term | Meaning |
| --- | --- |
| Anonymous ID | Browser-scoped durable ID stored by the SDK. Not proof of app user identity. |
| Session token | Helpin-issued credential for widget session API/WS calls. |
| Captured identity | Visitor-provided email/name from pre-chat or unsigned identify. |
| Verified identity | User/account identity verified from a valid signed token. |
| Identity token | Customer backend-generated JWT sent to Helpin widget. |
| Monitor mode | Validate tokens if present, but do not reject legacy unsigned identity. |
| Enforce mode | Reject trusted identity mutation unless a valid token is present. |

## 10. Product Behavior

### 10.1 Anonymous Visitor

If no identity token is provided:

- Widget boots normally.
- Session is created as anonymous.
- Conversation source remains widget.
- Email collected through pre-chat is saved as unverified lead identity.
- CRM lead creation remains allowed, but the source is unverified.
- Verified user history is not loaded by email alone.

### 10.2 Verified Logged-In User

If a valid token is provided:

- Session becomes `identity_verified = true`.
- `external_user_id` comes from `sub`.
- Email/name/company claims come from token.
- Browser-provided identity fields do not override token claims.
- CRM contact match order is `external_id` first, then email fallback.
- Conversation history can be keyed by verified external user ID.

### 10.3 Invalid Token

Examples:

- expired token
- wrong `aud`
- missing `sub`
- invalid signature
- unsupported `alg`
- token TTL too long

Monitor mode:

- Record audit event.
- Keep visitor anonymous or legacy-unverified.
- Do not mark identity verified.

Enforce mode:

- Reject identity upgrade.
- Continue anonymous chat if possible.
- Return non-sensitive error code to SDK.

### 10.4 Logout

Customer app logout must clear Helpin widget state:

- revoke current widget session
- clear local session token
- clear cached verified identity
- return widget to anonymous state

### 10.5 Conversation History During Cutover

No existing conversation becomes verified automatically during migration.

When a user later presents a valid identity token:

- new conversations are verified and keyed by `external_user_id`
- existing verified conversations for the same `external_user_id` are safe to restore
- legacy conversations with the same anonymous ID may be shown because they belong to the same browser
- legacy conversations with matching email may be shown only as unverified history when token email matches the legacy email
- matching legacy email must not mark those conversations verified
- a support-side banner or metadata should distinguish "verified current session" from "legacy unverified history"

This gives continuity during rollout without retroactively upgrading old records to verified.

## 11. Flow Diagrams

### 11.1 Marketing Site Anonymous Flow

```txt
Visitor opens marketing page
        |
        v
Widget boots with widgetKey only
        |
        v
Helpin creates anonymous session
        |
        v
Visitor sends message
        |
        v
Helpin creates conversation as anonymous widget lead
        |
        v
Support inbox shows unverified visitor
```

### 11.2 Logged-In App Flow

```txt
User opens authenticated app page
        |
        v
Frontend requests /api/helpin/widget-token
        |
        v
Customer backend signs JWT using Helpin widget secret
        |
        v
Frontend boots widget with identityToken
        |
        v
Helpin verifies JWT
        |
        +-- valid ------> verified widget session
        |
        +-- invalid ----> anonymous/unverified session
```

### 11.3 Identity State Machine

```txt
             +----------------+
             |   anonymous    |
             +----------------+
                    |
                    | pre-chat email or unsigned identify
                    v
             +----------------+
             | unverified_lead|
             +----------------+
                    |
                    | valid identity JWT
                    v
             +----------------+
             | verified_user  |
             +----------------+
                    |
                    | logout / revoke
                    v
             +----------------+
             |   anonymous    |
             +----------------+

Invalid JWT in monitor mode: stay anonymous/unverified.
Invalid JWT in enforce mode: reject identity upgrade.
```

### 11.4 Token Validation Flow

```txt
Receive identity_token
        |
        v
Parse JWT header and claims
        |
        v
Is alg exactly HS256?
        |
        +-- no --> reject unsupported_alg
        |
        v
Find widget security secret by aud/widget_key
        |
        v
Verify signature
        |
        +-- fail --> reject invalid_signature
        |
        v
Validate exp, iat, max TTL
        |
        +-- fail --> reject expired_or_invalid_time
        |
        v
Validate aud and required sub
        |
        +-- fail --> reject invalid_claims
        |
        v
Accept verified identity
```

## 12. Sequence Diagrams

### 12.1 Anonymous Marketing Visitor

```txt
Browser              Marketing Site          Helpin Widget API       Helpin Inbox
  |                        |                         |                    |
  | GET /                  |                         |                    |
  |----------------------->|                         |                    |
  | HTML + widgetKey       |                         |                    |
  |<-----------------------|                         |                    |
  |                        |                         |                    |
  | boot(widgetKey)        |                         |                    |
  |------------------------------------------------->|                    |
  |                        |                         | create anonymous   |
  |                        |                         | session            |
  | session_token          |                         |                    |
  |<-------------------------------------------------|                    |
  |                        |                         |                    |
  | send message           |                         |                    |
  |------------------------------------------------->|                    |
  |                        |                         | create conversation|
  |                        |                         | and message        |
  |                        |                         |------------------->|
  |                        |                         |                    | unverified visitor
```

### 12.2 Verified Logged-In SaaS User

```txt
Browser             Customer Backend        Helpin Widget API        Helpin CRM/Inbox
  |                        |                         |                    |
  | GET /app               |                         |                    |
  |----------------------->|                         |                    |
  | app shell              |                         |                    |
  |<-----------------------|                         |                    |
  |                        |                         |                    |
  | GET /helpin-token      |                         |                    |
  |----------------------->|                         |                    |
  |                        | sign JWT with secret    |                    |
  | identityToken          |                         |                    |
  |<-----------------------|                         |                    |
  |                        |                         |                    |
  | boot(widgetKey, token) |                         |                    |
  |------------------------------------------------->|                    |
  |                        |                         | verify signature   |
  |                        |                         | verify exp/aud/sub |
  |                        |                         | mark session       |
  |                        |                         | verified           |
  | verified session       |                         |                    |
  |<-------------------------------------------------|                    |
  |                        |                         |                    |
  | send message           |                         |                    |
  |------------------------------------------------->|                    |
  |                        |                         | create/update CRM  |
  |                        |                         | by external user ID|
  |                        |                         |------------------->|
  |                        |                         |                    | verified user
```

### 12.3 Invalid Token In Enforce Mode

```txt
Browser             Customer Backend        Helpin Widget API
  |                        |                         |
  | request token          |                         |
  |----------------------->|                         |
  | bad/expired token      |                         |
  |<-----------------------|                         |
  |                        |                         |
  | boot(widgetKey, token) |                         |
  |------------------------------------------------->|
  |                        |                         | verify token
  |                        |                         | fail
  | identity rejected      |                         |
  |<-------------------------------------------------|
  |                        |                         |
  | widget remains anonymous or shows safe error     |
```

### 12.4 Logout

```txt
Browser              Customer App           Helpin Widget API
  |                        |                         |
  | click logout           |                         |
  |----------------------->|                         |
  | app clears auth        |                         |
  |<-----------------------|                         |
  |                        |                         |
  | helpin.shutdown()      |                         |
  |------------------------------------------------->|
  |                        |                         | revoke session
  | clear local state      |                         |
  |<-------------------------------------------------|
```

## 13. Customer Integration

### 13.1 Anonymous Marketing Page

No identity token is needed.

```html
<script>
  window.helpinSettings = {
    widgetKey: "wk_live_xxx"
  };
</script>
<script async src="https://cdn.helpin.ai/sdk.js"></script>
```

### 13.2 Logged-In App Page

Frontend:

```ts
const response = await fetch("/api/helpin/widget-token", {
  credentials: "include",
});

const { token } = await response.json();

window.helpin("boot", {
  widgetKey: "wk_live_xxx",
  identityToken: token,
});
```

Backend:

```ts
import jwt from "jsonwebtoken";

app.get("/api/helpin/widget-token", requireAuth, (req, res) => {
  const token = jwt.sign(
    {
      sub: req.user.id,
      email: req.user.email,
      name: req.user.name,
      company: {
        id: req.user.account.id,
        name: req.user.account.name,
        domain: req.user.account.domain,
      },
      aud: "wk_live_xxx",
    },
    process.env.HELPIN_WIDGET_SECRET,
    {
      algorithm: "HS256",
      expiresIn: "10m",
    },
  );

  res.json({ token });
});
```

## 14. API Contract

### 14.1 Widget Boot / Session Create

Request:

```json
{
  "widget_key": "wk_live_xxx",
  "anonymous_id": "anon_123",
  "identity_token": "eyJ..."
}
```

Response:

```json
{
  "session_token": "wst_...",
  "expires_at": "2026-06-01T00:00:00Z",
  "identity_verified": true,
  "identity_source": "jwt",
  "external_user_id": "user_123"
}
```

### 14.2 Session Upgrade

Request:

```json
{
  "session_token": "wst_...",
  "identity_token": "eyJ..."
}
```

Legacy request fields remain accepted in monitor mode:

```json
{
  "session_token": "wst_...",
  "email": "jane@example.com",
  "name": "Jane Doe"
}
```

Legacy fields create unverified identity unless enforcement is disabled and the workspace intentionally allows legacy behavior.

### 14.3 Identify Fallback

HTTP fallback should accept:

```json
{
  "widget_key": "wk_live_xxx",
  "anonymous_id": "anon_123",
  "identity_token": "eyJ..."
}
```

Deprecate public `api_key + anonymous_id + email` as trusted identity.

## 15. JWT Claims

Required:

| Claim | Type | Required | Notes |
| --- | --- | --- | --- |
| `sub` | string | yes | Stable customer user ID. |
| `aud` | string | yes | Widget key in v1. |
| `iat` | number | yes | Unix timestamp. |
| `exp` | number | yes | Unix timestamp. |

Recommended:

| Claim | Type | Notes |
| --- | --- | --- |
| `email` | string | Customer user email. |
| `name` | string | Display name. |
| `first_name` | string | Preferred over splitting `name`. |
| `last_name` | string | Preferred over splitting `name`. |
| `company.id` | string | Stable account/company ID. |
| `company.name` | string | Display name for company. |
| `company.domain` | string | Optional matching aid. |
| `iss` | string | Optional issuer. Stored/audited when present; not required for v1 authorization. |

Validation rules:

- `alg` must be `HS256`.
- `sub` must be non-empty.
- `aud` must match the widget key in v1.
- `exp` must be in the future.
- `iat` must not be too far in the future.
- max token TTL should be configurable, default 15 minutes.
- `iss` is optional in v1. If present, store it in audit/session metadata. Do not require or authorize by it yet.
- raw JWT must never be logged or stored.

### 15.1 Replay Handling

V1 accepts token replay within the token TTL. This matches common messenger-security products and keeps the customer integration simple.

Replay controls:

- short token TTL, default 10 minutes and maximum 15 minutes
- `aud` binding to widget key
- session revoke on logout
- audit events for repeated failures or unusual token use

Do not add `jti` replay cache in v1 unless an enterprise customer explicitly requires one. If added later, it should be optional per installation because it introduces distributed cache requirements.

## 16. Data Model

Messenger Security configuration lives on the support widget installation, because different widgets can have different install surfaces and enforcement maturity.

Secrets must be encrypted at rest. Use the existing `server/internal/crypto` AES-GCM helpers rather than storing plaintext secrets.

### 16.1 `support_widget_sessions`

Add:

- `external_user_id TEXT`
- `identity_verified BOOLEAN NOT NULL DEFAULT false`
- `identity_verified_at TIMESTAMPTZ`
- `identity_source TEXT NOT NULL DEFAULT 'anonymous'`
- `identity_token_subject TEXT`
- `identity_token_issuer TEXT`
- `identity_token_audience TEXT`

Valid `identity_source` values:

- `anonymous`
- `prechat_unverified`
- `legacy_unsigned`
- `jwt`

### 16.2 `support_conversations`

Add:

- `external_user_id TEXT`
- `identity_verified BOOLEAN NOT NULL DEFAULT false`
- `identity_verified_at TIMESTAMPTZ`
- `identity_source TEXT`

### 16.3 `crm_contacts`

Use or add:

- `external_id TEXT`
- index on `(workspace_id, external_id)`

Matching order:

1. `workspace_id + external_id`
2. `workspace_id + email`
3. create contact

### 16.4 `crm_companies`

Use or add:

- `external_id TEXT`
- index on `(workspace_id, external_id)`

Matching order:

1. `workspace_id + company.external_id`
2. `workspace_id + domain`
3. `workspace_id + name`
4. create company

### 16.5 `support_widget_identity_events`

Audit table:

- `id`
- `workspace_id`
- `widget_installation_id`
- `anonymous_id`
- `session_id`
- `conversation_id`
- `event_type`
- `status`
- `reason_code`
- `subject`
- `audience`
- `issuer`
- `expires_at`
- `origin`
- `user_agent`
- `created_at`

Do not store:

- raw JWT
- signing secret
- full request headers

### 16.6 `support_widget_installations`

Add or embed in settings:

- `messenger_security_enabled BOOLEAN`
- `messenger_security_enforced BOOLEAN`
- `messenger_security_secret_current_encrypted TEXT`
- `messenger_security_secret_previous_encrypted TEXT`
- `messenger_security_secret_current_id TEXT`
- `messenger_security_secret_previous_id TEXT`
- `messenger_security_previous_expires_at TIMESTAMPTZ`
- `messenger_security_max_token_ttl_seconds INT DEFAULT 900`

If this is stored inside the existing installation settings JSON, it should still be accessed through typed helpers so verification code does not parse raw JSON ad hoc.

## 17. Backend Design

### 17.1 Verification Service

Create a service such as:

```go
type WidgetIdentityVerifier interface {
    Verify(ctx context.Context, widgetKey string, token string) (*VerifiedWidgetIdentity, error)
}
```

Responsibilities:

- parse JWT safely
- enforce algorithm allowlist
- load active secret
- verify signature
- validate claims
- normalize identity payload
- return typed verified identity

Package placement:

- Preferred: `server/internal/widgetsecurity`
- Acceptable: `server/internal/auth/widgetsecurity`
- Avoid mixing this into generic JWT auth, because widget identity tokens are not Helpin user access tokens.

### 17.2 Identity Application Service

Create a single identity application path used by:

- WebSocket `session:create`
- WebSocket `session:restore`
- WebSocket `session:upgrade`
- HTTP `/widget/identify`
- legacy pre-chat upgrade

This prevents drift between WebSocket and HTTP fallback behavior.

CRM matching must be implemented inside this unified identity application path, not as a later separate pass. Verified sessions and CRM records should start with the same external-ID precedence from the first release.

### 17.3 Error Codes

Use stable reason codes:

- `missing_token`
- `malformed_token`
- `unsupported_alg`
- `invalid_signature`
- `expired_token`
- `invalid_audience`
- `missing_subject`
- `token_ttl_too_long`
- `identity_mismatch`
- `secret_not_configured`

## 18. SDK Design

### 18.1 Public API

Boot:

```ts
helpin("boot", {
  widgetKey: "wk_live_xxx",
  identityToken: token,
});
```

Identify/update:

```ts
helpin("identify", {
  identityToken: token,
});
```

Logout:

```ts
helpin("shutdown");
```

### 18.2 Token Refresh

Token refresh should be customer-controlled, but long-lived SPAs need a first-class SDK pattern.

- SDK accepts a fresh token whenever customer app obtains one.
- SDK should not generate tokens.
- SDK should not persist tokens beyond current runtime unless needed.
- If session restore needs identity and token is unavailable, restore anonymous state.

Recommended API for authenticated app installs:

```ts
helpin("configure", {
  getIdentityToken: async () => {
    const res = await fetch("/api/helpin/widget-token", { credentials: "include" });
    return (await res.json()).token;
  },
});
```

For authenticated app installs, `getIdentityToken` should be treated as the preferred integration, not a nice-to-have. Passing a one-time `identityToken` at boot is acceptable for simple page loads, but long-lived SPAs should provide `getIdentityToken` so the SDK can refresh after token expiry without requiring a full widget reboot.

## 19. Admin UI

Settings area: Chat Widget -> Security.

Sections:

1. Status
   - Not configured
   - Monitoring
   - Secured
   - Enforced
2. Secret
   - reveal once / copy
   - rotate
   - active key ID
   - previous key ID and expiry during rotation overlap
3. Integration snippets
   - Node
   - Go
   - Ruby
   - Python
4. Install logs
   - recent valid tokens
   - invalid token reason codes
   - missing token count
5. Enforcement
   - enable only after recent successful valid tokens
   - warning about breaking legacy identify

## 20. Inbox UI

Show identity trust state:

```txt
Jane Doe
jane@example.com
Verified by app
Account: Acme Inc
```

Unverified:

```txt
Jane Doe
jane@example.com
Unverified lead
```

Avoid alarming customer-facing copy. This is primarily an internal support/team signal.

## 21. Rollout Plan

### Phase 0: Design And Schema

- Add PRD and integration guide.
- Add schema fields.
- No behavior change.

### Phase 1: Monitor Mode

- SDK accepts `identityToken`.
- Backend verifies token if present.
- Store verification state.
- Legacy unsigned identify still works.
- Audit events record valid/invalid/missing tokens.
- CRM matching for valid JWT uses external ID first from this phase onward.

### Phase 2: Verified Identity Semantics

- Token claims become source of truth when valid.
- Verified conversation history uses `external_user_id`.
- CRM matching prefers external ID.
- Inbox shows verified/unverified state.

### Phase 3: Admin Controls

- Add secret rotation.
- Add token debugger.
- Add integration snippets.
- Add install logs.

### Phase 4: Optional Enforcement

- Workspace/widget can enforce signed identity.
- Unsigned identify cannot mutate trusted identity.
- Invalid token rejects identity upgrade.
- Anonymous chat remains possible unless customer disables anonymous widget access.

### Phase 5: Default Secure For New App Installs

- For new SaaS app installs, recommend JWT path by default.
- Keep marketing-only installs anonymous by default.

## 22. Breaking Changes And Impact

### 22.1 No Immediate Breaking Change

Monitor mode is additive:

- Existing widget installs keep working.
- Existing pre-chat keeps working.
- Existing frontend-only identify keeps working as unverified/legacy identity.

### 22.2 Breaking When Enforcement Is Enabled

The following become true for enforced widgets:

- `helpin.identify({ email })` from browser is not accepted as verified identity.
- `/widget/identify` without token cannot set trusted identity.
- Logged-in app installs must provide a backend-signed token.
- Conversation history for verified users requires valid `sub`.
- CRM matching may shift from email-first to external-ID-first.
- Logout must clear widget session to avoid shared-browser leakage.

### 22.3 Customer Work Required

Customers installing inside authenticated apps must:

1. Store Helpin widget secret on backend.
2. Add token endpoint.
3. Sign token for current app user.
4. Pass token to Helpin frontend SDK.
5. Call shutdown on logout.

Marketing-only pages need no additional work.

## 23. Security Considerations

- Secret must never be exposed client-side.
- Messenger Security secrets must be encrypted at rest with `server/internal/crypto`.
- JWT must be short-lived.
- Helpin must reject `alg=none`.
- Helpin must enforce expected algorithm.
- Helpin must validate audience.
- Helpin must not log raw token.
- Verified identity should be immutable from unsigned browser fields.
- Shared browser logout must clear session.
- Customer support impersonation tools should disable or reset widget identity when admins view end-user accounts.
- Customer token endpoints must require same-site app authentication and CSRF protection appropriate to the customer's auth model.
- Token replay within TTL is accepted in v1 and documented as a known limit.

## 24. Monitoring And Metrics

Track:

- token verification success count
- invalid token count by reason
- missing token count on authenticated install surfaces if detectable
- enforced rejection count
- verified vs unverified conversations
- CRM contacts created from verified identity
- CRM contacts created from unverified pre-chat
- customer workspaces with enforcement enabled

Alerts:

- spike in invalid signatures
- spike in expired tokens
- high enforced rejection rate after enabling enforcement

## 25. Testing Plan

Backend unit tests:

- valid token
- invalid signature
- expired token
- future `iat`
- missing `sub`
- wrong `aud`
- unsupported `alg`
- TTL too long
- token wins over unsigned email
- external ID matching before email

Backend integration tests:

- WebSocket session create with valid token
- WebSocket session upgrade with valid token
- HTTP identify fallback with valid token
- monitor mode with invalid token
- enforce mode with invalid token
- anonymous chat without token

SDK tests:

- boot sends identity token
- identify sends identity token
- restore includes token when provided
- logout clears verified identity state
- token is not persisted unnecessarily

Browser/e2e:

- anonymous marketing widget still works
- verified app widget shows verified state in support inbox
- invalid token remains unverified

## 26. Implementation Plan

1. Add schema fields and migrations.
2. Add token verification service.
3. Add identity audit event table.
4. Implement unified identity application with CRM external-ID precedence.
5. Extend widget session create/restore/upgrade payloads.
6. Extend HTTP identify payload.
7. Update SDK boot/identify APIs.
8. Add support inbox verified/unverified indicators.
9. Add monitor-mode admin status.
10. Add secret rotation and enforcement controls.
11. Add docs and examples.

## 27. Open Questions

- Should pre-chat email be stored in separate unverified fields?
- Should verified conversation history be keyed only by `external_user_id`, or by `(external_user_id, company_id)` for B2B accounts?
- Should token generation snippets use `sub=email` when customers do not have stable user IDs, or should we require stable IDs?

## 28. References

- Intercom Messenger JWT security docs: https://www.intercom.com/help/en/articles/10589769-authenticating-users-in-the-messenger-with-json-web-tokens-jwts
- Intercom migration from identity verification to JWTs: https://www.intercom.com/help/en/articles/10807823-migrating-from-identity-verification-to-messenger-security-with-jwts
- Intercom deprecated HMAC identity verification: https://www.intercom.com/help/en/articles/7946878-what-is-identity-verification-deprecated
- Crisp identity verification docs: https://docs.crisp.chat/guides/chatbox-sdks/web-sdk/identity-verification/
