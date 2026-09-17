# PRD: Support Inbox Sidebar And Status Model

**Status:** Draft -> Ready for implementation  
**Version:** v1.0  
**Date:** 2026-04-13  
**Owners:** Support, AI Platform, Frontend  
**Primary areas:**
- `frontend/src/components/layout/sidebar/config.ts`
- `frontend/src/stores/supportInboxStore.ts`
- `frontend/src/lib/supportInboxFilters.ts`
- `frontend/src/components/support/ConversationList.tsx`
- `frontend/src/components/support/ConversationRow.tsx`
- `frontend/src/components/support/MessageThread.tsx`
- `server/internal/model/support_inbox.go`
- `server/internal/handler/support_inbox.go`
- `server/internal/repository/support_inbox.go`
- `server/internal/service/support_ai.go`
- `server/internal/service/support_flow_state.go`

---

## 1. Context

The support inbox currently exposes three different state systems in the same UI:

- `status`
- `ai_state`
- `flow_state`

It also mixes queue selection and mailbox selection in the same sidebar interaction model.

That creates a predictable product failure:

- the user sees `Escalated` as a destination
- the backend treats escalation as a handoff into human work
- the frontend often scopes AI views back to shared inbox only
- the resulting screen is empty or misleading

The product question is simple:

When AI escalates a conversation, where does that conversation live?

The current UI does not answer that clearly. This PRD fixes that by separating:

- queue ownership
- lifecycle status
- AI outcome/history
- mailbox scope

---

## 2. Problem Statement

Today the support inbox has four overlapping concepts competing for the same space:

1. `status` is a lifecycle field shown in the dropdown.
2. `ai_state` is used for AI queue filters and unread counters.
3. `flow_state` is the actual operational routing state for AI and human ownership.
4. `mailbox_id` controls routing scope but the sidebar does not make that scope explicit.

This causes four concrete product issues:

1. `Escalated` looks like an inbox, but escalation is actually a transition into human work.
2. `AI Resolved` overlaps with `flow_state = resolved_by_ai`, so the meaning is duplicated and unclear.
3. `Waiting` is vague. It does not tell the team whether the system means waiting on the customer or waiting on internal work.
4. The sidebar resets mailbox scope to shared on nav changes, which can hide valid escalated conversations routed to team inboxes.

---

## 3. Goals

1. Make the sidebar reflect actual queue ownership rather than internal state-machine leakage.
2. Make AI escalation land in an obvious human destination.
3. Keep `Resolved by AI` as a visible concept without duplicating semantics.
4. Clarify the lifecycle dropdown so support agents can understand it without reading implementation details.
5. Surface `flow_state` in the UI through badges and routing rules.
6. Fix mailbox scoping so queue views do not silently hide conversations.
7. Reduce dead or ambiguous state values before more routing features are built.

---

## 4. Non-Goals

- Replacing the underlying support AI execution pipeline
- Rebuilding mailbox routing from scratch
- Introducing a dedicated `After Hours` sidebar queue in v1
- Splitting lifecycle into a large Jira-style state machine
- Converting `ai_state` into a generic historical flag in v1

---

## 5. Decision Lock-Ins

1. `Escalated` is removed as a primary sidebar destination.
2. `Resolved by AI` stays, but it is driven by `flow_state = resolved_by_ai`.
3. `ai_state` remains an AI outcome/history field in v1. It is not repurposed into a vague `ai_involved` boolean.
4. `status = waiting` remains the persisted enum value in v1, but the user-facing label becomes `Waiting on Customer`.
5. `queued_for_human` is deprecated. If historical records exist, the UI treats it as `waiting_for_human` until cleanup is complete.
6. Mailbox scope and queue selection become separate concerns in the sidebar model.

---

## 6. Current-State Review

### 6.1 Current fields and why they conflict

| Field | Current purpose | Real problem |
|---|---|---|
| `status` | ticket lifecycle | `waiting` is ambiguous |
| `ai_state` | AI pending/resolved/escalated filters | used as both queue logic and history |
| `flow_state` | actual operational owner/state | not surfaced in the frontend |
| `mailbox_id` | shared vs team inbox routing | sidebar hides this dimension |

### 6.2 Current operational truth

The backend already treats `flow_state` as the real operational state:

- `ai_handling`
- `waiting_for_human`
- `queued_for_human`
- `after_hours_queue`
- `assigned_to_human`
- `resolved_by_ai`
- `resolved_by_human`

