# Helpin Support Desktop Plan

## Status

Active implementation plan for a dedicated Helpin desktop application focused on support operations on macOS and Windows.

Primary decisions:

- use Tauri 2 as the native shell
- ship a support-first desktop app, not a full Helpin desktop client
- include contextual CRM only where it helps an active support conversation
- prioritize desktop auth/session and realtime resilience before broader UI extraction
- start with one shared package, `support-core`
- defer `support-ui` extraction until duplication justifies it
- keep the desktop app on Tauri because it preserves a future mobile path

Current implementation progress:

- `packages/support-core` now owns shared session/auth handling, shared support query/state logic, and the desktop support realtime controller
- the web app is already using the extracted shared auth and support-core pieces
- `apps/support-desktop` is now a real Tauri app boundary with `src-tauri` scaffolding and a React/Vite frontend
- desktop auth persistence now hydrates and stores session data through a Tauri-native store-backed adapter
- desktop support now mounts the real shared support inbox UI from `frontend/src/components/support`
- desktop realtime now handles connect, reconnect, offline/online transitions, wake/stale recovery, and support query resync
- desktop support UI has been brought much closer to web parity, including desktop-specific shell polish and a `WorkspaceSwitcher` in the sidebar
- native desktop notifications are now implemented for inbound support messages, including permission handling, dedupe, per-conversation cooldown, focused-thread suppression, click-through routing, and window reveal/focus while the app is already running
- the desktop shell now includes a light/dark/system theme menu in the native Tauri menu bar

Current delivery state:

- completed:
  - desktop host scaffold
  - shared auth/session extraction
  - desktop session persistence
  - support-core extraction for the initial support logic layer
  - support-only realtime resilience for desktop
  - shared support inbox mounted in desktop
  - native desktop notifications for support messages
  - desktop visual parity pass for the core support workspace
- in progress:
  - validating the integrated support experience end to end in desktop across real app lifecycle scenarios
- remaining:
  - tray/menu bar
  - unread badge
  - deep links
  - single-instance behavior
  - updater
  - signing/packaging pipeline
  - stronger secret storage hardening beyond the current Tauri store adapter
  - bundle trimming and code-splitting for the large desktop bundle
  - full desktop QA across sleep/wake, reconnect, notification click-through, workspace switching, and installed macOS/Windows builds

What is done now:

- the desktop app can sign in, persist session, restore session, and switch into a real support workspace
- the desktop app renders the real shared support inbox UI instead of a placeholder preview
- support realtime is resilient enough for reconnect, offline/online transitions, and wake/stale recovery
- desktop notifications can alert for inbound support messages and route to the correct conversation while the app is already running
- support-linked CRM, docs, and PM routes intentionally hand off to the web app instead of bloating the desktop surface

What is left now:

- tray or menu bar presence
- unread badge integration
- deep-link handling from outside the app
- single-instance behavior
- updater integration
- release packaging, signing, and notarization
- stronger credential hardening if Tauri Store is not sufficient for long-lived secrets
- performance and bundle-size cleanup
- end-to-end QA and release hardening

Known current limitation:

- notification click-to-open works when the desktop app process is already running; cold-start open from a notification is not implemented yet

## Goal

Create a production-grade native desktop app for Helpin support that:

- lets agents stay online all day without depending on browser tabs
- handles inbound conversation workflows faster than the current web experience
- uses the same backend APIs and support domain logic as the web app
- minimizes frontend divergence by sharing support logic across web and desktop
- adds desktop-native behavior where it materially improves support operations

## Why This Plan

Helpin already has a substantial web support surface:

- support routes and page shell already exist in `frontend`
- support API access is centralized
- support query hooks and websocket flows already exist

The main opportunity is not rebuilding support UI from scratch. It is extracting the right shared seams so a desktop host can mount the same support product logic while adding native features:

- tray/menu bar presence
- native notifications
- unread badges
- deep linking to conversations
- single-instance app behavior
- secure desktop session storage

## Product Scope

## In Scope For V1

- inbox list
- conversation thread
- reply composer
- attachments
- canned responses
- AI draft rewrite / agent assist flows already supported by backend APIs
- assignment, status changes, mailbox switching
- teammate presence, viewing, typing, and unread counts
- native notifications
- tray or menu bar presence
- deep link directly to a conversation
- desktop preferences needed for support workflows
- contextual CRM in the conversation sidebar:
  - contact summary
  - company summary
  - recent deal summary
  - quick note or handoff actions
  - open full CRM record in the browser

## Explicitly Out Of Scope For V1

- full CRM pipelines
- CRM list views
- sequences and campaign workflows
- broad PM workflows beyond support-linked shortcuts
- workspace settings beyond minimal desktop preferences
- a general-purpose Helpin desktop client

