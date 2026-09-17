# Helpin MCP user experience

**Status:** Draft for product, design, security, and frontend review

**Version:** v1.0

**Date:** 2026-07-10

**Owners:** Product, Design, Frontend, Platform, Security, Developer Experience

**Related documents:** [Helpin Public MCP Guide](../public-mcp-server.md), [Helpin Public MCP Server PRD](helpin-public-mcp-server.md), [Helpin Public MCP Server Implementation Plan](../plans/2026-07-10-helpin-public-mcp-server-plan.md)

---

## 1. Summary

Helpin needs a first-class user experience for connecting external AI clients, understanding what they can access, controlling workspace policy, revoking access, creating restricted service credentials, and seeing work performed through MCP.

The MCP backend is not complete as a product if users must configure URLs manually and administrators must inspect database records to understand access. The UI should make three questions easy to answer:

1. **How do I connect my AI client?**
2. **What can this client do in this workspace?**
3. **What has it done, and how do I stop it?**

The feature introduces four related surfaces:

- **AI Clients settings** at `/w/$slug/settings/mcp`
- **OAuth consent** for workspace selection and permission narrowing
- **Connection, policy, service-token, and activity management**
- **MCP attribution** on agent runs and product activity created through MCP

The page follows Helpin's current settings architecture: TanStack file routes, `SettingsRouteViewport`, `SettingsPageFrame`, permission-filtered metadata in `settingsSections.tsx`, TanStack Query hooks, and thin service adapters.

---

## 2. Product Decisions

### 2.1 Use the human-facing name “AI Clients”

The settings sidebar label is **AI Clients**. The page title is **AI Clients & MCP**. Supporting copy explains MCP once rather than requiring users to understand the acronym before entering the page.

### 2.2 Put the page in workspace settings

The route is:

```text
/w/$slug/settings/mcp
```

It appears in the existing **Workspace** settings group after **Access** and before **Repositories** because every connection is bound to exactly one workspace.

This PRD does not reorganize the rest of the settings navigation into a new Integrations group.

### 2.3 Separate personal access from workspace administration

Every eligible workspace member can:

- view setup instructions
- connect an AI client as themselves
- list and revoke their own MCP connections
- view the effective workspace policy and their granted toolsets

Workspace settings managers can additionally:

- enable or disable MCP for the workspace
- enforce read-only mode
- choose allowed toolsets
- view and revoke all workspace connections
- control whether service identities are allowed
- create, rotate, and revoke restricted service tokens when permitted
- view MCP activity and security-relevant denials

Beta uses existing Helpin permissions:

- personal connection access: authenticated workspace membership plus `workspace.read`
- policy and all-connection management: `settings.manage`
- activity visibility: `settings.read`; sensitive security detail remains platform/operator-only

A dedicated `mcp.read`/`mcp.manage` permission may be introduced before GA if customers need separate delegation. The UI must consume backend capability booleans rather than infer authority from role labels.

### 2.4 Make read-only the safe default

New workspace policy defaults to MCP enabled only when explicitly activated and connections read-only unless the user and workspace policy both permit requested write scopes.

The UI consistently distinguishes:

- read-only connection
- bounded write connection
- service identity
- disabled or revoked connection

### 2.5 Show results in existing product surfaces

MCP is an access channel, not a separate work universe. Tasks, documents, notes, and agent runs created through MCP appear in their normal Helpin locations. The UI adds attribution and links, not duplicate MCP-specific copies of those entities.

---

## 3. Goals

1. Let a user connect a supported AI client in under two minutes.
2. Make the selected workspace, client identity, scopes, toolsets, and read-only status obvious before authorization.
3. Let users revoke their own connection without administrator help.
4. Let workspace managers understand and control the workspace's MCP exposure.
5. Make restricted service-token creation safe, deliberate, and one-time-secret aware.
6. Provide a useful, sanitized audit view for MCP calls and access failures.
7. Attribute MCP-originated product activity and agent runs to both the Helpin actor and external client.
8. Reuse current Helpin settings, query, access, modal, table, toast, and responsive patterns.
9. Make disabled, denied, expired, revoked, and partially configured states understandable.

