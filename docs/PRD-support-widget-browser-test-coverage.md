# PRD: Support And Widget Browser Test Coverage

**Status:** Draft  
**Version:** v1  
**Date:** 2026-03-27  
**Owners:** Support, Frontend Platform, Widget SDK  
**Primary areas:** `frontend/e2e`, `frontend/playwright.support.config.ts`, `packages/sdk-js/test/e2e/widget`, support realtime flows, widget session flows

---

## 1. Context

We have recently hardened two critical live-chat surfaces:

- the internal support inbox in `frontend`
- the customer-facing widget in `packages/sdk-js` and `@helpin/widget-core`

The product risk is not just API correctness. The highest-risk failures are browser-session failures:

- typing state not appearing across agents
- reconnect leaving stale presence behind
- precedence regressions between customer typing, agent typing, unread, and passive viewing
- widget session restore/create drift
- widget unread, link preview, typing, and attachment regressions that only show up in a real browser runtime

We now have deterministic Playwright harnesses for both support and widget. This PRD defines the required browser coverage so these flows remain stable as the product evolves.

This PRD complements, but does not replace:

- [PRD-support-live-chat.md](/root/teampulse/docs/PRD-support-live-chat.md)
- [PRD_WIDGET_SDK_FEATURE_PARITY.md](/root/teampulse/docs/PRD_WIDGET_SDK_FEATURE_PARITY.md)

---

## 2. Problem Statement

Support and widget bugs have recently come from behavior that unit tests did not catch:

- first typing events not being sent
- other agents not seeing live typing
- conversation list precedence regressing while fixing thread presence
- widget bare URLs rendering as plain text
- widget preview cards not rendering after backend enrichment
- reconnect/snapshot handling leaving stale UI state behind

Without a clear product-level test plan, coverage grows reactively and misses important edge cases.

---

## 3. Goals

1. Define the minimum browser-level coverage required for support realtime behavior.
2. Define the minimum browser-level coverage required for widget runtime behavior.
3. Keep the default suites deterministic and fast enough to run locally and in CI when enabled.
4. Separate default mocked browser coverage from opt-in real-backend smoke coverage.
5. Make the required precedence rules explicit so future fixes do not accidentally regress adjacent cases.

---

## 4. Non-Goals

- Replacing unit, integration, or Go service tests
- Turning the default Playwright suites into true end-to-end tests against shared `docker compose` state
- Full visual regression tooling
- Cross-browser parity beyond Chromium in the immediate phase
- Exhaustive load/performance benchmarking

---

## 5. Testing Strategy

### 5.1 Test Layers

We will maintain three browser-testing layers:

1. **Deterministic harness tests**
   - Browser runtime
   - Mocked transport at the Playwright boundary
   - Real React/UI codepaths
   - Primary layer for presence and precedence rules

2. **Full-app mocked route tests**
   - Browser runtime
   - Actual app shell and route tree
   - Mocked API and mocked websocket transport
   - Used to catch route wiring, auth bootstrap, query changes, and layout regressions

3. **Opt-in real-backend smoke tests**
   - Browser runtime against a real API/websocket stack
   - Not default
   - Used only after deterministic coverage is already in place

### 5.2 Default Rule

The default gating path must remain mocked and deterministic.

Reason:

- shared local infra creates flaky state
- seeded users/workspaces can collide
- websocket timing becomes environment-sensitive
- failures become harder to triage

### 5.3 Real Backend Rule

Real-backend browser tests are allowed only as:

- opt-in
- isolated
- explicitly seeded
- explicitly cleaned up or uniquely namespaced

They must never replace the deterministic mocked suites as the main regression harness.

---

## 6. Environments

### 6.1 Default Support Browser Suite

- Config: [playwright.support.config.ts](/root/teampulse/frontend/playwright.support.config.ts)
- App surface:
  - harness: [support-presence-harness.tsx](/root/teampulse/frontend/e2e/support-presence-harness.tsx)
  - full route: [support-full-app.spec.ts](/root/teampulse/frontend/e2e/support-full-app.spec.ts)
- Browser: Chromium only

### 6.2 Default Widget Browser Suite

- App surface:
  - spec: [widget.spec.ts](/root/teampulse/packages/sdk-js/test/e2e/widget/widget.spec.ts)
  - mocks: [widgetE2E.ts](/root/teampulse/packages/sdk-js/test/e2e/widget/widgetE2E.ts)
- Browser: Chromium only

### 6.3 Opt-In Real Backend Suite

Not yet default. When added, it must:

