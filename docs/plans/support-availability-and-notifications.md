# Support availability and agent notifications

> Source review, 2026-09-17

Historical availability design. Current escalation resolves teammate presence and
office hours, chooses a handoff state, and renders state-specific reply-time and
next-opening text in [support_ai_escalate.go](../../server/internal/service/support_ai_escalate.go)
using [handoff helpers](../../server/internal/service/support_handoff_state.go).
The original missing-behavior diagnosis is not the current implementation.
Broader notification/assignment proposals below are not certified by that check.
For schema work, follow the [migration runbook](../ops/database-migrations.md):
Community disables AutoMigrate, so additive columns also require ledger migrations.

**Status:** In Progress
**Date:** 2026-03-24
**Author:** Engineering
**Module:** Support + Notifications
**Inspiration:** Crisp, Intercom

## 1. Why This Exists

Helpin already supports:
- live support conversations
- AI-assisted support flows
- customer email fallback when the customer is offline
- support inbox presence
- per-user notification infrastructure

What is still missing is the operational layer that mature support products expose:
- a clear AI-first support model
- clear human availability / office hours
- customer-facing reply expectations when a conversation needs human follow-up
- teammate email notifications for unread customer replies
- routing decisions that understand who is actually available

This is the gap Crisp and Intercom cover well.

This spec intentionally combines those features into one roadmap, but in a clear order:

1. **AI-first support model**
2. **Human availability**
3. **Wire AI handoff + availability properly into the widget and app**
4. **Unread teammate notifications**
5. **Routing and escalation improvements**

The key product decision is that **AI comes first and human availability comes second**. Without that separation, the product risks leading with "offline" messaging even when AI can help immediately.

## 1.1 Current Implementation Status

### Completed

- Workspace business-hours settings already exist in support chat settings
- Workspace and account notification preferences are now consolidated under `settings/notifications`
- Widget config now exposes computed availability state
- Widget preview/runtime now support AI-first entry copy
- Widget conversation handoff now reveals human availability only after a human request
- Widget/settings preview now reflects business-hours availability
- Support-specific notification controls now exist under `settings/notifications`
- Support notification taxonomy now exists in the shared notification system
- Support customer replies now create in-app teammate notifications for owned conversations
- Delayed fallback email for unread customer replies is now implemented with suppression on read/reply

### Partially complete

- Human availability is implemented for the widget experience, but only the widget handoff path uses the new AI-first reveal rules so far
- Human availability exists as a backend/widget signal, but is not yet used by support inbox operational logic
- AI is now modeled in the widget entry flow as the front door, and backend flow-state persistence now exists, but richer live-status/routing behavior is still not implemented

### Not yet complete

- Human live status model (`online`, `away`, `offline`)
- Team-specific availability overrides
- Availability-aware routing, assignment, and escalation behavior

## 2. External Product Reference

### Crisp

Crisp documents:
- teammate email notifications for unread visitor messages after a **3 minute delay**
- suppression if the message gets read before that delay
- single-operator notification priority
- operator availability scheduling
- routing rules that pick only online agents and can auto-reassign when an assigned operator is offline

### Intercom

Intercom documents:
- default and team-specific **office hours**
- customer-facing reply expectations based on office hours
- teammate email notifications for customer replies after **3 minutes**
- immediate browser/in-app notifications, with email as delayed fallback
- custom office hours for teams and regions

### Product takeaway

The pattern is not "send email when something happens."

The pattern is:
- let AI handle the first response and triage layer
- define when the team is available
- reveal human expectations only when human help is actually needed
- notify teammates only if the message remains unattended
- route responsibility to one eligible person

## 2.1 Product Principles from Crisp and Intercom

These docs suggest a few concrete product rules that Helpin should follow:

1. **Availability is not the same as notification preference.**
   Availability determines whether a team is considered online and what the customer sees.
   Notification preferences determine how a teammate gets alerted.

2. **Office hours are workspace/team constructs, not per-teammate schedules.**
   Intercom explicitly supports workspace defaults plus team-specific office hours and reply times, but not teammate-level office hours.

3. **Reply expectations should be configurable and shown at the right stage.**
   Intercom supports showing reply expectations before assignment or only after a conversation has been assigned to a team. Their recommended mode is to show them only after team assignment when teams have different hours.

4. **Immediate in-app/browser signals and delayed email serve different purposes.**
   Intercom sends in-app/browser immediately, while email is delayed fallback. Crisp also treats email as fallback, not the primary notification surface.