In practice, the active escalation path uses:

- `assigned_to_human`
- `waiting_for_human`
- `after_hours_queue`

This means the frontend is currently hiding the most important field and over-exposing the less important one.

### 6.3 Why `Escalated` feels empty

Current frontend behavior:

- selecting a nav filter resets `selectedMailboxId` to `shared`
- the conversation list always sends `mailbox_id`
- the backend interprets `shared` as `mailbox_id IS NULL`

Current backend escalation behavior:

- escalation may route to the current mailbox
- escalation may route to a triage mailbox
- escalation may route to a configured handoff mailbox
- escalation may set `opened_by_user_id` for a selected human

Result:

- a conversation escalated into a team inbox is valid
- the AI sidebar view scopes back to shared
- the user sees no result and assumes escalation failed

This is a product bug, not just a labeling issue.

### 6.4 Why `Waiting` is not good enough

The current `status = waiting` value is shown as just `Waiting`.

That label is underspecified. In support software, `Waiting` can mean:

- waiting on customer
- waiting on engineering
- waiting on billing
- waiting on vendor

There is no evidence in the current model that `waiting` is used for all of those. The safer interpretation is:

- in v1, `waiting` means waiting on customer

The product should say that explicitly.

### 6.5 Why `queued_for_human` should not survive by accident

`queued_for_human` exists in constants but is not part of the active escalation path.

That is worse than not having the constant at all because it implies a supported state without product semantics.

This PRD treats it as deprecated:

- if no records exist, remove it from active code paths
- if records exist, alias it to `waiting_for_human` in the UI and backfill later

---

## 7. Product Principles

### 7.1 The sidebar shows queues, not transitions

A sidebar item should answer:

- who owns this work now
- or which queue should I check now

It should not expose a past event as if it were a queue.

`Escalated` is a past event. It is not a queue.

### 7.2 Lifecycle and ownership must be separate

The top status dropdown is lifecycle.

The sidebar is queue ownership.

Badges are metadata.

These three layers should not compete.

### 7.3 Mailbox scope must be explicit

Shared inbox and team inboxes are scopes.

They are not queue ownership by themselves.

The user should be able to understand:

- which mailbox scope they are in
- which queue filter they are in

without guessing.

### 7.4 `flow_state` drives placement, `status` drives stage

This is the central design rule.

- `flow_state` decides sidebar placement
- `status` decides lifecycle filtering
- `ai_state` supports AI history/outcome and compatibility logic

---

## 8. Proposed Sidebar Model

### 8.1 Sidebar information architecture

```text
SUPPORT

Scope
  > All inboxes
    Shared Inbox
    Billing
    Sales
    Partnerships

Queues
  > My Inbox
    Unassigned
    Mentions
    All

AI
    AI Active
    Resolved by AI

Status
  [ All statuses v ]
```

This is a conceptual model. The visual design can vary, but the interaction model must preserve these separations:

- mailbox scope
- queue filter
- lifecycle filter

### 8.2 Queue definitions

| Queue | Meaning | Inclusion rule | Exclusions |
|---|---|---|---|
| `My Inbox` | work currently owned by me | human-owned conversations where `opened_by_user_id = current_user_id` | `flow_state = ai_handling`, `flow_state = resolved_by_ai` |
| `Unassigned` | human work with no human owner yet | human-owned conversations where `opened_by_user_id` and `assigned_agent_id` are empty | AI-owned queues |
| `Mentions` | conversations where I was mentioned | mention-based derived view within current mailbox scope | not a routing destination |
| `All` | all human-work conversations in scope | all conversations not currently AI-owned or AI-resolved | `flow_state = ai_handling`, `flow_state = resolved_by_ai` |
| `AI Active` | AI currently owns the next response | `flow_state = ai_handling` | human-owned and AI-resolved conversations |
| `Resolved by AI` | AI completed the conversation without human handoff | `flow_state = resolved_by_ai` | active AI and human queues |

### 8.3 Mailbox scope definitions

| Scope | Meaning |
|---|---|
| `all` | all accessible shared and team inbox conversations |
| `shared` | conversations with no mailbox or explicitly shared routing |
| `mailbox:{id}` | only that mailbox |

Queue selection must not reset mailbox scope.

Mailbox selection may preserve the current queue if the queue is still valid in that scope.

### 8.4 Sidebar naming

Approved labels:

- `My Inbox`
- `Unassigned`
- `Mentions`
- `All`
- `AI Active`
- `Resolved by AI`

Removed labels:

- `All AI`
- `Escalated`

Reason:

- `AI Active` is clearer than `All AI`
- `Resolved by AI` is clearer than `Resolved`
- `Escalated` is not a destination

---

## 9. Canonical State Ownership Model

### 9.1 Field responsibilities

| Field | Owner | Product meaning | Sidebar role |
|---|---|---|---|
| `status` | support lifecycle | open/in progress/waiting/resolved/closed/spam | dropdown filter only |
| `flow_state` | operational owner | AI active, human waiting, assigned, after-hours, resolved-by-AI | primary placement field |
| `ai_state` | AI outcome/history | pending, resolved, escalated | badge/compatibility/analytics |
| `mailbox_id` | routing scope | shared vs team inbox | mailbox scope |
| `opened_by_user_id` | active human owner | who currently owns/picked up the conversation | `My Inbox` logic |
| `assigned_agent_id` | agent assignment metadata | assigned agent when used | secondary human ownership signal |

### 9.2 v1 semantics

#### `status`

Persisted values remain:

- `open`
- `in_progress`
- `waiting`
- `resolved`
- `closed`
- `spam`

User-facing labels become:

- `Open`
- `In Progress`
- `Waiting on Customer`
- `Resolved`
- `Closed`
- `Spam`

Decision:

- do not add `waiting_on_internal` in this PRD
- first make `waiting` explicitly customer-facing everywhere
- if internal wait becomes a real use case later, add a new enum then

#### `flow_state`

Approved meanings:

- `ai_handling`: AI owns the next response
- `waiting_for_human`: needs human pickup, no active owner yet
- `after_hours_queue`: human work deferred by availability policy
- `assigned_to_human`: a human owner has been chosen
- `resolved_by_ai`: AI resolved the conversation
- `resolved_by_human`: a human resolved the conversation

Deprecated:

- `queued_for_human`

v1 treatment:

- if encountered, treat it as `waiting_for_human`
- remove it from new writes

#### `ai_state`

v1 meaning remains:

- `pending`: AI is or was active in the conversation
- `resolved`: AI resolved the conversation
- `escalated`: AI escalated to human

Important rule:

`ai_state` remains useful, but it is not the primary source of sidebar placement.

### 9.3 Preferred field combinations

| Situation | `status` | `flow_state` | `ai_state` |
|---|---|---|---|
| AI currently answering | `open` or `in_progress` | `ai_handling` | `pending` |
| AI resolved successfully | `resolved` | `resolved_by_ai` | `resolved` |
| AI escalated, no owner yet | `open` or `in_progress` | `waiting_for_human` | `escalated` |
| AI escalated after hours | `open` or `in_progress` | `after_hours_queue` | `escalated` |
| AI escalated to a selected human | `open` or `in_progress` | `assigned_to_human` | `escalated` |
| Human-owned conversation | any non-terminal human state | `waiting_for_human` or `assigned_to_human` | `null` or `escalated` |
| Human resolved conversation | `resolved` | `resolved_by_human` | `null` or `escalated` |

---

## 10. Queue Placement Rules

### 10.1 Primary placement algorithm

```text
if flow_state == ai_handling
  -> AI Active

else if flow_state == resolved_by_ai
  -> Resolved by AI

else
  -> Human queues
```

### 10.2 Human queue algorithm

```text
human queue selection
  |
  +-- mentioned current user?
  |      -> visible in Mentions view
  |
  +-- opened_by_user_id == current_user_id?
  |      -> My Inbox
  |
  +-- no opened_by_user_id and no assigned_agent_id?
  |      -> Unassigned
  |
  +-- otherwise
         -> All
```

`Mentions` is a derived view, not the canonical ownership destination.

### 10.3 AI escalation destination rules

This answers the core product question: where does an escalated conversation go?

| Escalation outcome | Required fields | Sidebar destination |
|---|---|---|
| specific human selected | `flow_state = assigned_to_human`, `opened_by_user_id = selected_user` | selected user sees it in `My Inbox`; everyone else sees it in `All` within that mailbox scope |
| no human selected, in office hours | `flow_state = waiting_for_human`, no owner | `Unassigned` within the target mailbox scope |
| no human selected, after hours | `flow_state = after_hours_queue`, no owner | `Unassigned` within the target mailbox scope, with `After Hours` badge |
| escalation routed to shared | `mailbox_id = null` | shared scope |
| escalation routed to team inbox | `mailbox_id = <team mailbox>` | that mailbox scope |