- use explicit environment gating
- use deterministic bootstrap
- avoid reusing shared human/dev data
- prove only a narrow smoke path

---

## 7. Product Rules That Must Be Preserved

### 7.1 Support Presence Precedence

For a conversation row in the support inbox:

1. customer typing wins over all agent states
2. agent typing wins over unread badge and passive viewing
3. unread badge wins over passive viewing
4. passive viewing avatars show only when there is no stronger state
5. when typing stops, the row falls back to the next valid state rather than going blank

### 7.2 Support Thread Rules

- teammate typing must show identity, not anonymous dots only
- multiple typing teammates must preserve identity
- reconnect must request a resync
- authoritative snapshots must replace stale local state, not merge it blindly

### 7.3 Widget Rules

- widget boot must always result in a valid session path
- stored session restore must work on boot and reconnect
- rejected stored sessions must fall back to `session:create`
- bare URLs must render as clickable links
- backend-enriched link previews must render in the thread
- inbound agent activity must affect unread state correctly

---

## 8. Current Coverage Snapshot

### 8.1 Support

Current support browser coverage already exists for:

- agent typing visible to another agent in list and thread
- multiple agents typing together
- customer typing precedence over agent typing and unread
- unread precedence over passive viewing
- passive viewing fallback after typing stops
- reconnect-driven `support:presence:sync`
- authoritative snapshot replacement after reconnect
- full-app route boot with mocked auth, query, and websocket wiring

### 8.2 Widget

Current widget browser coverage already exists for:

- boot and launcher render
- unread badge
- clickable bare URL and preview card rendering
- agent typing identity start/stop
- transcript request flow
- inbound agent messages and unread updates
- stored session restore
- invalid stored session fallback to `session:create`
- attachment upload and send

This PRD formalizes those as minimum required coverage and adds the missing scenarios below.

---

## 9. Support Coverage Matrix

Legend:

- `P0`: must be covered for regression confidence
- `P1`: should be covered in the next hardening pass
- `P2`: opt-in or future hardening
- `Status`: `Existing`, `Planned`, or `Future`

| ID | Priority | Status | Layer | Scenario | Required Assertions |
|----|----------|--------|-------|----------|---------------------|
| S-001 | P0 | Existing | Harness | Agent A typing is visible to Agent B | List row shows `Name is typing…`, typing dots, typing avatar, thread typing identity |
| S-002 | P0 | Planned | Full-app mocked | Typing starts on first compose activity | Browser sees first `support:typing:start` event before updates |
| S-003 | P0 | Planned | Full-app mocked | Typing updates while draft changes | Browser sees `support:typing:update` after start, not repeated starts |
| S-004 | P0 | Planned | Full-app mocked | Typing stops after idle timeout | Browser sees `support:typing:stop`, row clears or falls back appropriately |
| S-005 | P0 | Planned | Full-app mocked | Typing stops on conversation switch | Opening another conversation clears typing for the previous thread |
| S-006 | P0 | Planned | Full-app mocked | Typing stops on composer unmount/navigation | Leaving the route clears presence cleanly |
| S-007 | P0 | Planned | Full-app mocked | Internal note mode suppresses customer-facing typing | `replyMode = note` sends no external typing frames |
| S-008 | P0 | Existing | Harness | Multiple teammates typing together | List shows group avatar cluster, thread shows all visible identities |
| S-009 | P0 | Existing | Harness | Customer typing overrides agent typing and unread | Row renders customer typing treatment only |
| S-010 | P0 | Existing | Harness | Unread overrides passive viewing | Unread badge remains visible, passive viewing avatars hidden |
| S-011 | P0 | Existing | Harness | Agent typing falls back to passive viewing after stop | Typing label disappears and viewing avatar remains |
| S-012 | P0 | Existing | Harness + Full-app mocked | Reconnect requests presence resync | Browser sends `support:presence:sync` and active thread `support:viewing:start` |
| S-013 | P0 | Existing | Harness + Full-app mocked | Authoritative snapshot replaces stale presence | Old typers/viewers disappear, snapshot state becomes the sole source of truth |
| S-014 | P0 | Planned | Full-app mocked | Same user viewing the selected conversation is not duplicated | Current user avatar is not duplicated in viewing stack |
| S-015 | P1 | Planned | Full-app mocked | Mention detection auto-switches to note mode without corrupting presence | UI switches to note mode and external typing is suppressed |
| S-016 | P1 | Planned | Full-app mocked | Sending a teammate reply clears typing and keeps thread stable | Message appears, typing state clears, no stale indicator remains |
| S-017 | P1 | Planned | Full-app mocked | Sending an internal note does not alter customer-facing unread/typing semantics | Note appears only in allowed surfaces and no external typing side effects occur |
| S-018 | P1 | Planned | Full-app mocked | Attachment upload and send do not break typing/presence lifecycle | Upload works, send succeeds, presence clears correctly |
| S-019 | P1 | Planned | Full-app mocked | Support link previews render after teammate or customer URL messages | Autolink is clickable and preview card shows after enrichment |
| S-020 | P1 | Planned | Full-app mocked | Conversation list keeps correct row state while active thread changes | Selecting threads does not leak prior row presence state |
| S-021 | P1 | Planned | Harness | Customer and teammate typing timers expire independently | One actor timing out does not clear another actor’s state |
| S-022 | P1 | Planned | Harness | Presence snapshot with viewer-only data leaves no stale typers | Typers absent from snapshot are removed |
| S-023 | P2 | Future | Real backend smoke | Two authenticated agents share real websocket presence | Agent A typing is seen by Agent B in the real route |
| S-024 | P2 | Future | Real backend smoke | Real reconnect after websocket drop restores presence | Reconnect path issues resync and list/thread state recovers |