## Boundary Rule

The desktop app is a support workstation. It is not a full replacement for the web app.

Open in-app:

- support inbox
- support conversation
- support-linked contact and company context
- support-linked quick CRM or task handoff actions

Open in the default browser:

- full CRM records and pipelines
- settings and admin pages
- PM pages beyond support handoff shortcuts
- any low-frequency workflow that does not justify desktop-native treatment

This rule should be enforced intentionally to avoid product drift.

## Platform Choice

Use Tauri 2.

Reasons:

- Helpin already uses React 19 + Vite
- the desktop app is primarily a native shell around existing frontend and backend contracts
- Tauri keeps shell code smaller than Electron for this use case
- Tauri gives a clear path for notifications, tray behavior, deep links, single-instance handling, and updating
- Tauri also preserves a plausible iOS and Android path if Helpin later wants mobile support workflows from the same host technology

## Current Code Constraints

Current support functionality already has useful seams:

- `frontend/src/pages/pm/Support.tsx` is thin
- `frontend/src/lib/services/supportService.ts` centralizes support HTTP calls
- `frontend/src/hooks/queries/useSupport.ts` centralizes support queries and mutations
- `frontend/src/hooks/useWebSocket.ts` centralizes the main workspace websocket path

Current blockers for desktop reuse:

- auth/session is tightly coupled to `localStorage`
- request retry and refresh behavior assume browser storage and browser redirects
- websocket lifecycle assumes browser tab visibility and interaction patterns
- support layout and related components depend on app-level router and stores
- some support components directly reference `window`, `document`, and `navigator`

Current notes after implementation:

- the desktop app now bypasses the worst of the original auth/session coupling through an adapter-based shared session layer
- support-only websocket and resync behavior is implemented for desktop, but the older web-wide websocket/realtime stack still exists separately
- the desktop app now consumes the real support UI directly from `frontend/src`, so bundle size and selective code-splitting are now practical follow-up concerns
- support-linked CRM, docs, and PM routes currently open in the web app instead of being implemented in desktop

## Architecture Direction

## 1. Build A Dedicated Desktop Host

Add a desktop app under `apps/support-desktop`.

The desktop host owns:

- Tauri configuration
- native notifications
- tray or menu bar behavior
- deep links
- app startup behavior
- secure credential storage
- packaging and updates

The desktop host does not own support business logic.

## 2. Extract Shared Support Logic Into `support-core`

Create `packages/support-core` for shared support logic used by both web and desktop.

Initial contents:

- support domain types
- support API client
- query hooks
- websocket and presence helpers
- support inbox state
- host adapter interfaces for auth, navigation hooks, notifications, and external linking where needed

Do not start by extracting all support UI into a separate package.

## 3. Keep UI Extraction Incremental

Initially:

- keep most support UI components in the current frontend tree or move only the minimum needed
- extract logic first
- introduce adapter seams where components currently depend on router or app-level stores

If web and desktop start sharing enough presentational components to justify it, extract `support-ui` later.

## 4. Define Host Adapters Early

The shared logic should not assume browser-only behavior.

Expected adapter seams:

- session storage
- login/logout/session expiry behavior
- navigation to conversation routes
- open external links
- desktop notification behavior
- browser fallback behavior

## Workstreams

## A. Auth And Session Workstream

This is the first critical path item.

Requirements:

- desktop sign-in flow
- secure storage for long-lived session material
- access token refresh
- logout
- workspace switching
- app restart session recovery
- explicit session expiry handling

Do not ship desktop auth by reusing the current `localStorage` pattern unchanged.

## B. Realtime And Resilience Workstream

This is the second critical path item.

Requirements:

- websocket connect and reconnect
- sleep/wake recovery
- offline/online recovery
- stale socket detection
- post-reconnect resync for unread counts and active conversation state
- safe handling of desktop-specific lifecycle differences from browser tabs

Realtime resilience is core product behavior for a support app and must not be treated as late polish.

## C. Shared Logic Extraction Workstream

Requirements:

- move support domain logic into `support-core`
- preserve current web support behavior during migration
- reduce direct dependencies on browser storage and router-only state where possible
- keep the web app functioning throughout the extraction

## D. Desktop Shell Workstream

Requirements:

- inbox shell and desktop layout
- native notifications
- tray/menu bar
- unread badge
- deep links to conversations
- single-instance behavior
- start minimized or reopen behavior when needed
- updater integration

## E. Build, Packaging, And Signing Workstream

This starts early, not at the end.

Requirements:

- choose build strategy for macOS and Windows runners
- define signing and notarization requirements
- estimate CI cost and runtime
- add internal build workflows early
- produce predictable internal builds before beta

## Sequencing