5. **Routing should use both assignment and online eligibility.**
   Crisp’s docs make it clear that notification and routing behavior prefer the assigned operator, but also consider current online availability.

6. **Availability and presence are not the front door of the product.**
   Crisp and Intercom both care deeply about availability, but the support experience should still separate first-response handling from human staffing signals. For Helpin, that means AI-first entry, with human availability revealed on escalation.

## 3. Goals

- Introduce a first-class AI-first support model in Helpin
- Introduce a first-class human availability model in Helpin
- Let customers know when a human team is available only when human follow-up is actually needed
- Notify one responsible teammate when customer replies remain unread
- Reuse existing Helpin systems where practical
- Keep the system low-noise and operationally understandable

## 4. Non-Goals

- Full workforce management
- Per-teammate shift scheduling in v1
- On-call rotations in v1
- SMS/push/Slack notifications in v1
- Auto-replying from agent notification emails in v1
- SLA policy engine in v1

## 5. What We Were Missing

Compared to Crisp and Intercom, the previous draft was still missing or under-defined:
- explicit AI coverage as a first-class support layer
- explicit availability / office hours as a first-class support feature
- a clean definition of who counts as an available teammate
- customer-facing reply expectations tied to availability
- team-level office hours
- a clear implementation order so notifications are built on top of availability instead of before it
- a clean settings architecture separating delivery preferences from operational availability

## 5.1 Settings Information Architecture

Helpin should model these settings in three distinct layers:

### Account Notifications

Location:
- `settings/notifications`

Purpose:
- how a user wants to be notified across the product

Examples:
- pause all notifications
- email enabled
- digest frequency
- digest time/day
- digest timezone
- badge mode

### Workspace Notifications

Location:
- `settings/notifications`

Purpose:
- which workspace activity from the current workspace is eligible to reach that user

Examples:
- mute this workspace
- assignments
- comments
- mentions
- status changes
- sprint notifications
- in-app vs email by category

Product decision:
- move workspace notification preferences out of `settings/general`
- keep all notification preferences under the notifications surface

### Support Notifications

Location:
- `settings/notifications`

Purpose:
- how a user wants to be notified about support inbox activity

Examples:
- unread customer replies
- assigned support conversations
- mentions in internal notes
- future browser/mobile/sound controls

### Support Availability

Location:
- `settings/chat-general` or a dedicated support/chat availability section

Purpose:
- operational support availability, not personal delivery preference

Examples:
- business hours / office hours
- timezone
- outside-hours message
- team availability overrides
- reply expectations shown in widget

Core rule:
- **Notifications** decides how a user gets alerted
- **Availability** decides whether the human team is considered online and what the customer sees after escalation

## 5.2 Canonical Support State Model

To avoid ambiguity in later phases, Helpin should treat these as distinct concepts:

### AI coverage

Definition:
- whether AI is available to handle first response, triage, information gathering, and resolution attempts

Primary uses:
- widget/app entry experience
- first response expectations
- triage before human involvement
- deciding whether a conversation should be escalated to humans

Product rule:
- AI is the front door of support
- human availability must not be used as the default first-contact experience when AI can still help immediately

### Office-hours availability

Definition:
- whether the workspace or assigned team is considered open for human support based on configured office hours

Primary uses:
- human reply expectations after escalation
- routing eligibility
- fallback recipient selection

### Live presence

Definition:
- whether a teammate is actively connected to Helpin / present in the inbox right now

Primary uses:
- immediate in-app/browser notification suppression
- richer human routing later
- future “active in inbox” behaviors

### Ownership

Definition:
- who is assigned to the conversation or team inbox

Primary uses:
- recipient priority
- routing responsibility
- conversation accountability

### Notification eligibility

Definition:
- whether a teammate should actually receive a given notification

Computed from:
- ownership
- office-hours availability
- live presence
- AI escalation state
- account/workspace/support notification preferences
- DND

Product rule:
- later phases must not use “available” as a vague umbrella term; they must specify which of these signals is being used

## 6. Proposed Rollout Order

### Phase 1: AI-First Support Model

Build the AI-first support model first.

This should introduce:
- AI as the default first-response layer
- explicit human handoff conditions
- customer-facing copy that does not lead with human online/offline state
- queue states that distinguish AI handling from waiting for human
- groundwork for later availability-aware routing and notifications

### Phase 2: Human Availability

Once AI-first flow is explicit, build the human availability system.