### 9.1 Support Exit Criteria

Support browser hardening is considered complete for this phase when:

- all `P0` support cases exist and pass in Chromium
- support harness and full-app mocked route both stay green
- presence precedence rules are explicitly asserted, not implied
- reconnect and snapshot replacement are covered in both harness and full route

---

## 10. Widget Coverage Matrix

| ID | Priority | Status | Layer | Scenario | Required Assertions |
|----|----------|--------|-------|----------|---------------------|
| W-001 | P0 | Existing | Widget mocked browser | Widget boots and launcher renders | `helpin('boot')` results in visible launcher and ready state |
| W-002 | P0 | Planned | Widget mocked browser | Initial boot creates a session when none exists | Browser sends `session:create` and receives joined payload |
| W-003 | P0 | Existing | Widget mocked browser | Stored session restores on boot | Browser sends `session:restore` and thread loads prior conversation |
| W-004 | P0 | Existing | Widget mocked browser | Stored session restores after reconnect | Second socket reconnects and sends restore again |
| W-005 | P0 | Existing | Widget mocked browser | Invalid stored session falls back to create | Browser sends `session:restore`, receives error, then sends `session:create` |
| W-006 | P0 | Existing | Widget mocked browser | Launcher unread badge is correct while widget is hidden | Badge increments and remains visible |
| W-007 | P0 | Existing | Widget mocked browser | Inbound agent message updates unread and thread | Hidden widget shows unread; opened widget shows new message |
| W-008 | P0 | Existing | Widget mocked browser | Agent typing identity renders and clears | Indicator shows avatar/name and disappears on stop |
| W-009 | P0 | Existing | Widget mocked browser | Customer bare URL renders as clickable link | Anchor element exists with correct `href` |
| W-010 | P0 | Existing | Widget mocked browser | Link preview card renders after enriched message echo | Preview title, site, and host render correctly |
| W-011 | P0 | Existing | Widget mocked browser | Transcript request flow works | Correct POST request body is sent and success state is shown |
| W-012 | P0 | Existing | Widget mocked browser | Attachment upload and send works | Initiate, upload, confirm, and final message send all occur |
| W-013 | P0 | Planned | Widget mocked browser | Typing from the visitor sends outbound typing frames | Browser emits typing start/update/stop for customer compose activity |
| W-014 | P1 | Planned | Widget mocked browser | Multiple conversations can be listed and switched | Conversation list renders correctly and active thread changes safely |
| W-015 | P1 | Planned | Widget mocked browser | Session revoke or shutdown clears stored session | Local storage/session state is cleared and next boot creates a new session |
| W-016 | P1 | Planned | Widget mocked browser | Pre-chat requirements are enforced when configured | Email/name gating blocks send until requirements are satisfied |
| W-017 | P1 | Planned | Widget mocked browser | Force identify flow upgrades anonymous session correctly | `session:upgrade` path updates customer identity and preserves history |
| W-018 | P1 | Planned | Widget mocked browser | AI-first / talk-to-human handoff is visible and stateful | Widget shows handoff CTA and resulting state changes |
| W-019 | P1 | Planned | Widget mocked browser | Connection failure UI is visible and recoverable | Widget surfaces failure instead of silently failing |
| W-020 | P1 | Planned | Widget mocked browser | Read state resets unread badge after opening/viewing thread | Badge drops after the thread is read |
| W-021 | P1 | Planned | Widget mocked browser | Inbound preview cards from teammate messages render correctly | Agent-sent enriched URLs render link previews too |
| W-022 | P1 | Planned | Widget mocked browser | Attachment failure path is visible and recoverable | Failed upload shows error UI and does not send broken message state |
| W-023 | P1 | Planned | Widget mocked browser | Transcript failure path is visible | Request failure produces user-visible error state |
| W-024 | P2 | Future | Real backend smoke | Real widget boot against backend creates or restores a session | One smoke path proves live API and websocket contract alignment |

