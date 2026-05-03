# PRD - Platform Admin Operations Console

**Status:** Draft v1  
**Author:** Helpin product/engineering  
**Date:** 2026-04-29  
**Related:** `docs/PRD-platform-admin-access.md`  
**Target app:** `apps/admin` mounted at `/admin`

---

## 1. Summary

Helpin needs a first-class platform admin operations console for internal staff to investigate tenants, users, integrations, and system health without direct database access or ad hoc scripts.

The current admin app covers three useful but isolated tools:

- Chat Playground
- Webhook Events
- Email Queue

This PRD expands the admin panel into an operational entry point with four priority areas:

1. Workspace / tenant overview
2. User and access management
3. Integration diagnostics
4. System health dashboard

The goal is not to recreate workspace settings. The admin console should expose cross-tenant operational visibility, safe support actions, and debugging context that only Helpin platform admins should have.

---

## 2. Problem

Today, platform investigations require engineers or operators to jump between:

- Neon/PostgreSQL queries
- application logs
- Temporal UI or worker logs
- Postmark dashboards
- GitHub App installation state
- workspace-scoped product screens
- existing admin tools for webhooks, email queue, and support AI preview

This creates several issues:

- Slow incident response because there is no single starting point for a tenant or user.
- Higher risk of mistakes from direct database access.
- Incomplete auditability for manual support/debug actions.
- No consolidated view of health signals across API, workers, integrations, and queues.
- Harder onboarding for non-engineering operators.

---

## 3. Goals

1. Give platform admins a searchable tenant directory with owner, status, modules, usage, and quick debug links.
2. Give platform admins a searchable user directory with memberships, access state, admin/MFA/passkey state, and safe access actions.
3. Give platform admins integration diagnostics for GitHub, Gmail, Postmark, S3/MinIO, Temporal, LLM providers, Exa/search, and webhooks.
4. Give platform admins a system health dashboard based on latency, traffic, errors, and saturation.
5. Reduce direct database access for common investigations.
6. Ensure every admin action is permission-gated and audit-logged.
7. Keep tenant data exposure deliberate, minimal, and clearly marked.

---

## 4. Non-goals

- Customer-facing workspace administration.
- Replacing observability backends such as logs, metrics, tracing, or Temporal UI.
- Building a full billing system.
- Building fine-grained platform admin roles in v1.
- Editing every workspace setting from the platform admin panel.
- Exposing secrets, OAuth tokens, raw credentials, private keys, or full email bodies by default.
- Supporting destructive tenant deletion or user deletion in v1.

---

## 5. Users

### Primary users

- Helpin founders and engineering operators
- Support engineers
- Customer success staff with platform-admin access

### Secondary users

- Security/compliance reviewers
- On-call engineers

---

## 6. Existing State

### Admin frontend

The admin SPA lives in `apps/admin` and is mounted under `/admin`.

Current routes:

- `/admin/chat-playground`
- `/admin/webhook-events`
- `/admin/email-queue`

### Admin backend

Current admin-specific API routes:

- `GET /api/admin/webhook-events`
- `GET /api/admin/webhook-events/{id}`
- `GET /api/admin/email-queue`

The backend platform-admin access model is covered separately by `docs/PRD-platform-admin-access.md`.

---

## 7. Product Scope

### 7.1 Workspace / Tenant Overview

### Purpose

This is the starting point for most investigations. A platform admin should be able to find a workspace and understand its owner, status, modules, usage, and recent operational state in under 30 seconds.

### Route

- `/admin/workspaces`
- `/admin/workspaces/:workspaceId`

### List view requirements

The list view must support:

- Search by workspace name, slug, owner email, organization name, and workspace ID.
- Filters:
  - status: active, suspended, disabled, deleted/archived if applicable
  - plan: free, trial, paid, enterprise, unknown
  - module enabled: PM, CRM, Support, Docs, Automation
  - created date range
  - has integration errors
  - has active support incidents or stuck jobs