This should introduce:
- workspace default office hours
- no team-specific office hours in v1
- support availability computation for human coverage
- customer-facing human reply expectations after escalation
- groundwork for routing and notification suppression
- configurable reply expectations for inside and outside office hours
- explicit support for showing reply expectations either before assignment or only after team assignment

### Phase 3: Wire AI Handoff and Availability Properly Into the Widget and App

Once AI-first and human availability exist as backend capabilities, we need to finish the product wiring.

Today, Helpin already stores business hours in support settings and exposes widget availability, but the behavior is incomplete for the new target model:
- widget currently exposes human availability too early
- AI-first entry and handoff states are not modeled explicitly
- app-side support workflows do not consistently use availability

This phase should add:
- widget/app entry that leads with AI availability
- handoff states that reveal human expectations only when needed
- widget config fields for computed human availability
- widget home and conversation UI that reflects human availability at the right stage
- outside-hours message rendering in the human handoff path
- app-side support surfaces that show availability state where relevant
- backend consumers that rely on the shared availability resolver instead of hardcoded assumptions
- support settings copy and layout that clearly treat business hours as availability, not notification preferences

### Current status

- AI-first entry model: partially done
- Widget config human availability: done
- Widget human availability UI wiring: partially done, with handoff visibility now aligned to AI-first entry
- Settings preview wiring: done
- App-side operational usage: not done
- Availability settings IA rename/cleanup: partially done

### Phase 4: Agent Email Notifications for Unread Customer Replies

After availability exists, build the delayed teammate email notifications:
- 3 minute delay
- suppress if read before the delay expires
- prefer assigned / available teammate
- send to one person only
- model email as fallback after immediate in-app/browser notification surfaces

### Phase 5: Routing and Escalation

After we trust availability and notification behavior:
- improve assignment logic
- use team availability in routing
- support fallback recipient pools
- later consider reminders and SLA-aware escalation

## 7. Recommended Product Plan

If we implement only one thing next, it should be:

## AI-first support model

That is the strongest foundation because it unlocks:
- better entry experience
- cleaner escalation rules
- better widget expectations when human help is actually needed
- better routing
- better notification recipient selection
- cleaner future SLA logic

After that, the next best step is:

## Human availability

That defines scheduled human coverage without confusing it with AI availability.

After those two, the next step should be:

## Wire AI handoff and availability properly into the widget and app

That closes the current product gap and removes the mismatch between AI-first intent and human-availability-first UI.

After those three, the next step should be:

## Unread teammate email notifications

That gives immediate operational value once AI handoff and human availability are fully wired.

After those four, the next step should be:

## Team-aware routing and reply expectations

That means this is the recommended sequence:

1. AI-first support model
2. Human availability
3. Wire AI handoff and availability properly into the widget and app
4. Agent email notifications for unread customer replies
5. Routing + reply expectations refinement
6. Reminders / SLA-style escalation

## 8. Phase 1 PRD: AI-First Support Model

### 8.1 Problem Statement

Today Helpin has AI-assisted support, business hours for the widget, and outside-hours messaging, but it does not yet model support as:
- AI first
- human escalation second
- human availability shown only when relevant

That means:
- the widget can lead with human availability even when AI can help immediately
- AI handoff is not explicit enough in the product model
- later routing and teammate notifications do not have a clean escalation state to build on

### 8.2 Goals

- Define AI as the front door of support
- Define when AI should hand off to humans
- Ensure human availability is only revealed when human help is actually needed
- Keep the first version simple and predictable

### 8.3 Product Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Front door | AI first | Avoids leading with "offline" when AI can still help |
| Human availability at first contact | Hidden by default | Reveal only when escalation or human request makes it relevant |
| Handoff timing | AI escalates on low confidence, explicit human request, or issues requiring human judgment/action | Keeps AI useful without overpromising |
| Queue model | Distinguish AI handling from waiting for human | Needed for clean ops and reporting |
| Human availability dependency | Separate from AI coverage | Prevents office hours from muting AI availability |

### 8.4 User Stories

- As a customer, I want immediate help without having to first interpret whether the human team is online.
- As a customer, when AI cannot solve my issue, I want clear expectations for human follow-up.
- As an admin, I want AI to handle first response while still respecting human support coverage.
- As the system, I want a clean escalation state so routing and notifications can make better decisions.

### 8.5 Functional Requirements

#### AI-first entry