This is the direct fix for the current `Escalated` confusion.

The user should never have to ask:

"Did escalation fail, or am I just in the wrong tab?"

---

## 11. Status Dropdown And Badge Model

### 11.1 Status dropdown

The status dropdown remains lifecycle-only.

Approved options:

- `All statuses`
- `Open`
- `In Progress`
- `Waiting on Customer`
- `Resolved`
- `Closed`
- `Spam`

### 11.2 Row and thread badges

Badges expose operational metadata that should not become sidebar destinations.

Approved badges:

| Badge | Condition | Purpose |
|---|---|---|
| `AI Active` | `flow_state = ai_handling` | makes AI-owned work explicit |
| `Escalated by AI` | `ai_state = escalated` | preserves handoff history |
| `After Hours` | `flow_state = after_hours_queue` | explains delayed human pickup |
| `Resolved by AI` | `flow_state = resolved_by_ai` | explains AI-only closure |
| `Requested Human` | `customer_requested_human_at != null` | explains why AI stopped |

Badge rules:

- badges appear in conversation rows and thread headers
- badges supplement the status pill; they do not replace it
- multiple badges are allowed when the meaning is distinct

Example:

- `Open`
- `Escalated by AI`
- `After Hours`

That is much clearer than a single ambiguous `Escalated` queue.

---

## 12. ASCII Flows

### 12.1 Sidebar interaction flow

```text
user opens support inbox
        |
        v
choose mailbox scope
  all / shared / specific mailbox
        |
        v
choose queue filter
  My Inbox / Unassigned / Mentions / All / AI Active / Resolved by AI
        |
        v
apply lifecycle filter
  All statuses / Open / In Progress / Waiting on Customer / Resolved / Closed / Spam
        |
        v
render conversations
  with row badges from flow_state and ai_state
```

### 12.2 Conversation placement flow

```text
conversation updated
      |
      v
read flow_state
      |
      +--> ai_handling
      |      |
      |      +--> AI Active
      |
      +--> resolved_by_ai
      |      |
      |      +--> Resolved by AI
      |
      +--> waiting_for_human / after_hours_queue / assigned_to_human / resolved_by_human
             |
             +--> apply mailbox scope
             |
             +--> owner = current user?
             |       |
             |       +--> My Inbox
             |
             +--> no owner?
             |       |
             |       +--> Unassigned
             |
             +--> otherwise
                     |
                     +--> All
```

### 12.3 AI escalation flow

```text
AI decides to escalate
        |
        v
resolve target mailbox
  triage mailbox
  or current mailbox
  or configured handoff mailbox
  or shared
        |
        v
resolve human recipient
  available teammate?
        |
        +--> yes
        |     |
        |     +--> flow_state = assigned_to_human
        |     +--> opened_by_user_id = teammate
        |     +--> lands in My Inbox for that teammate
        |
        +--> no
              |
              +--> in office hours?
                    |
                    +--> yes
                    |     +--> flow_state = waiting_for_human
                    |     +--> lands in Unassigned
                    |
                    +--> no
                          +--> flow_state = after_hours_queue
                          +--> lands in Unassigned
                          +--> row shows After Hours badge
```

---

## 13. Functional Requirements

### 13.1 Sidebar behavior

1. Selecting a queue must not silently reset mailbox scope.
2. Selecting a mailbox scope must not silently rename or reinterpret the queue.
3. The default support landing view should be:
   - mailbox scope: `all`
   - queue: `all`
   - status: `all`
4. Old persisted nav values must be migrated safely.

### 13.2 Row behavior

1. Conversation rows must show the lifecycle status pill.
2. Conversation rows must show flow/AI badges when applicable.
3. Escalated conversations must be visually distinguishable inside human queues.

### 13.3 Thread behavior

1. Thread header must show the current mailbox.
2. Thread header must show flow/AI badges.
3. Thread header should make it obvious whether the conversation is:
   - AI-owned
   - human-owned
   - after-hours queued
   - AI-resolved

### 13.4 Unread counts

Unread badges should prioritize operational queues:

- `My Inbox`
- `Unassigned`
- `All`
- `AI Active`

`Resolved by AI` does not need an unread badge in v1.

### 13.5 Backward compatibility