- Sort by:
  - created date
  - updated date
  - usage
  - active users
  - recent error count

Each row should show:

- workspace name
- slug
- workspace ID, copyable
- organization
- owner name/email
- plan/status
- created date
- active member count
- enabled modules
- high-level usage summary
- health badges:
  - webhook errors
  - email queue pending
  - failed jobs
  - integration errors

### Detail view requirements

The detail view must show:

- Identity:
  - workspace ID
  - name
  - slug
  - organization
  - owner
  - created/updated timestamps
  - timezone
- Status:
  - plan/status
  - active/disabled/suspended state
  - module access status
- Usage:
  - members
  - teams
  - support conversations
  - docs/articles/spaces
  - CRM contacts/companies/deals
  - PM tasks/epics/objectives
  - agent runs
  - email volume
  - storage usage if available
  - token usage if available
- Recent activity:
  - latest sign-ins
  - latest support conversations
  - latest failed jobs
  - latest webhook failures
  - latest agent runs
- Quick links:
  - support conversations for workspace
  - webhook events filtered by workspace
  - email queue filtered by workspace
  - agent runs filtered by workspace
  - integrations filtered by workspace
  - main app workspace URL

### Safe actions

V1 actions:

- Copy workspace ID.
- Open workspace in main app.
- Refresh workspace summary.
- Retry safe background diagnostics where available.

Explicitly out of v1:

- Delete workspace.
- Transfer ownership.
- Change billing plan.
- Edit workspace settings.
- Purge workspace data.

---

### 7.2 User and Access Management

### Purpose

Platform admins need to find a user by email, understand which workspaces they can access, whether they are a platform admin, and whether their current security posture is acceptable.

### Route

- `/admin/users`
- `/admin/users/:userId`

### List view requirements

The list view must support:

- Search by email, name, user ID.
- Filters:
  - platform admin: yes/no
  - MFA enabled: yes/no
  - passkey registered: yes/no
  - disabled/locked if available
  - created date range
  - last seen range if available
- Sort by:
  - created date
  - last sign-in
  - email
  - workspace count

Each row should show:

- full name
- email
- user ID, copyable
- platform admin badge
- MFA/passkey status
- default workspace
- workspace membership count
- created date
- last sign-in or last seen if available

### Detail view requirements

The detail view must show:

- Profile:
  - user ID
  - email
  - full name
  - avatar
  - created/updated timestamps
  - default workspace
- Security:
  - platform admin flag
  - MFA enabled
  - passkey count
  - password auth enabled if available
  - last password change if available
  - active sessions if session tracking exists
  - recent login attempts if available
- Memberships:
  - organization memberships
  - workspace memberships
  - role per workspace
  - team memberships per workspace where practical
- Recent activity:
  - recent sign-ins
  - recent admin actions performed by this user
  - recent agent runs or comments if useful and available

### Safe actions

V1 actions:

- Copy user ID.
- Open related workspace.
- Revoke refresh sessions if token/session storage supports it.
- Disable user login if a user disabled flag exists or is added.
- Grant/revoke platform admin only if an explicit confirmation and audit trail are implemented.

If supporting platform-admin grant/revoke in v1, require:

- confirmation text
- reason field
- audit event
- actor cannot revoke their own platform-admin access
- actor cannot disable their own login

Explicitly out of v1:

- Delete user.
- Edit user email.
- Reset password from admin.
- Impersonation.

### Security requirement

No user management action may rely only on frontend gating. All actions must be enforced by `/api/admin/*` middleware and service-level checks.

---

### 7.3 Integration Diagnostics

### Purpose

Platform admins need one place to inspect whether a workspace's external integrations are connected, healthy, failing, rate-limited, or misconfigured.

### Route

- `/admin/integrations`
- `/admin/integrations/:provider`
- `/admin/workspaces/:workspaceId/integrations`