The customer-facing support entry should:
- lead with AI availability, not human online/offline state
- invite the user to ask a question immediately
- avoid promising human response timing before escalation

#### Human handoff conditions

The system should hand off to humans when:
- AI confidence is low
- the issue requires human judgment or action
- the user asks for a human
- policy/routing rules require human review

#### Escalation states

The system should explicitly distinguish at least:
- `ai_handling`
- `waiting_for_human`
- `queued_for_human`
- `after_hours_queue`
- `assigned_to_human`
- `resolved_by_ai`
- `resolved_by_human`

### 8.6 Acceptance Criteria

- Customer entry leads with AI-first messaging
- Human availability is not shown by default at first contact
- Handoff states exist in the product model
- The product can distinguish AI handling from human-queue states

### Implementation status

- AI-assisted support exists in product: done
- AI-first support model in PRD/system behavior: partially done
- Explicit handoff/queue states: partially done
- Human availability hidden until escalation: partially done

## 9. Phase 2 PRD: Human Availability

### 9.1 Problem Statement

Once AI is the first layer, Helpin still needs a clean model of scheduled human support coverage for:
- human reply expectations
- routing
- teammate notifications
- after-hours handling

### 9.2 Goals

- Define when human support is considered available
- Expose that state to backend logic and later handoff UI
- Support a workspace default plus team overrides
- Keep the first version simple and predictable

### 9.3 Product Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Availability scope | Workspace default + optional team overrides | Supports shared hours with team-specific schedules |
| Per-teammate schedules in v1 | No | Too much complexity too early |
| Timezone basis | Each office-hours config has its own timezone | Needed for team/region support |
| Human visibility in widget | Only after escalation / human request | Matches AI-first product direction |
| Routing use | Availability is advisory in v1, authoritative in later phases | Reduces rollout risk |
| Reply expectation visibility | Support both `before_assignment` and `after_team_assignment`; default to `after_escalation` in AI-first mode | Preserves flexibility without leading with human staffing |
| Dynamic reply times in v1 | No | Manual presets/custom are enough for first rollout |

### 9.4 User Stories

- As a customer, when AI escalates my issue, I want to know when the human team is available and when I should expect a reply.
- As an admin, I want to configure support hours for the workspace.
- As an admin, I want different teams to optionally have different support hours.
- As the system, I want to know whether a support team is currently available so routing and notifications can make better decisions.

### 9.5 Functional Requirements

#### Workspace default office hours

Add support configuration for:
- enabled/disabled
- timezone
- weekday schedule
- default reply expectation while online
- default reply expectation while offline
- reply expectation visibility mode

#### Team-specific office hours

Deferred post-v1.

For now, Helpin should use:
- workspace default office hours
- teammate live status
- teammate manual status overrides

If team-specific office hours are added later, they can override the workspace default for:
- human reply expectation
- routing decisions
- later notification recipient selection

#### Availability computation

The backend should expose a computed human-availability result:

```go
type SupportAvailabilitySnapshot struct {
    IsOnline bool
    NextOnlineAt *time.Time
    ReplyTimeLabel string
    ReplyExpectationVisibility string // before_assignment | after_team_assignment
    Source string // workspace_default
    TeamID *string // reserved for future team override support
}
```

#### Handoff behavior

Once AI escalates, the widget/app should be able to render:
- humans available now
- reply expected in a few minutes / hours / by next business day
- back on Monday / back at 9:00 AM style messaging
- optionally hide team-specific reply expectations until assignment when configured

### 9.6 Data Model

We already have `SupportInboxSettings` with:
- `BusinessHoursEnabled`
- `BusinessHoursTimezone`
- `BusinessHoursSchedule`
- `OutsideHoursMessage`

For the broader availability model, add:

```go
ShowReplyExpectations bool   `json:"show_reply_expectations"`
ReplyExpectationVisibility string `json:"reply_expectation_visibility"` // before_assignment | after_team_assignment
OnlineReplyTimeMode   string `json:"online_reply_time_mode"`   // few_minutes | few_hours | one_day | dynamic | custom
OnlineReplyTimeHours  *int   `json:"online_reply_time_hours,omitempty"`
OfflineReplyTimeMode  string `json:"offline_reply_time_mode"`  // next_business_time | one_day | custom
OfflineReplyTimeHours *int   `json:"offline_reply_time_hours,omitempty"`
```

Deferred post-v1, if needed:

```go
type SupportTeamAvailability struct {
    ID          string
    WorkspaceID string
    TeamID      string
    Enabled     bool
    Timezone    string
    Schedule    string // JSONB
    OnlineReplyTimeMode   string
    OnlineReplyTimeHours  *int
    OfflineReplyTimeMode  string
    OfflineReplyTimeHours *int
    ReplyExpectationVisibility string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}
```

### 9.7 APIs

Add:
- workspace support settings read/write for reply expectation fields
- public widget config field for availability snapshot
- internal resolver that computes effective availability for the workspace

Later, if team overrides are added:
- team availability CRUD endpoints
- resolver support for workspace vs team override

### 9.8 Acceptance Criteria

- Admin can set default support office hours
- Widget/app can show human expectations using computed availability after escalation
- Backend can resolve effective availability for a conversation
- Availability logic is timezone-correct
- Reply expectations can be shown either before assignment or only after team assignment

### Implementation status

- Default support office hours: done
- Team overrides: deferred post-v1
- Widget/app human expectations after escalation: not done
- Backend availability resolution for widget use: done
- Backend availability resolution for conversation/team routing decisions: not done

## 10. Phase 3 PRD: Wire AI Handoff and Availability Properly Into the Widget and App

This phase takes the AI-first model plus human availability model and makes them visible and operational.

### 10.1 Problem Statement

Helpin already has business-hours settings in the support settings UI and some human-availability wiring in the widget, but it is not yet aligned to the AI-first product direction.

That creates a mismatch:
- the widget can lead with human availability before AI escalation
- the outside-hours message is not fully honored in the right handoff context
- support app behavior still does not use availability as a first-class signal

### 10.2 Goals

- Make AI the front door in the widget/app
- Make human availability visible only when it becomes relevant
- Make support availability available to app-side support workflows
- Replace static widget reply copy with AI-first and escalation-aware messaging
- Ensure business-hours settings are actually reflected in the product

### 10.3 Functional Requirements

#### Public widget config

Extend widget config with an availability payload, for example:

```ts
availability: {
  isOnline: boolean
  nextOnlineAt?: string
  outsideHoursMessage?: string
  replyTimeText?: string
  visibilityMode: 'before_assignment' | 'after_team_assignment'
}
```

This payload must be computed server-side from the support availability resolver and used only in the appropriate handoff stage.

#### Widget UI

Update the widget so:
- home view leads with AI-first messaging
- conversation header can show human availability status when appropriate
- static copy like "We typically reply in a few minutes" is replaced by AI-first or escalation-aware text
- outside-hours message is shown when the human team is offline and handoff is needed
- reply expectations can be hidden until team assignment if configured

#### App-side support UI

Update the support app so relevant surfaces can show:
- current availability status
- effective office-hours source where relevant
- a reliable basis for later routing and notification decisions

At minimum, this should be available in:
- support settings preview
- support inbox context where assignment/routing decisions are made

#### Settings IA cleanup

As part of this phase, Helpin should make the settings split explicit:
- keep business hours / office hours in support chat settings
- move workspace notification preference cards out of `settings/general`
- consolidate per-user notification preferences under `settings/notifications`
- reserve support chat settings for operational support configuration

### 10.4 Acceptance Criteria

- widget entry leads with AI-first messaging
- widget config includes computed human availability
- outside-hours message appears in the correct human-handoff context
- static reply-time copy is removed where AI-first or escalation-aware copy should be used
- app-side support code can consume the same availability snapshot
- widget respects reply expectation visibility mode

### Implementation status

- widget config includes computed human availability: done
- widget home view still reflects human availability too early: needs revision
- outside-hours message appears in widget copy: partially done, but not yet tied to explicit human handoff
- static reply-time copy removed from widget home CTA: done
- app-side support code consumes availability snapshot operationally: not done

## 11. Phase 4 PRD: Agent Email Notifications for Unread Customer Replies

This is the original unread-teammate-notification feature, but now built on top of availability.

### 11.1 Problem Statement

When a customer sends a new message and no teammate notices it, Helpin should notify one responsible teammate by email after a delay.

### 11.2 Trigger

Schedule a notification candidate when:
- a new `support_message` is created
- `sender_type = customer`
- `is_internal = false`
- message is a normal reply
- conversation is not terminal
- workspace feature is enabled

### 11.3 Delay

Fixed at **180 seconds**

### 11.4 Suppression

Do not send if the conversation is read or replied to by a teammate before the timer fires.

### 11.5 Recipient Selection