---

## 4. Non-Goals

- Building the MCP protocol server or OAuth token engine in the frontend.
- Displaying access tokens, refresh tokens, token hashes, authorization codes, or internal security data.
- Letting users bypass backend scope, RBAC, module, entitlement, or workspace policy checks.
- Editing individual MCP tool schemas in the UI.
- Building a general OAuth application developer portal in v1.
- Showing raw tool inputs and outputs in the activity table.
- Exposing chain of thought or private model reasoning from MCP-started agent runs.
- Adding customer-visible support sends, destructive actions, Docs publishing, member management, or integration administration to v1.
- Replacing existing Agent Dock or run-detail experiences.
- Supporting cross-workspace connections from one consent flow.

---

## 5. Personas and Jobs

### 5.1 Workspace member

**Job:** “Connect Codex, Claude, Cursor, or another assistant to this workspace and know what it can do.”

Needs:

- simple setup instructions
- clear workspace identity
- readable permissions
- safe defaults
- personal revoke control

### 5.2 Workspace administrator

**Job:** “Control which external AI capabilities are available and audit how they are used.”

Needs:

- workspace enable/disable
- allowed toolsets and read-only enforcement
- connection inventory
- immediate revoke
- service identity controls
- sanitized audit visibility

### 5.3 Automation builder

**Job:** “Create a narrowly scoped credential for a headless workflow without using my personal login.”

Needs:

- named service identity
- expiry and scope controls
- one-time token copy
- rotation/revocation
- last-used and activity visibility

### 5.4 Reviewer or operator

**Job:** “Understand whether a task, document, or run came through MCP and which principal caused it.”

Needs:

- actor and client attribution
- connection or service identity
- relevant run/entity link
- outcome and denial reason without raw sensitive content

---

## 6. Information Architecture

### 6.1 Settings navigation

Add a section to `frontend/src/lib/settingsSections.tsx`:

```text
Workspace
  General
  Billing
  Members
  Teams
  Access
  AI Clients       <- new
  Repositories
  Knowledge
```

Proposed metadata:

```text
id: mcp
label: AI Clients
description: Connect external AI assistants and manage their access to this workspace.
group: Workspace
```

The route must be a concrete file route, matching existing workspace settings pages:

```text
frontend/src/routes/_authenticated/w/$slug/settings/mcp.tsx
frontend/src/pages/settings/MCPSettingsPage.tsx
```

### 6.2 Page sections

The page uses a compact tab or segmented navigation when all admin capabilities are available:

- **Setup**
- **Connections**
- **Service accounts**
- **Activity**
- **Workspace policy**

For members without administrative capabilities:

- Setup
- My connections

Tabs the user cannot access are omitted rather than shown as permanently disabled. Direct URLs to unauthorized tabs render an inline permission state and do not leak counts or metadata.

### 6.3 OAuth routes

OAuth consent is outside the workspace settings shell because the user may need to sign in and select a workspace first:

```text
/oauth/authorize
/oauth/error
/oauth/success
```

The backend remains authoritative for OAuth parameters, client metadata, workspace choices, requested scopes, and redirect validation.

---

## 7. AI Clients Settings Page

### 7.1 Page header

Title:

```text
AI Clients & MCP
```

Description:

```text
Connect AI assistants to Helpin, control what they can access, and review their activity.
```

Header actions:

- **Connect a client** — opens setup/client chooser
- **Create service account** — managers only and only when workspace policy permits
- documentation link

If MCP is disabled by workspace policy, the page shows a clear disabled state before the rest of the content. Managers get an **Enable MCP** action; members see who can enable it.

### 7.2 Overview summary

Managers see compact summary values:

- active user connections
- active service accounts
- calls in the last 7 days
- denied or rate-limited calls in the last 7 days

Members see:

- their active connections
- effective mode: read-only or read/write
- allowed toolsets

These are operational summaries, not decorative analytics. Do not add charts unless beta usage proves trend visualization is useful.

### 7.3 Layout sketch

```text
+------------------------------------------------------------------+
| AI Clients & MCP                         [Connect a client]       |
| Connect AI assistants and manage workspace access.               |
+------------------------------------------------------------------+
| Setup | Connections | Service accounts | Activity | Policy       |
+------------------------------------------------------------------+
|                                                                  |
|  Content for selected section                                    |
|                                                                  |
+------------------------------------------------------------------+
```

On narrow screens, tabs scroll horizontally or collapse into the existing accessible tab/select pattern. Tables become stacked rows with the primary action preserved.

---

## 8. Setup Experience

### 8.1 Client chooser

Show supported-client options:

- Codex
- Claude Code/Desktop
- Cursor
- VS Code / GitHub Copilot
- ChatGPT custom connector
- Other MCP client

Each option includes:

- client name and icon
- support status: tested, beta, or manual
- authentication method
- one-click install action where supported
- manual configuration fallback

### 8.2 Connection details

Always show:

```text
Server URL: https://mcp.helpin.ai/mcp
Authentication: OAuth 2.1
Workspace: <current workspace>
```

Provide copy actions with confirmation feedback. Do not show a token or instruct the user to paste a Helpin web JWT.

### 8.3 Guided instructions

Instructions are client-specific and generated from a versioned frontend/backend configuration, not scattered hard-coded strings across components.

Each guide covers:

1. Add the server.
2. Complete Helpin sign-in and consent.
3. Verify with `get_current_context`.
4. Run a safe example such as listing tasks or searching Docs.
5. Manage or revoke the connection in Helpin.

### 8.4 Empty state

When the user has no connections:

```text
Connect your AI assistant

Use Helpin tasks, documents, CRM, support context, and agents from the tools where you already work.

[Connect a client]  [Read how access works]
```

Avoid framing MCP as developer-only infrastructure.

---

## 9. OAuth Consent Experience

### 9.1 Consent content

The page shows:

- verified client name
- client and redirect domains
- selected workspace name and organization
- authenticated Helpin user
- requested toolsets
- individual scope groups in readable language
- read-only/read-write status
- workspace policy restrictions
- connection and token expiry behavior
- link to security documentation

### 9.2 Workspace selection

Only workspaces where the user is currently eligible are listed. Each option shows workspace name, organization, role, and whether MCP is enabled.

One authorization selects exactly one workspace. The user cannot paste or edit a workspace ID.

### 9.3 Permission narrowing

The client requests OAuth scopes. Helpin proposes matching toolsets after applying workspace policy.

The user may:

- remove optional toolsets
- remove optional write access
- force the connection to read-only
- choose a different eligible workspace

The user may not add authority beyond the client request or workspace policy. Expansion later requires reauthorization.

### 9.4 Risk communication

Use readable groups rather than a wall of scope strings:

```text
Projects — Read tasks and teams
Docs — Search and read documents
Projects — Create and update tasks
Agents — Start Helpin agents and view their results
```

Write groups are visually distinct and say what can change. Avoid alarming generic warning banners when a precise action description is available.

### 9.5 Consent actions

- **Allow connection** — primary
- **Cancel** — returns a standards-compliant OAuth denial
- **Back to workspace selection**

The primary button includes the selected workspace name when space allows:

```text
Allow access to Acme Workspace
```

### 9.6 Error states

Dedicated, non-technical states:

- invalid or untrusted redirect
- client registration disabled
- MCP disabled for workspace
- user lost workspace access
- required client scope blocked by workspace policy
- authorization request expired
- sign-in or MFA required

Errors never reveal raw OAuth parameters, client secrets, token values, or other workspace names.

---

## 10. Connections