### 10.1 Widget Exit Criteria

Widget browser hardening is considered complete for this phase when:

- all `P0` widget cases exist and pass in Chromium
- the widget suite covers session lifecycle, unread, typing, links, previews, and attachments
- failures in transcript, reconnect, or upload paths are surfaced rather than silently ignored

---

## 11. Harness Requirements

### 11.1 Support Harness Requirements

The support harness must allow us to inject:

- current user identity
- unread count
- viewer state
- agent typing state with identity
- customer typing state
- reconnect events
- authoritative snapshot payloads

The harness must keep using:

- real `useRealtimeSync`
- real presence store
- real list/thread components

### 11.2 Widget Harness Requirements

The widget harness must allow us to control:

- widget config fetch
- websocket frames
- persisted session presence
- stored-session rejection
- transcript requests
- attachment initiate/upload/confirm responses
- inbound message payloads including metadata and preview data

The widget harness must avoid:

- test-only branches in production widget runtime
- dependence on a live test server for standard suite execution

---

## 12. Assertions Policy

Browser tests must prioritize user-visible assertions first:

- visible label text
- badges
- avatars
- typing indicators
- link anchors
- preview cards
- attachment chips
- transcript status

Protocol assertions are still required, but only as support for visible behavior:

- websocket frame type
- request method/path/body
- reconnect frame sequence
- restore/create fallback sequence

We should not overfit tests to internal implementation details like component-local state names.

---

## 13. Data and Isolation Rules

### 13.1 Default Suites

Default suites must:

- run without a real backend
- own all data in-browser or at the Playwright boundary
- not depend on a running shared database
- not depend on pre-existing users, workspaces, or sessions

### 13.2 Real Backend Suites

If real backend smoke tests are added, they must:

- use unique, time-scoped test identities
- use deterministic bootstrap scripts/helpers
- tolerate reconnect timing with explicit waits
- avoid persistent collisions with local dev data
- remain opt-in until proven stable

---

## 14. Recommended Execution Commands

### Support

```bash
pnpm --dir frontend run test:e2e:support:install
pnpm --dir frontend run test:e2e:support
```

### Widget

```bash
pnpm --dir packages/sdk-js exec playwright install chromium
pnpm --dir packages/sdk-js exec playwright test --config=./playwright.widget.config.ts --project=chromium
```

### Build Verification

```bash
pnpm --dir frontend run build
pnpm --dir packages/sdk-js run build
```

---

## 15. Implementation Order

### Phase A

- lock in all `P0 Existing` cases as non-negotiable
- add missing `P0 Planned` support cases around actual composer send/start/update/stop lifecycle
- add missing `P0 Planned` widget case for fresh `session:create`

### Phase B

- add `P1` support cases for notes, mentions, attachments, and link previews
- add `P1` widget cases for pre-chat enforcement, session revoke, failure handling, and handoff

### Phase C

- add one opt-in real-backend support smoke path
- add one opt-in real-backend widget smoke path
- keep both out of the default gating path until stable

---

## 16. Definition Of Done

This PRD is satisfied when:

1. The documented `P0` support and widget cases exist as real Playwright tests.
2. The default support and widget suites remain deterministic and Chromium-stable.
3. The product’s precedence and reconnect rules are explicitly encoded in tests.
4. New regressions in typing, unread, viewing, link previews, session restore, or attachments are caught at the browser layer before release.
5. Any future real-backend browser coverage remains additive and opt-in, not a replacement for the mocked harnesses.

---

## 17. Immediate Next Work

The next highest-value additions from this PRD are:

1. Support full-app mocked tests for composer lifecycle:
   `typing:start`, `typing:update`, `typing:stop`, note-mode suppression, and conversation-switch cleanup
2. Widget tests for fresh `session:create` and outbound customer typing frames
3. Widget failure-path tests for transcript and attachment errors
4. Support and widget opt-in real-backend smoke design, but only after the deterministic matrix above is complete