### Providers in scope

V1 should include diagnostic coverage for:

- GitHub
- Gmail
- Postmark
- S3/MinIO
- Temporal
- LLM providers: Claude/OpenAI-compatible providers
- Exa/search
- Webhooks

### List view requirements

The integration overview must support:

- Filter by provider.
- Filter by workspace.
- Filter by status:
  - healthy
  - degraded
  - failing
  - disconnected
  - rate-limited
  - unknown
- Search by workspace name, account email, installation ID, repository, webhook message ID where applicable.

Each integration row/card should show:

- provider
- workspace
- connected account or installation label
- status
- last successful sync/call
- last failure
- failure count in recent window
- rate-limit state if available
- quick links to provider-specific detail

### Provider-specific requirements

#### GitHub

Show:

- GitHub App installation ID
- organization/account login
- repositories linked to Helpin
- active/deleted integration state
- webhook delivery status if tracked
- last sync/import time
- last error
- repository claim conflicts

Safe actions:

- refresh installation metadata
- retry failed webhook processing where event is stored

#### Gmail

Show:

- connected account email
- token status: healthy, expired, refresh failed, revoked, unknown
- last sync time
- last message sync status
- last error
- sync workflow status if Temporal-backed

Safe actions:

- trigger account sync
- mark connection for reconnect if needed

Sensitive data rule:

- Never display OAuth access tokens or refresh tokens.

#### Postmark

Show:

- outbound delivery health
- inbound webhook health
- recent webhook event counts by type
- bounces and spam complaints
- email fallback queue impact
- last webhook processing error

Safe actions:

- replay stored webhook event
- resend safe outbound email only if idempotency and audit are implemented

#### S3/MinIO

Show:

- storage mode: AWS S3 or MinIO
- bucket configured
- public URL support
- last successful health check
- last failure
- upload/download presign health

Sensitive data rule:

- Never display access keys, secret keys, presigned URLs, or object bodies.

#### Temporal

Show:

- worker connectivity
- namespace
- task queue health if available
- workflow counts:
  - running
  - failed
  - timed out
  - stuck/old
- recent workflow failures

Safe actions:

- link to Temporal UI if configured
- retry workflow only where service code exposes safe retry semantics

#### LLM providers

Show:

- configured provider by workspace or feature
- model
- last successful call
- last error
- latency
- token usage if available
- rate-limit or quota errors

Sensitive data rule:

- Never display API keys, prompts containing customer secrets, or full model payloads by default.

#### Exa/Search

Show:

- enabled/disabled status
- last successful request
- last error
- latency
- rate-limit/quota state if available

#### Webhooks

Build on the existing Webhook Events page:

- filter by workspace
- filter by provider
- filter by event type
- filter by status/processing outcome if available
- show replay eligibility
- show linked conversation/email log

---

### 7.4 System Health Dashboard

### Purpose

The system health dashboard should answer: "Is Helpin healthy right now, and where is it failing?"

It should use the SRE golden signals:

- latency
- traffic
- errors
- saturation

### Route

- `/admin/health`

### Dashboard sections

#### API health

Show:

- current status: healthy, degraded, down, unknown
- request rate
- p50/p95/p99 latency
- 4xx and 5xx error rates
- slow route list
- top failing routes
- build/version if available
- uptime if available

#### Database health

Show:

- connectivity
- query error rate if available
- slow query signal if available
- connection pool saturation:
  - open connections
  - in-use connections
  - wait count/duration
- migration head/status if cheap and safe to expose

#### Worker health

Show:

- Temporal worker status
- background worker status
- job queue depth
- failed job counts
- oldest pending job
- retry volume

#### WebSocket health

Show:

- active connections if available
- active workspaces connected
- Redis relay status if enabled
- publish/broadcast errors
- recent reconnect/error signal

#### Email provider health

Show:

- Postmark API status from internal health check if available
- inbound webhook processing rate
- outbound success/failure counts
- pending fallback queue count
- overdue email queue count

#### Integration health summary

Show aggregate health for:

- GitHub
- Gmail
- S3/MinIO
- Temporal
- LLM providers
- Exa/search
- Webhooks

### Alerting

V1 does not need to page humans directly, but the dashboard must make unhealthy states visible with:

- red/yellow/green status
- recent error count
- oldest stuck timestamp
- link to affected detail page

Future versions may connect these statuses to Slack, PagerDuty, or notification channels.

---

## 8. Information Architecture

Recommended sidebar after this PRD:

- Overview
- Workspaces
- Users
- Health
- Integrations
- Support Tools
  - Chat Playground
  - Webhook Events
  - Email Queue

Alternative: keep existing tools top-level until there are enough grouped support tools.

Default route:

- `/admin` redirects to `/admin/overview`

Overview page should summarize:

- total workspaces
- active workspaces
- platform health
- pending email queue
- failing integrations
- recent webhook failures
- recent admin audit events
- quick search for workspace/user

---

## 9. Data and API Requirements

All new endpoints live under `/api/admin/*` and require:

- authenticated access token
- platform admin claim
- MFA/passkey-satisfied token
- admin audit logging

### Proposed endpoints

#### Workspaces

- `GET /api/admin/workspaces`
- `GET /api/admin/workspaces/{workspaceId}`
- `GET /api/admin/workspaces/{workspaceId}/usage`
- `GET /api/admin/workspaces/{workspaceId}/activity`
- `GET /api/admin/workspaces/{workspaceId}/integrations`

#### Users

- `GET /api/admin/users`
- `GET /api/admin/users/{userId}`
- `GET /api/admin/users/{userId}/memberships`
- `GET /api/admin/users/{userId}/security`
- `POST /api/admin/users/{userId}/revoke-sessions`
- `POST /api/admin/users/{userId}/disable`
- `POST /api/admin/users/{userId}/enable`
- `POST /api/admin/users/{userId}/platform-admin/grant`
- `POST /api/admin/users/{userId}/platform-admin/revoke`

Actions should be implemented only when the backend has the underlying data model to enforce them safely.

#### Integrations

- `GET /api/admin/integrations`
- `GET /api/admin/integrations/summary`
- `GET /api/admin/integrations/{provider}`
- `GET /api/admin/integrations/{provider}/{id}`
- `POST /api/admin/integrations/{provider}/{id}/retry`
- `POST /api/admin/webhook-events/{id}/replay`

#### Health

- `GET /api/admin/health`
- `GET /api/admin/health/api`
- `GET /api/admin/health/database`
- `GET /api/admin/health/workers`
- `GET /api/admin/health/websocket`
- `GET /api/admin/health/email`

### Query conventions

List endpoints must support:

- `page`
- `per_page`
- `q`
- `sort`
- `direction`
- relevant typed filters

Response shape:

```json
{
  "data": [],
  "page": 1,
  "per_page": 25,
  "total": 100,
  "total_pages": 4
}
```

### Error conventions

Admin APIs must return structured errors:

```json
{
  "error": "human readable message",
  "code": "stable_error_code"
}
```

---

## 10. Data Model Considerations

The first implementation should avoid unnecessary new tables. Prefer read-only aggregation from existing tables and service health checks.

Potential new or extended data needed:

### User access controls

If not already present:

- `users.disabled_at`
- `users.disabled_by`
- `users.disabled_reason`
- refresh/session revocation table or token version

### Admin audit log

If structured audit storage is not yet persisted beyond logs, add:

- `admin_audit_events`
  - `id`
  - `actor_user_id`
  - `actor_email`
  - `action`
  - `target_type`
  - `target_id`
  - `workspace_id`
  - `request_id`
  - `ip_address`
  - `user_agent`
  - `metadata_json`
  - `created_at`