### 10.1 Connection list

Each row shows:

- client name and icon
- user or service identity
- workspace-bound mode
- toolset badges
- read-only/read-write badge
- status: active, expired, revoked, or disabled by policy
- created and last-used time
- recent call count when authorized
- overflow actions

Member default is **My connections**. Managers can switch to **All workspace connections**.

### 10.2 Filters

- status
- client
- identity type
- read-only/read-write
- toolset
- last used

Search by client name, user/service identity, or visible connection ID prefix.

### 10.3 Connection detail

Open a side panel or detail page with:

- client and verified domains
- connection ID
- actor and workspace
- granted scopes and toolsets
- created, expires, last used, and revoked timestamps
- read-only status
- last activity summary
- relevant security/policy status
- revoke action

Never show access or refresh tokens.

### 10.4 Revoke

Revoke confirmation states the effect:

```text
Claude Code will immediately lose access to this workspace. Existing Helpin records and already-started agent runs are not deleted.
```

The dialog identifies whether active MCP-started agent runs will continue under current product policy. V1 recommendation: revoking a connection prevents new calls but does not automatically cancel already-started Helpin agent runs; the user can open and cancel those runs separately.

After success:

- remove or mark the row revoked without a full page reload
- invalidate connection and activity queries
- show a concise toast
- do not offer token recovery

### 10.5 Expired and policy-disabled connections

Expired connections offer **Reconnect** if the client supports it. Connections disabled by workspace policy explain that an administrator must change policy; reconnecting cannot bypass that state.

---

## 11. Workspace Policy

Managers configure:

- MCP enabled/disabled
- enforce read-only for all user connections
- allowed toolsets
- user connections enabled/disabled
- service accounts enabled/disabled
- maximum service-token expiry allowed
- optional allowed-client policy when enterprise controls exist

### 11.1 Toolset control

Use named product groups with examples:

- Context
- Search
- Projects
- Docs
- CRM
- Support
- Agents
- Git

Each row shows:

- what it permits
- read and/or bounded-write availability
- module/plan availability
- whether enabled for this workspace

Changing policy cannot expand existing connection scopes. Narrowing policy takes effect on the next request and the UI marks affected connections as restricted by policy.

### 11.2 Disable workspace MCP

Disabling MCP requires confirmation and explains:

- all user and service credentials stop working immediately
- existing product records remain
- active agent runs are not automatically deleted
- re-enabling does not silently restore revoked connections

### 11.3 Unsaved changes

Policy changes use explicit Save/Cancel behavior with the established sticky settings footer when multiple fields can change together. Do not auto-save security policy toggles individually if doing so creates ambiguous partial policy.

---

## 12. Service Accounts and Restricted Tokens

### 12.1 List

Managers see:

- service account name
- active token count
- toolsets and read/write mode
- created by
- last used
- expiry
- status

### 12.2 Create flow

Fields:

- descriptive name
- purpose/owner note
- expiry: 30 days, 90 days, or policy-limited custom date
- toolsets
- read-only/write scopes allowed by workspace policy

Before creation, show a compact authority summary. The user confirms that the credential is for headless automation and should be stored in a secret manager.

### 12.3 One-time token reveal

After creation:

- show the token exactly once
- provide Copy action
- require acknowledgement that it cannot be viewed again
- show an environment-variable/header example without embedding the actual token in analytics or logs
- closing the dialog permanently hides the token

Do not put the token in the URL, route state, browser storage, query cache, toast, error report, or telemetry.

### 12.4 Rotate and revoke

Rotation creates a new token and supports a short, explicit overlap window only if backend policy permits. Revocation is immediate. The UI distinguishes revoking one token from disabling the service identity.

---

## 13. MCP Activity

### 13.1 Activity table

Managers with activity permission see sanitized MCP audit events:

- timestamp
- client
- actor/service identity
- action category and tool name
- read/write/risk classification
- safe target reference and Helpin link where available
- outcome: success, denied, rate-limited, or error
- duration
- correlation ID copy action