## Phase 1: Desktop Auth Spike

Timebox: weeks 1-2

Deliverables:

- scaffold `apps/support-desktop`
- prove desktop login, token refresh, logout, and restart recovery
- introduce session storage adapter or equivalent abstraction
- decide secure storage approach for desktop

Exit criteria:

- desktop app can authenticate and stay authenticated across restart
- session expiry and logout behavior are defined and testable

Status:

- completed
- implemented with shared session storage adapters plus a Tauri store-backed desktop session backend

## Phase 2: Desktop Realtime Spike

Timebox: weeks 2-3

Deliverables:

- websocket adapter or refactor for desktop lifecycle handling
- reconnect and resync strategy
- tests or scripted verification for sleep/wake and offline/online flows

Exit criteria:

- desktop app reliably reconnects after network loss or wake
- unread and active conversation state recover without manual reload

Status:

- completed for support desktop scope
- implemented as a support-only shared realtime controller with reconnect, stale detection, and support query resync

## Phase 3: Extract `support-core`

Timebox: weeks 3-5

Deliverables:

- shared support API client
- shared support query hooks
- shared presence and websocket helpers
- shared support state
- web app migrated to use shared support logic

Exit criteria:

- web support remains functional on top of shared core
- desktop app can import the same logic layer

Status:

- partially completed
- shared auth/session, support hooks, support query keys/cache helpers, support stores, and support realtime are extracted
- broader UI extraction into a separate package is intentionally deferred

## Phase 4: Mount Inbox In Desktop

Timebox: weeks 5-6

Deliverables:

- conversation list
- conversation thread
- reply composer
- attachments
- assignment and status actions
- mailbox switching

Exit criteria:

- a support teammate can handle real conversations end to end in the desktop app

Status:

- completed for the current integration milestone
- desktop now mounts the real shared support inbox layout from the web app instead of a desktop-only preview

## Phase 5: Native Shell Features

Timebox: weeks 6-8

Deliverables:

- native notifications
- tray/menu bar presence
- unread badge
- notification click-through to conversation
- deep link handling
- single-instance behavior

Exit criteria:

- desktop experience is materially better than a browser tab for active support work

Status:

- not started beyond the base Tauri shell scaffold

## Phase 6: Beta Hardening

Timebox: week 9+

Deliverables:

- error and crash visibility
- offline and reconnect edge-case fixes
- keyboard flow improvements
- packaging/signing completion
- internal beta rollout checklist

Exit criteria:

- signed internal builds for macOS and Windows
- release checklist and rollback path

Status:

- not started

## Acceptance Criteria

The V1 desktop app is successful if:

- support reps can keep it open all day as their main support workstation
- notification-to-thread response flow is faster than web
- support business logic is shared across web and desktop through `support-core`
- desktop-specific code remains mostly shell and integration code
- full CRM remains out of scope while contextual CRM supports support workflows
- reconnect failures or stale data are rare, visible, and recoverable

## Testing Strategy

- unit tests for shared support logic and adapter behavior
- integration tests for support flows built on shared logic
- desktop smoke coverage for:
  - sign in
  - restart recovery
  - websocket reconnect
  - notification opens exact conversation
  - send reply
  - attachment upload
  - unread badge updates
- manual platform QA on:
  - macOS Apple Silicon
  - macOS Intel if still supported
  - Windows 11

## Risks

## 1. Auth Is More Disruptive Than It Looks

The current request flow assumes browser storage and browser redirects. Desktop auth must be treated as a first-order architectural seam.

## 2. Realtime Reliability Is A Product Requirement

Sleep, wake, and network loss behavior differ materially from browser tab behavior. This can create silent stale-state failures if not handled early.

## 3. Scope Drift Into A Full Helpin Client

The main product risk is slowly adding CRM, PM, and settings until the desktop app becomes a second full client to maintain.

## 4. Packaging And Signing Add Operational Complexity

macOS and Windows builds, signing, notarization, and CI runner cost must be planned early.

## 5. Over-Extraction Can Slow Delivery

If package boundaries become too elaborate too early, the team will spend time polishing abstractions instead of shipping a usable desktop client.

## Non-Goals

- maintaining separate native UI codebases for macOS and Windows
- achieving complete parity with the browser app in V1
- moving every support component into a shared UI package up front
- turning the desktop app into the default surface for all Helpin modules

## Immediate Next Steps

1. Add native desktop behavior: notifications, tray/menu bar, unread badge, and deep links.
2. Add single-instance handling and startup behavior for desktop workflows.
3. Decide whether the current Tauri store-backed session persistence needs stronger secret storage hardening.
4. Add packaging/signing/build automation for macOS and Windows.
5. Trim bundle size now that the desktop app consumes the real shared support UI.