One recipient only, in this order:

1. assigned agent, if eligible
2. recent public replying teammate, if eligible
3. teammate in an available team for that conversation
4. fallback eligible teammate

Eligibility:
- has support access
- has email enabled
- not in DND
- opted into support customer reply emails

Signal priority for this phase:
- ownership decides who is preferred
- office-hours availability decides which team/user bucket is eligible
- live presence is used only for suppression and later refinement, not as the only definition of availability

### 11.6 Preferences

Workspace-level switch:

```go
SupportAgentEmailNotificationsEnabled bool `json:"support_agent_email_notifications_enabled"`
```

Per-user preference via existing notification preferences:

```json
{
  "support.customer_reply.in_app": true,
  "support.customer_reply.browser": true,
  "support.customer_reply.email": true
}
```

These controls should live under:
- `settings/notifications` -> support notifications

They should not live under:
- `settings/general`
- support availability / business hours settings

### 11.7 Delivery Model

Reuse existing notification tables and async delivery infrastructure.

Notification channel intent:
- `in_app`: immediate
- `browser`: immediate
- `email`: delayed fallback

### 11.8 Acceptance Criteria

- unread customer replies schedule a delayed email check
- read-before-delay suppresses email
- reply-before-delay suppresses email
- exactly one eligible teammate gets emailed
- assigned agent is preferred
- availability-aware fallback routing is respected

## 12. Phase 5 PRD: Routing and Escalation

Once availability and unread notifications are stable, expand to:
- routing conversations to teams that are currently available
- default fallback recipient pools for unassigned conversations
- reminder/escalation workflows when conversations remain unanswered
- future SLA-aware deadlines

This should be a separate implementation phase, not bundled into the first release.

## 13. What We Should Build Next After AI-First and Availability

If AI-first support model is phase 1 and human availability is phase 2, the next best feature is:

### wire AI handoff and availability properly into the widget and app

Why:
- the current business-hours setup is incomplete without this wiring step
- it closes the user-facing gap immediately
- it makes the availability model trustworthy before downstream systems depend on it

After that:

### agent email notifications for unread customer replies

Why:
- immediately reduces missed customer replies
- uses the availability model for better routing
- delivers operational value faster than SLA or reminder systems

After that, build:

### routing + reply expectations refinement

Then later:

### reminders / SLA-aware escalation

## 14. Recommended Engineering Approach

### AI-first + availability

Build this in two steps:
- model AI-first entry + handoff states
- then expose a single human-availability resolver service

```go
ResolveAvailability(ctx, workspaceID, teamID, atTime) -> SupportAvailabilitySnapshot
```

That resolver becomes the shared dependency for:
- widget config
- support inbox UI
- routing
- teammate notifications
- later SLA logic

### Settings architecture

Refactor settings surfaces so they map to the product model:
- `settings/notifications`
  - account notifications
  - workspace notifications
  - support notifications
- `settings/chat-general` or equivalent support settings
  - support availability
  - widget behavior
  - AI and routing

Specifically:
- remove workspace notification cards from `settings/general`
- keep support availability out of the notifications page
- add support-specific notification controls to `settings/notifications` before or alongside unread teammate email delivery
- model support notifications as a distinct section with channels, not as a single email-only toggle

### Notifications

Once availability exists, implement teammate unread-email notifications by reusing:
- `notifications`
- `notification_events`
- `notification_deliveries`
- existing per-user notification preferences

Do not build a second notification stack under support unless we hit a real limitation.

## 14.1 Implementation Checklist by Module

This section translates the PRD into concrete codebase work.

### Phase 1A: Settings Information Architecture

Status:
- done

Backend:
- [x] keep account-level notification settings on `/user/notification-settings`
- [x] keep workspace-level notification preferences on `/notifications/preferences`
- [x] add support-specific notification preference keys to the existing notification preference model

Frontend:
- [x] move workspace notification preference UI into `settings/notifications`
  - [NotificationSettings.tsx](../../frontend/src/pages/NotificationSettings.tsx)
  - [NotificationPreferencesPanels.tsx](../../frontend/src/components/settings/NotificationPreferencesPanels.tsx)
- [x] remove workspace notification cards from `settings/general`
  - [GeneralTab.tsx](../../frontend/src/components/settings/GeneralTab.tsx)
- [x] add a dedicated `Support Notifications` panel to `settings/notifications`
  - [NotificationPreferencesPanels.tsx](../../frontend/src/components/settings/NotificationPreferencesPanels.tsx)