### 13.2 Privacy boundary

The table does not show:

- raw tool input or output
- message/document bodies
- tokens, hashes, authorization codes, or cookies
- chain of thought
- internal stack traces
- security detection details that could help bypass controls

Tool parameters may be represented only as normalized safe target references, such as `ENG-482` or a document title the viewer already has permission to read.

### 13.3 Filters and detail

Filters:

- time range
- connection/client
- actor
- toolset/tool
- read/write
- outcome

The detail panel shows policy decision reason codes and linked Helpin entities where authorized. Platform-only security data remains in the operations console, not workspace settings.

### 13.4 Retention messaging

The page states the effective customer-visible activity retention and links to policy. Expired events disappear without suggesting that product activity itself was deleted.

---

## 14. Product and Agent-Run Attribution

### 14.1 Agent runs

Agent Dock and run detail show a compact origin label:

```text
Started via Claude Code by Alex
```

The label links to the MCP connection detail only when the current viewer may access it. Otherwise it remains plain text.

The normal Helpin user is the actor. The external client is attribution metadata, not a replacement user.

### 14.2 Product activity

Task, document, CRM, Support draft, and other activity entries created through MCP use normal product activity with secondary attribution:

```text
Alex created ENG-482 via Codex
```

No activity should say “MCP created this” without identifying the authenticated user or service identity.

### 14.3 Asynchronous runs

The settings UI is not a live run monitor. MCP-started runs appear in existing Agent Dock/run surfaces, where users can inspect, respond, or cancel according to normal permissions.

Revoked connections remain visible in historical attribution. Deleting a connection does not rewrite historical product actors.

---

## 15. UI States

Every section defines:

- loading skeleton
- empty state
- permission-denied state
- MCP-disabled state
- recoverable API error with retry
- stale/revoked data refresh
- partially unavailable module/toolset
- rate-limited response

### 15.1 Optimistic behavior

Use optimistic updates only for reversible, low-risk UI state. Revocation, policy updates, token creation, and rotation wait for server confirmation before showing success.

### 15.2 Query behavior

- Connections and policy use workspace-scoped query keys.
- Personal and all-workspace connection lists use distinct keys.
- Revoke invalidates connection, policy summary, and activity queries.
- Policy updates invalidate the effective capability view and connection restrictions.
- Audit polling is conservative; no live WebSocket is required for v1.

---

## 16. Frontend Architecture

Recommended files:

```text
frontend/src/routes/_authenticated/w/$slug/settings/mcp.tsx
frontend/src/pages/settings/MCPSettingsPage.tsx
frontend/src/components/settings/mcp/
  MCPSetupPanel.tsx
  MCPConnectionsPanel.tsx
  MCPConnectionDetail.tsx
  MCPWorkspacePolicyPanel.tsx
  MCPServiceAccountsPanel.tsx
  MCPTokenRevealDialog.tsx
  MCPActivityPanel.tsx
  MCPToolsetSummary.tsx
frontend/src/lib/services/mcpService.ts
frontend/src/hooks/queries/useMCP.ts
```

Also update:

- `frontend/src/lib/settingsSections.tsx`
- `frontend/src/lib/queryKeys.ts`
- frontend permission/capability types
- router-generated route tree
- settings-section tests

### 16.1 Data access

Use TanStack Query hooks over a thin `mcpService`. Do not call `fetch` directly from page components. The service returns typed API envelopes consistent with existing frontend conventions.

### 16.2 Proposed UI API surface

Exact backend paths may change, but the UI needs these capabilities:

```text
GET    /api/mcp/settings?workspace_id=...
PUT    /api/mcp/settings?workspace_id=...
GET    /api/mcp/connections?workspace_id=...&view=mine|workspace
GET    /api/mcp/connections/{id}?workspace_id=...
DELETE /api/mcp/connections/{id}?workspace_id=...
GET    /api/mcp/activity?workspace_id=...&cursor=...
GET    /api/mcp/service-principals?workspace_id=...
POST   /api/mcp/service-principals?workspace_id=...
POST   /api/mcp/service-principals/{id}/tokens?workspace_id=...
POST   /api/mcp/access-tokens/{id}/rotate?workspace_id=...
DELETE /api/mcp/access-tokens/{id}?workspace_id=...
```

Responses include explicit capability booleans such as:

```text
can_connect_personally
can_manage_policy
can_view_workspace_connections
can_manage_service_accounts
can_view_activity
```

The frontend must not recreate the backend authorization matrix from role names.

---

## 17. Accessibility, Responsive Design, and Content

### 17.1 Accessibility

- Full keyboard navigation for tabs, lists, dialogs, and menus.
- Visible focus states.
- Status is communicated with text, not color alone.
- Token copy and one-time reveal announcements use appropriate live regions.
- Consent groups have programmatic names and descriptions.
- Destructive revoke/disable dialogs return focus to the initiating control.
- Icons have labels where they convey client or risk meaning.

### 17.2 Responsive design

- Consent works at mobile width without horizontal overflow.
- Connection/activity tables collapse to readable cards.
- Primary revoke and copy actions remain reachable.
- Long client names, toolsets, and scope descriptions wrap safely.
- Secret tokens are horizontally scrollable within their own controlled field and never force page overflow.

### 17.3 Content guidelines

- Say “AI client” before “MCP client” in user-facing onboarding.
- Use “read-only” and “can make changes” rather than “GET/POST access.”
- Use exact actions: “Create tasks” rather than “write PM.”
- Explain immediate consequences for revoke and disable actions.
- Never imply that Helpin can recover a token shown once.

---

## 18. Analytics and Telemetry

Product events:

- MCP settings viewed
- setup client selected
- install instructions copied
- OAuth consent started/completed/denied by reason category
- connection detail viewed
- personal/workspace connection revoked
- workspace policy changed
- service identity/token created, rotated, or revoked
- activity filters used
- MCP-origin link opened from a product activity or run

Telemetry must never include token values, authorization codes, raw scope parameters from untrusted clients, tool inputs/outputs, document content, support content, or CRM PII.

---

## 19. Success Metrics

### Activation

- Median time from opening Setup to successful connection under two minutes.
- At least 90% of supported-client OAuth attempts complete without operator help.
- At least 80% of connected users make a successful first tool call in the same session.

### Comprehension and control

- At least 90% of usability-test participants correctly identify the selected workspace and whether a connection can make changes.
- At least 90% can revoke their connection without documentation.
- Under 5% of consent cancellations are attributed to unclear permission language.

### Administration

- P99 revoke-to-enforcement time displayed as completed only after backend confirmation and remains under the backend five-second SLO.
- 100% of service-token creations show the one-time-secret acknowledgement.
- 100% of workspace policy changes produce an auditable backend event.

---

## 20. Rollout

### Phase 1 — Navigation, setup, and personal connections

- route and settings metadata
- setup/client instructions
- personal connection list/detail/revoke
- disabled and read-only states

### Phase 2 — OAuth consent

- workspace selection
- requested permissions and toolset narrowing
- read-only override
- OAuth success/error return states

### Phase 3 — Workspace administration

- workspace policy
- all-connections view
- manager revoke
- effective capability states

### Phase 4 — Service accounts and activity

- service identity list/create
- one-time token reveal
- rotate/revoke
- sanitized activity table and filters

### Phase 5 — Product attribution and polish

- agent-run origin
- product activity origin
- install links and client compatibility content
- accessibility and responsive audit
- analytics and usability validation

Each phase remains behind the corresponding backend capability and workspace feature flag. The UI must not show an action before its backend authorization and audit path is complete.

---

## 21. Acceptance Criteria