### Health snapshots

Health can be computed live in v1. If dashboard latency becomes high, add periodic snapshots later.

---

## 11. Security, Privacy, and Compliance

### Access control

- All routes require platform-admin access.
- Frontend route guards are UX only.
- Backend middleware and service checks are authoritative.

### Auditability

Every admin action must log:

- actor user ID
- actor email
- action
- target type
- target ID
- workspace ID when relevant
- timestamp
- IP address
- user agent
- result: success/failure
- reason if provided

Read-only views should at minimum be covered by request audit logs. Mutating actions require durable audit events.

### Sensitive data

Do not display:

- passwords
- JWTs
- OAuth access or refresh tokens
- API keys
- encryption keys
- S3 presigned URLs
- full raw email bodies by default
- customer secrets inside prompts or model payloads

Raw payload views must:

- redact known sensitive fields
- be collapsed by default
- be limited to users with platform-admin access
- be logged when opened if practical

### Cross-tenant data

Because this console is intentionally cross-tenant:

- every page must make the current workspace/tenant context explicit
- list rows must include workspace identifiers where data is tenant-scoped
- actions must require confirmation when they affect a tenant

### Self-protection

Platform admins must not be able to:

- revoke their own platform-admin access
- disable their own login
- revoke their own active session as the only recovery path without confirmation

---

## 12. UX Requirements

### General

- Dense operational UI, optimized for scanning and investigation.
- Tables should support pagination, filters, and copyable IDs.
- Status badges should be consistent across pages:
  - healthy
  - degraded
  - failing
  - disabled
  - unknown
- Detail pages should prioritize summary first, then raw technical detail.
- Empty states should explain what data is missing and whether that is normal.

### Search

Global search should eventually support:

- workspace name/slug/ID
- user email/user ID
- webhook event ID
- conversation ID
- GitHub installation ID

V1 can implement page-level search first.

### Quick links

Every entity detail page should provide links to related admin views:

- workspace -> users, integrations, webhook events, email queue, support tools
- user -> workspaces and recent admin/audit activity
- integration -> workspace and related events/jobs
- health card -> affected diagnostic list

---

## 13. Implementation Plan

### Phase 0 - Foundation check

- Confirm platform-admin access PRD is implemented.
- Confirm `/api/admin/*` audit middleware is active.
- Add a persistent admin audit table if mutating actions are included in v1.
- Add shared admin list response and error conventions.

### Phase 1 - Workspace / tenant overview

Frontend:

- Add `/admin/workspaces`
- Add `/admin/workspaces/:workspaceId`
- Add sidebar item.

Backend:

- Add admin workspace handler/service/repository methods.
- Implement searchable, paginated workspace list.
- Implement workspace detail summary.
- Implement basic usage counts from existing tables.

Acceptance:

- Admin can search by workspace name, slug, ID, and owner email.
- Admin can open a workspace detail page.
- Detail page shows owner, status, modules, created date, usage counts, and quick links.

### Phase 2 - User and access management

Frontend:

- Add `/admin/users`
- Add `/admin/users/:userId`
- Add sidebar item.

Backend:

- Add admin user handler/service/repository methods.
- Implement searchable, paginated user list.
- Implement user detail with memberships and security posture.
- Add safe actions only where backend enforcement exists.

Acceptance:

- Admin can search by email and user ID.
- Admin can view memberships and workspace roles.
- Admin can see platform-admin, MFA, and passkey status.
- Any mutating action writes an audit event.

### Phase 3 - Integration diagnostics

Frontend:

- Add `/admin/integrations`
- Add provider detail pages where useful.
- Link existing Webhook Events page into this section.

Backend:

- Add integration summary service.
- Implement provider-specific diagnostics incrementally.
- Start with Postmark/webhooks, Gmail, GitHub, and Temporal because they are most operationally relevant.

Acceptance:

- Admin can filter integration health by provider and workspace.
- Admin can see last success, last error, and connected account/installation labels.
- Webhook Events support workspace/provider filtering.

### Phase 4 - System health dashboard

Frontend:

- Add `/admin/health`
- Add dashboard cards for API, DB, workers, websocket, email, and integrations.

Backend:

- Add lightweight health aggregation endpoint.
- Reuse existing health checks where available.
- Add DB pool stats and worker/queue stats where available.

Acceptance:

- Admin can see overall platform health at a glance.
- Dashboard exposes latency, traffic, errors, and saturation where metrics exist.
- Every unhealthy card links to a useful diagnostic view.

---

## 14. Acceptance Criteria

### Workspace overview

- Workspace list loads in under 2 seconds for normal production scale with pagination.
- Search supports workspace name, slug, owner email, and ID.
- Detail page shows owner, status, created date, enabled modules, usage, and quick links.
- No workspace secrets are exposed.

### User management

- User list supports email/name/ID search.
- Detail page shows workspace memberships and security state.
- Platform-admin state is visible.
- MFA/passkey status is visible.
- Dangerous self-actions are blocked.
- Mutating actions require reason and confirmation.

### Integration diagnostics

- Admin can view integration health by provider.
- Provider views show last success and last error where available.
- Webhook diagnostics support workspace/provider/event filtering.
- Retry/replay actions are audited and idempotent where implemented.

### System health

- Health dashboard shows API, DB, workers, websocket, email, and integration status.
- Dashboard includes at least one metric from each golden signal category where available:
  - latency
  - traffic
  - errors
  - saturation
- Stale or unavailable metrics are clearly marked unknown, not shown as healthy.

### Security

- All endpoints are under `/api/admin/*`.
- All endpoints reject non-platform-admin users.
- All mutating actions are audit-logged.
- Sensitive fields are redacted.

---

## 15. Metrics for Success

- 80 percent of common tenant investigations start from `/admin/workspaces`.
- Fewer direct production database queries for support/debug tasks.
- Time to identify workspace integration failure is under 2 minutes.
- Time to find a user's workspace memberships is under 30 seconds.
- Every admin mutation has an audit event.
- On-call can determine whether Helpin is healthy from `/admin/health` in under 60 seconds.

---

## 16. Open Questions

1. Do we have a canonical plan/status source today, or should v1 show `unknown` until billing is formalized?
2. Do we want platform-admin grant/revoke in this project, or should that remain DB/manual until a separate security review?
3. What is the canonical source for active sessions and session revocation?
4. Which metrics backend should power latency/traffic/error charts: app DB snapshots, logs, Prometheus, provider API, or a lightweight in-process endpoint?
5. Should raw webhook payload access create a separate durable audit event?
6. Which mutating integration actions are safe enough for v1: retry sync, replay webhook, resend email, or none?
7. Should support engineers get a narrower future role than full platform admin?

---

## 17. Risks

### Overexposure of tenant data

Mitigation:

- redact sensitive fields
- collapse raw payloads
- audit raw detail views where practical
- keep views operational and minimal

### Admin actions cause customer impact

Mitigation:

- keep v1 mostly read-only
- require reason and confirmation for mutations
- make retries idempotent
- audit every action

### Dashboard shows false health

Mitigation:

- distinguish healthy from unknown
- show last-updated timestamps
- avoid green status when health checks fail or are stale

### Scope creep

Mitigation:

- ship in phases
- do not build billing, impersonation, or full workspace settings in v1
- prioritize read-only visibility before controls

---

## 18. Future Work

- Global admin search.
- Durable audit log UI.
- Agent run diagnostics.
- Background jobs/workflows console.
- Support conversations cross-tenant search.
- Billing and usage limits.
- Platform admin sub-roles.
- Alert routing to Slack/PagerDuty.
- Safe customer impersonation with explicit consent and full audit trail.
- Admin API export for incident reports.