- [x] ensure settings nav copy reflects the split between notifications and availability
  - `frontend/src/pages/Settings.tsx` (historical path; absent from this checkout)
  - [routes/_authenticated/w/$slug/settings/$section.tsx](../../frontend/src/routes/_authenticated/w/$slug/settings/$section.tsx)

### Phase 1B: AI-First + Workspace Human Availability Model

Status:
- partially done

Backend model/service:
- [x] define persisted AI-first/handoff flow state model
  - support conversation/service models under `server/internal/model/` and `server/internal/service/`
- [x] keep workspace business-hours settings on `SupportInboxSettings`
  - [support_inbox.go](../../server/internal/model/support_inbox.go)
- [x] compute widget-facing availability from support settings
  - [support_inbox_settings.go](../../server/internal/service/support_inbox_settings.go)
- [ ] add reply expectation visibility fields
  - [support_inbox.go](../../server/internal/model/support_inbox.go)
- [ ] extract/rename the availability helper into a first-class resolver API
  - [support_inbox_settings.go](../../server/internal/service/support_inbox_settings.go)
- [ ] defer team availability model + repository until post-v1
  - keep workspace-level office hours as the only scheduled-availability source for now

Backend API:
- [x] expose widget availability through the public widget config
  - [support_inbox_widget.go](../../server/internal/service/support_inbox_widget.go)
  - [support_inbox_widget.go](../../server/internal/handler/support_inbox_widget.go)
- [ ] extend support settings PATCH/GET payloads for reply expectation visibility
  - [support_inbox.go](../../server/internal/model/support_inbox.go)
  - [support_inbox.go](../../server/internal/handler/support_inbox.go)
- [ ] skip team availability CRUD endpoints in v1

Tests:
- [x] add focused widget-availability backend tests
  - [support_inbox_availability_test.go](../../server/internal/service/support_inbox_availability_test.go)
- [ ] add API-level tests for reply expectation settings
- [ ] add tests for AI-first handoff state transitions

### Phase 2: Wire AI Handoff and Availability Properly Into the Widget and App

Status:
- partially done

Shared contract:
- [x] add `availability` to widget config types
  - [widget-config.ts](../../packages/shared/src/types/widget-config.ts)
  - [types.ts](../../packages/widget-core/src/types.ts)

Widget UI:
- [x] replace current widget entry with AI-first messaging
  - [HomeView.tsx](../../packages/widget-core/src/components/HomeView.tsx)
- [x] render availability state in conversation header fallback state
  - [ConversationView.tsx](../../packages/widget-core/src/components/ConversationView.tsx)
- [x] add supporting styles
  - [widget.css](../../packages/widget-core/src/styles/widget.css)
- [ ] respect `reply_expectation_visibility`
  - [HomeView.tsx](../../packages/widget-core/src/components/HomeView.tsx)
  - [ConversationView.tsx](../../packages/widget-core/src/components/ConversationView.tsx)
- [x] reveal human availability only after AI escalation / human-request path
  - [HomeView.tsx](../../packages/widget-core/src/components/HomeView.tsx)
  - [ConversationView.tsx](../../packages/widget-core/src/components/ConversationView.tsx)

Settings preview:
- [x] reflect availability in preview config
  - [WidgetPreview.tsx](../../frontend/src/components/settings/WidgetPreview.tsx)
  - [ChatGeneralTab.tsx](../../frontend/src/components/settings/ChatGeneralTab.tsx)

App UI:
- [x] rename `Business Hours` to `Availability`
  - [ChatGeneralTab.tsx](../../frontend/src/components/settings/ChatGeneralTab.tsx)
- [ ] add availability-specific copy and structure in chat settings
  - [ChatGeneralTab.tsx](../../frontend/src/components/settings/ChatGeneralTab.tsx)
- [ ] surface effective availability in support inbox context / assignment UI
  - support inbox components under `frontend/src/components/support/`

Verification:
- [x] frontend typecheck passes for widget/app config changes
- [x] widget-core typecheck passes for availability config changes
- [x] add widget-core rendering tests for AI-first entry and human handoff visibility
  - `packages/widget-core/src/__tests__/`

### Phase 3: Support Notification Taxonomy and Preferences

Status:
- done

Backend model/service:
- [x] define support notification event taxonomy
  - [notification.go](../../server/internal/model/notification.go)