1. `AI Clients` appears in the Workspace settings group and routes to `/w/$slug/settings/mcp`.
2. Members can access setup and manage only their own connections.
3. Managers can manage workspace policy and all connections only when backend capability flags permit it.
4. The setup screen supports Codex, Claude, Cursor, VS Code, ChatGPT, and a generic client path without displaying secrets.
5. Consent shows the verified client, redirect domain, workspace, requested permission groups, toolsets, and read-only/write state.
6. Consent allows narrowing but never expanding requested or policy-allowed authority.
7. One consent authorization binds exactly one workspace.
8. Personal and manager revocation take effect through the backend and the UI never reports success before confirmation.
9. Connection lists show client, actor, toolsets, mode, status, created time, and last-used time.
10. Workspace policy narrowing marks affected connections and is enforced on the next backend request.
11. Service tokens are displayed once and never enter URLs, storage, query cache, telemetry, or error reporting.
12. Activity shows sanitized metadata and never raw tool inputs/outputs or tokens.
13. MCP-started agent runs and product activity identify the Helpin actor/service identity and external client.
14. Revoking a connection does not delete existing entities or silently cancel durable agent runs; the UI explains this.
15. Loading, empty, denied, disabled, expired, revoked, rate-limited, and recoverable-error states are covered by tests.
16. Settings navigation, dialogs, token reveal, tabs, and tables pass keyboard and screen-reader checks.
17. Mobile layouts preserve the consent decision, copy, revoke, and token acknowledgement actions.
18. Query invalidation keeps connections, policy, capability summaries, and activity consistent after mutations.
19. Frontend tests verify section visibility, backend capability gating, consent narrowing, one-time-secret handling, and audit redaction.
20. No MCP UI is enabled in production until the corresponding backend auth, policy, audit, rate-limit, and kill-switch gates pass.

---

## 22. Risks and Mitigations

| Risk | Mitigation |
| --- | --- |
| Users authorize the wrong workspace | Prominent workspace identity, explicit one-workspace selection, workspace name in primary consent action. |
| Scope language is too technical | Group scopes into product actions with exact read/change examples; retain raw scope names only in expandable details. |
| Frontend infers permissions incorrectly | Backend returns capability booleans; direct endpoints independently authorize every request. |
| Token leaks through browser state or telemetry | One-time local component state only, strict analytics exclusions, security tests, no URL/cache/storage use. |
| Admin policy appears to revoke more than it does | Explain whether credentials are restricted or revoked and whether active agent runs continue. |
| Activity exposes sensitive content | Sanitized audit contract, safe target references, no raw input/output, authorization on entity links. |
| Page becomes an operational dashboard | Keep summary compact; detailed platform security remains in the admin operations console. |
| MCP jargon reduces adoption | Lead with AI clients and concrete outcomes; explain MCP contextually. |
| UI ships ahead of backend safety | Feature flags and acceptance gate require backend auth, audit, rate limits, and kill switches first. |

---

## 23. Open Decisions

1. Should GA introduce dedicated `mcp.read` and `mcp.manage` permissions, or continue using workspace/settings permissions?
2. Which clients have one-click installation at beta versus manual setup instructions?
3. Should managers see all user connections by default, or only aggregate counts until they open an administrative view?
4. What overlap window, if any, is permitted when rotating a service token?
5. Should enterprise admins be able to allowlist client registrations or verified domains in the first GA release?
6. Where should the sanitized MCP activity export live: this page, the platform admin console, or both with different scopes?

---

## 24. Recommended First UI Release

The smallest useful UI release includes:

- AI Clients settings navigation and route
- MCP enabled/read-only workspace summary
- tested-client setup instructions and server URL copy
- OAuth workspace selection and consent narrowing
- personal connection list, detail, and revoke
- manager workspace enable/disable and allowed-toolset policy
- basic all-connections inventory
- MCP origin on Helpin agent runs

Service accounts, detailed activity, cross-client install polish, and broader product attribution can follow without changing the core information architecture.