During rollout:

- `ai_all` should map to `ai_active`
- `ai_escalated` should map to `all`
- `queued_for_human` should be rendered as `waiting_for_human`

---

## 14. Backend Requirements

### 14.1 Query model

The backend must support filtering by:

- mailbox scope
- queue semantics
- lifecycle status
- AI/flow metadata

Preferred direction:

- add explicit `flow_state`-aware filtering for list queries
- keep `ai_state` filters temporarily for compatibility where needed

### 14.2 Mailbox scope

The backend must support three mailbox scopes:

- all accessible inboxes
- shared only
- one specific mailbox

The current `mailbox_id` handling that collapses missing scope into shared is not sufficient for the target UI.

### 14.3 Unread stats

Unread counters should move toward:

- human queues driven by non-AI-active `flow_state`
- AI queue driven by `flow_state = ai_handling`
- optional compatibility fallback to `ai_state` during rollout

### 14.4 `waiting` semantics

Backend comments, tests, and API docs must clarify that `status = waiting` means:

- waiting on customer

No persisted enum change is required in v1.

### 14.5 `queued_for_human`

Required work:

1. audit whether records exist
2. stop writing new records
3. alias to `waiting_for_human` in UI and list filters during rollout
4. remove or backfill later

---

## 15. Frontend Requirements

### 15.1 Store model

The current store model should evolve from:

- `navFilter`
- `selectedMailboxId`

to a clearer shape:

- `queueFilter`
- `mailboxScope`
- `statusFilter`

This is a product clarity improvement, not just a refactor.

### 15.2 Sidebar config

Replace current AI items:

- `ai_all`
- `ai_pending`
- `ai_resolved`
- `ai_escalated`

With:

- `ai_active`
- `resolved_by_ai`

`Pending` may survive as a secondary filter only if there is a proven workflow need. It should not remain a top-level queue by default.

### 15.3 Badge rendering

Conversation rows and thread headers must read:

- `flow_state`
- `ai_state`
- `customer_requested_human_at`

and render the approved badges consistently.

### 15.4 Local storage migration

Persisted sidebar state must migrate old values safely:

| Old value | New value |
|---|---|
| `ai_all` | `ai_active` |
| `ai_escalated` | `all` |
| `selectedMailboxId = shared` legacy default | `mailboxScope = shared` only when the user explicitly chose shared |

---

## 16. Suggested Implementation Order

1. Add `flow_state` and AI badges to conversation rows and thread headers.
2. Split mailbox scope from queue filter in the store and query layer.
3. Stop resetting mailbox scope on nav changes.
4. Rename sidebar labels:
   - `All AI` -> `AI Active`
   - `Resolved` -> `Resolved by AI`
5. Remove `Escalated` as a sidebar destination.
6. Rebuild AI queue filters around `flow_state`.
7. Rename `Waiting` to `Waiting on Customer` in all user-facing labels and backend comments/tests.
8. Audit and deprecate `queued_for_human`.
9. Consider an `After Hours` queue later only if badge usage proves insufficient.

---

## 17. Success Metrics

1. Fewer support-team questions about where escalated conversations went.
2. Reduced empty-state visits to the former escalation view.
3. Faster pickup time for AI-escalated conversations.
4. Lower mismatch between mailbox routing and sidebar visibility.
5. Reduced product ambiguity in user interviews and dogfooding.

---

## 18. Risks And Mitigations

### Risk: removing `Escalated` surprises current users

Mitigation:

- add clear `Escalated by AI` badges
- migrate old nav state to `All`
- update release notes and internal demo script

### Risk: `ai_state` and `flow_state` drift during rollout

Mitigation:

- treat `flow_state` as source of truth for placement
- keep compatibility checks on `ai_state`
- add targeted tests for allowed state combinations

### Risk: mailbox scope refactor introduces list regressions

Mitigation:

- add focused frontend store tests
- add backend list query tests for all/shared/mailbox scopes

---

## 19. Final Product Decision

When AI escalates a conversation, the conversation leaves the AI queue.

It should land in exactly one of these places:

- `My Inbox` if a specific human was selected
- `Unassigned` if no human owner exists yet
- `Unassigned` with `After Hours` badge if availability policy deferred pickup

It should appear inside the correct mailbox scope:

- shared inbox
- or the routed team inbox

`Escalated` remains visible as history through badges and analytics, not as a primary sidebar destination.