- [x] add support-specific channel preference keys
  - [notification.go](../../server/internal/model/notification.go)
  - [notification_preference.go](../../server/internal/repository/notification_preference.go)
- [x] decide and encode support categories vs exact-event fallbacks in `ShouldNotify`
  - [notification_preference.go](../../server/internal/repository/notification_preference.go)

Frontend:
- [x] add support notification types to notification settings UI
  - [NotificationPreferencesPanels.tsx](../../frontend/src/components/settings/NotificationPreferencesPanels.tsx)
- [x] extend notification type definitions for support channels/keys
  - [notificationTypes.ts](../../frontend/src/lib/notificationTypes.ts)

Tests:
- [x] add `ShouldNotify` coverage for support event/channel preferences
  - `server/internal/repository/notification_preference_integration_test.go`

### Phase 4: Unread Teammate Email Notifications

Status:
- done

Trigger path:
- [x] emit support notification events from customer-message creation paths
  - support message creation service(s) in `server/internal/service/`
  - inbound email path in [email_fallback.go](../../server/internal/service/email_fallback.go)

Delivery logic:
- [x] add delayed support-customer-reply notification job / worker
  - existing notification service in [notification.go](../../server/internal/service/notification.go)
- [x] suppress on team read or teammate reply before delay fires
  - support conversation/message repositories and support services
- [x] select one recipient using ownership + preferences
  - support repos/services + notification preference repo
- [x] send email as delayed fallback after immediate notification channels
  - [notification.go](../../server/internal/service/notification.go)
  - email client under `server/internal/email/`

Frontend:
- [x] add support-notification preference controls before enabling delivery broadly
  - [NotificationPreferencesPanels.tsx](../../frontend/src/components/settings/NotificationPreferencesPanels.tsx)

Tests:
- [x] event emission tests
- [x] recipient-selection tests
- [x] suppression tests
- [x] delivery idempotency tests

### Phase 5: Availability-Aware Routing and Escalation

Status:
- partially done

Backend:
- [x] add automatic human live status model (`online` / `away` / `offline`)
  - websocket presence provider now tracks internal agent connections + last seen
  - support presence/service layer under `server/internal/service/`
- [x] add manual human status overrides on top of automatic presence
  - workspace-scoped per-user override layer now supports `Automatic`, `Online`, `Away`, `Offline`
- [x] extract the current workspace business-hours helper into a first-class availability resolver
- [x] use availability resolver in support routing/assignment decisions
  - support inbox service(s) in `server/internal/service/`
- [x] support fallback recipient pools / available-team routing
  - v1 now supports owner -> configured handoff team -> workspace fallback using workspace office hours plus live/manual status for after-hours exceptional availability
- [ ] keep team-level office-hour overrides deferred until post-v1
- [ ] later add reminder / escalation workflow layer

Frontend:
- [x] expose live owner presence in the support conversation header
  - support inbox components under `frontend/src/components/support/`
- [x] add a self-serve support status picker in the inbox UI
- [ ] expose availability/routing state in broader support inbox assignment surfaces

Open implementation rule:
- do not start this phase before Phase 3 and Phase 4 definitions are stable

## 14. Open Questions

- Should resolved conversations automatically reopen when a customer replies outside office hours?
- Do we want immediate browser notifications as part of the first support notifications release, or email-only first with the data model already prepared for browser later?

## 15. Recommendation

Yes, we should plan this as:

1. **AI-first support model**
2. **Human availability**
3. **Wire AI handoff and availability properly into the widget and app**
4. **Agent email notifications for unread customer replies**
5. **Routing and reply expectation refinement**
6. **Reminders / SLA-style escalation**

That sequence is the cleanest product and engineering path.

## 16. References

- Crisp: How do email notifications work?
  https://help.crisp.chat/en/article/how-do-email-notifications-work-q3gel4/
- Crisp: How can I always appear as online?
  https://help.crisp.chat/en/article/how-can-i-always-appear-as-online-2vmru7/
- Intercom: How teammates get notifications
  https://www.intercom.com/help/en/articles/187-how-teammates-get-notifications
- Intercom: Set your default office hours
  https://www.intercom.com/help/en/articles/732390-set-your-default-office-hours
- Intercom: Set custom office hours and reply times for your teams
  https://www.intercom.com/help/en/articles/3305941-set-custom-office-hours-and-reply-times-for-your-teams
- Intercom: Setting up the Inbox
  https://www.intercom.com/help/en/articles/10223008-setting-up-the-inbox
