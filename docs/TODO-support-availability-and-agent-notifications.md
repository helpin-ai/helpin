# TODO PRD: Support Availability and Agent Notifications

**Status:** In Progress
**Date:** 2026-03-24
**Author:** Engineering
**Module:** Support + Notifications
**Inspiration:** Crisp, Intercom

## 1. Why This Exists

Helpin already supports:
- live support conversations
- customer email fallback when the customer is offline
- support inbox presence
- per-user notification infrastructure

What is still missing is the operational layer that mature support products expose:
- clear team availability / office hours
- customer-facing reply expectations when the team is offline
- teammate email notifications for unread customer replies
- routing decisions that understand who is actually available

This is the gap Crisp and Intercom cover well.

This spec intentionally combines those features into one roadmap, but in a clear order:

1. **Availability**
2. **Wire availability properly into the widget and app**
3. **Unread teammate notifications**
4. **Routing and escalation improvements**

The key product decision is that **Availability comes first**. Without a reliable model of when the team is online, notification routing becomes noisy and arbitrary.

## 1.1 Current Implementation Status

### Completed

- Workspace business-hours settings already exist in support chat settings
- Workspace and account notification preferences are now consolidated under `settings/notifications`
- Widget config now exposes computed availability state
- Widget home and conversation views now render dynamic online/offline availability copy
- Widget/settings preview now reflects business-hours availability
- Support-specific notification controls now exist under `settings/notifications`
- Support notification taxonomy now exists in the shared notification system
- Support customer replies now create in-app teammate notifications for owned conversations
- Delayed fallback email for unread customer replies is now implemented with suppression on read/reply

### Partially complete

- Availability is implemented for the widget experience, but the settings UI is still labeled `Business Hours` rather than a first-class `Availability` section
- Availability exists as a backend/widget signal, but is not yet used by support inbox operational logic

### Not yet complete

- Team-specific availability overrides
- Support-specific notification controls in `settings/notifications`
- Unread teammate email notifications
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
- define when the team is available
- show the customer what to expect
- notify teammates only if the message remains unattended
- route responsibility to one eligible person

## 2.1 Product Principles from Crisp and Intercom

These docs suggest a few concrete product rules that Helpin should follow:

1. **Availability is not the same as notification preference.**
   Availability determines whether a team is considered online and what the customer sees.
   Notification preferences determine how a teammate gets alerted.

2. **Office hours are workspace/team constructs, not per-teammate schedules.**
   Intercom explicitly supports workspace defaults plus team-specific office hours and reply times, but not teammate-level office hours.

3. **Reply expectations should be configurable and optionally delayed until assignment.**
   Intercom supports showing reply expectations before assignment or only after a conversation has been assigned to a team. Their recommended mode is to show them only after team assignment when teams have different hours.

4. **Immediate in-app/browser signals and delayed email serve different purposes.**
   Intercom sends in-app/browser immediately, while email is delayed fallback. Crisp also treats email as fallback, not the primary notification surface.

5. **Routing should use both assignment and online eligibility.**
   Crisp’s docs make it clear that notification and routing behavior prefer the assigned operator, but also consider current online availability.

## 3. Goals

- Introduce a first-class support availability model in Helpin
- Let customers know whether the team is online and when they can expect a reply
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
- **Availability** decides whether the team is considered online and what the customer sees

## 5.2 Canonical Support State Model

To avoid ambiguity in later phases, Helpin should treat these as distinct concepts:

### Office-hours availability

Definition:
- whether the workspace or assigned team is considered open based on configured office hours

Primary uses:
- widget online/offline state
- reply expectations
- routing eligibility
- fallback recipient selection

### Live presence

Definition:
- whether a teammate is actively connected to Helpin / present in the inbox right now

Primary uses:
- immediate in-app/browser notification suppression
- richer routing later
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
- account/workspace/support notification preferences
- DND

Product rule:
- later phases must not use “available” as a vague umbrella term; they must specify which of these signals is being used

## 6. Proposed Rollout Order

### Phase 1: Availability

Build the availability system first.

This should introduce:
- workspace default office hours
- optional team-specific office hours
- support availability computation
- customer-facing "online/offline" and "back later" messaging
- groundwork for routing and notification suppression
- configurable reply expectations for inside and outside office hours
- explicit support for showing reply expectations either before assignment or only after team assignment

### Phase 2: Wire Availability Properly Into the Widget and App

Once availability exists as a backend capability, we need to finish the product wiring.

Today, Helpin already stores business hours in support settings, but the behavior is incomplete:
- settings are editable
- backend has business-hours computation
- but widget config does not expose availability state
- widget UI still uses static reply copy
- app-side support workflows do not consistently use availability

This phase should add:
- widget config fields for computed availability
- widget home and conversation UI that reflects online/offline state
- outside-hours message rendering in the widget
- app-side support surfaces that show availability state where relevant
- backend consumers that rely on the shared availability resolver instead of hardcoded assumptions
- support settings copy and layout that clearly treat business hours as availability, not notification preferences

### Current status

- Widget config availability: done
- Widget availability UI: done
- Settings preview wiring: done
- App-side operational usage: not done
- Availability settings IA rename/cleanup: not done

### Phase 3: Agent Email Notifications for Unread Customer Replies

After availability exists, build the Crisp/Intercom-style delayed teammate email notifications:
- 3 minute delay
- suppress if read before the delay expires
- prefer assigned / available teammate
- send to one person only
- model email as fallback after immediate in-app/browser notification surfaces

### Phase 4: Routing and Escalation

After we trust availability and notification behavior:
- improve assignment logic
- use team availability in routing
- support fallback recipient pools
- later consider reminders and SLA-aware escalation

## 7. Recommended Product Plan

If we implement only one thing next, it should be:

## Availability

That is the strongest foundation because it unlocks:
- better widget expectations
- better routing
- better notification recipient selection
- cleaner future SLA logic

After that, the next best step is:

## Wire availability properly into the widget and app

That closes the existing product gap immediately and makes the availability model real.

After those two, the next step should be:

## Unread teammate email notifications

That gives immediate operational value once availability is fully wired.

After those three, the next step should be:

## Team-aware routing and reply expectations

That means this is the recommended sequence:

1. Availability
2. Wire availability properly into the widget and app
3. Agent email notifications for unread customer replies
4. Routing + reply expectations refinement
5. Reminders / SLA-style escalation

## 8. Phase 1 PRD: Availability

### 8.1 Problem Statement

Today Helpin has business hours for the widget and outside-hours messaging, but it does not yet model support availability as a broader operational concept for:
- team-level support hours
- customer expectations
- routing
- teammate notifications

That means:
- the widget can feel online/offline, but the system does not know who is truly available to receive work
- customer reply expectations are limited
- teammate notifications cannot intelligently prefer the right person

### 8.2 Goals

- Define when support is available
- Expose that state to both backend logic and the widget
- Support a workspace default plus team overrides
- Keep the first version simple and predictable

### 8.3 Product Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| Availability scope | Workspace default + optional team overrides | Matches Intercom’s office-hours model |
| Per-teammate schedules in v1 | No | Too much complexity too early |
| Timezone basis | Each office-hours config has its own timezone | Needed for team/region support |
| Widget behavior | Show online/offline + return timing | Sets customer expectations |
| Routing use | Availability is advisory in v1, authoritative in later phases | Reduces rollout risk |
| Reply expectation visibility | Support both `before_assignment` and `after_team_assignment`; default to `before_assignment` until team routing is live | Matches Intercom’s model without blocking rollout |
| Dynamic reply times in v1 | No | Intercom supports dynamic reply time, but manual presets/custom are enough for first rollout |

### 8.4 User Stories

- As a customer, I want to know when the team is available and when I should expect a reply.
- As an admin, I want to configure support hours for the workspace.
- As an admin, I want different teams to optionally have different support hours.
- As the system, I want to know whether a support team is currently available so routing and notifications can make better decisions.

### 8.5 Functional Requirements

#### Workspace default office hours

Add support configuration for:
- enabled/disabled
- timezone
- weekday schedule
- default reply expectation while online
- default reply expectation while offline
- reply expectation visibility mode

#### Team-specific office hours

If a conversation is assigned or routed to a support team with custom hours, that team’s schedule overrides the workspace default for:
- widget reply expectation
- routing decisions
- later notification recipient selection

#### Availability computation

The backend should expose a computed availability result:

```go
type SupportAvailabilitySnapshot struct {
    IsOnline bool
    NextOnlineAt *time.Time
    ReplyTimeLabel string
    ReplyExpectationVisibility string // before_assignment | after_team_assignment
    Source string // workspace_default | team_override
    TeamID *string
}
```

#### Widget behavior

The widget should be able to render:
- online now
- reply expected in a few minutes / hours / by next business day
- back on Monday / back at 9:00 AM style messaging
- optionally hide team-specific reply expectations until assignment when configured

### 8.6 Data Model

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

For team overrides, add a new model:

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

### 8.7 APIs

Add:
- workspace support settings read/write for reply expectation fields
- team availability CRUD endpoints
- public widget config field for availability snapshot
- internal resolver that computes effective availability for workspace or team

### 8.8 Acceptance Criteria

- Admin can set default support office hours
- Admin can optionally override office hours for a support team
- Widget shows online/offline expectations using computed availability
- Backend can resolve effective availability for a conversation
- Availability logic is timezone-correct
- Reply expectations can be shown either before assignment or only after team assignment

### Implementation status

- Default support office hours: done
- Team overrides: not done
- Widget online/offline expectations: done
- Backend availability resolution for widget use: done
- Backend availability resolution for conversation/team routing decisions: not done

## 9. Phase 2 PRD: Wire Availability Properly Into the Widget and App

This phase takes the availability model and makes it visible and operational.

### 9.1 Problem Statement

Helpin already has business-hours settings in the support settings UI, but they are not fully wired into the public widget config or support app behavior.

That creates a mismatch:
- admins think business hours are active
- the widget does not reliably show online/offline state from those settings
- the outside-hours message is not fully honored in the customer experience
- support app behavior still does not use availability as a first-class signal

### 9.2 Goals

- Make support availability visible in the widget
- Make support availability available to app-side support workflows
- Replace static widget reply copy with availability-aware messaging
- Ensure business-hours settings are actually reflected in the product

### 9.3 Functional Requirements

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

This payload must be computed server-side from the support availability resolver.

#### Widget UI

Update the widget so:
- home view reflects online/offline state
- conversation header can show current availability status when appropriate
- static copy like "We typically reply in a few minutes" is replaced by dynamic availability-aware text
- outside-hours message is shown when the team is offline
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

### 9.4 Acceptance Criteria

- widget config includes computed availability
- widget home view reflects online/offline state from real settings
- outside-hours message appears when the team is offline
- static reply-time copy is removed where availability-aware copy should be used
- app-side support code can consume the same availability snapshot
- widget respects reply expectation visibility mode

### Implementation status

- widget config includes computed availability: done
- widget home view reflects real settings: done
- outside-hours message appears in widget copy: done
- static reply-time copy removed from widget home CTA: done
- app-side support code consumes availability snapshot operationally: not done

## 10. Phase 3 PRD: Agent Email Notifications for Unread Customer Replies

This is the original unread-teammate-notification feature, but now built on top of availability.

### 10.1 Problem Statement

When a customer sends a new message and no teammate notices it, Helpin should notify one responsible teammate by email after a delay.

### 10.2 Trigger

Schedule a notification candidate when:
- a new `support_message` is created
- `sender_type = customer`
- `is_internal = false`
- message is a normal reply
- conversation is not terminal
- workspace feature is enabled

### 10.3 Delay

Fixed at **180 seconds**

### 10.4 Suppression

Do not send if the conversation is read or replied to by a teammate before the timer fires.

### 10.5 Recipient Selection

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

### 10.6 Preferences

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

### 10.7 Delivery Model

Reuse existing notification tables and async delivery infrastructure.

Notification channel intent:
- `in_app`: immediate
- `browser`: immediate
- `email`: delayed fallback

### 10.8 Acceptance Criteria

- unread customer replies schedule a delayed email check
- read-before-delay suppresses email
- reply-before-delay suppresses email
- exactly one eligible teammate gets emailed
- assigned agent is preferred
- availability-aware fallback routing is respected

## 11. Phase 4 PRD: Routing and Escalation

Once availability and unread notifications are stable, expand to:
- routing conversations to teams that are currently available
- default fallback recipient pools for unassigned conversations
- reminder/escalation workflows when conversations remain unanswered
- future SLA-aware deadlines

This should be a separate implementation phase, not bundled into the first release.

## 12. What We Should Build Next After Availability

If availability is phase 1, the next best feature is:

### wire availability properly into the widget and app

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

## 13. Recommended Engineering Approach

### Availability

Build this first in the support settings and support-team model layer, then expose a single resolver service:

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

## 13.1 Implementation Checklist by Module

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
  - [NotificationSettings.tsx](/root/teampulse/frontend/src/pages/NotificationSettings.tsx)
  - [NotificationPreferencesPanels.tsx](/root/teampulse/frontend/src/components/settings/NotificationPreferencesPanels.tsx)
- [x] remove workspace notification cards from `settings/general`
  - [GeneralTab.tsx](/root/teampulse/frontend/src/components/settings/GeneralTab.tsx)
- [x] add a dedicated `Support Notifications` panel to `settings/notifications`
  - [NotificationPreferencesPanels.tsx](/root/teampulse/frontend/src/components/settings/NotificationPreferencesPanels.tsx)
- [x] ensure settings nav copy reflects the split between notifications and availability
  - [Settings.tsx](/root/teampulse/frontend/src/pages/Settings.tsx)
  - [routes/_authenticated/w/$slug/settings/$section.tsx](/root/teampulse/frontend/src/routes/_authenticated/w/$slug/settings/$section.tsx)

### Phase 1B: Workspace Availability Model

Status:
- partially done

Backend model/service:
- [x] keep workspace business-hours settings on `SupportInboxSettings`
  - [support_inbox.go](/root/teampulse/server/internal/model/support_inbox.go)
- [x] compute widget-facing availability from support settings
  - [support_inbox_settings.go](/root/teampulse/server/internal/service/support_inbox_settings.go)
- [ ] add reply expectation visibility fields
  - [support_inbox.go](/root/teampulse/server/internal/model/support_inbox.go)
- [ ] extract/rename the availability helper into a first-class resolver API
  - [support_inbox_settings.go](/root/teampulse/server/internal/service/support_inbox_settings.go)
- [ ] add team availability model + repository
  - `server/internal/model/support_team_availability.go`
  - `server/internal/repository/support_team_availability.go`
- [ ] wire team availability into startup / migrations
  - [main.go](/root/teampulse/server/cmd/api/main.go)

Backend API:
- [x] expose widget availability through the public widget config
  - [support_inbox_widget.go](/root/teampulse/server/internal/service/support_inbox_widget.go)
  - [support_inbox_widget.go](/root/teampulse/server/internal/handler/support_inbox_widget.go)
- [ ] extend support settings PATCH/GET payloads for reply expectation visibility and future team overrides
  - [support_inbox.go](/root/teampulse/server/internal/model/support_inbox.go)
  - [support_inbox.go](/root/teampulse/server/internal/handler/support_inbox.go)
- [ ] add team availability CRUD endpoints
  - [router.go](/root/teampulse/server/internal/router/router.go)

Tests:
- [x] add focused widget-availability backend tests
  - [support_inbox_availability_test.go](/root/teampulse/server/internal/service/support_inbox_availability_test.go)
- [ ] add API-level tests for team availability and reply expectation settings

### Phase 2: Wire Availability Properly Into the Widget and App

Status:
- partially done

Shared contract:
- [x] add `availability` to widget config types
  - [widget-config.ts](/root/teampulse/packages/shared/src/types/widget-config.ts)
  - [types.ts](/root/teampulse/packages/widget-core/src/types.ts)

Widget UI:
- [x] render dynamic availability copy on widget home view
  - [HomeView.tsx](/root/teampulse/packages/widget-core/src/components/HomeView.tsx)
- [x] render availability state in conversation header fallback state
  - [ConversationView.tsx](/root/teampulse/packages/widget-core/src/components/ConversationView.tsx)
- [x] add supporting styles
  - [widget.css](/root/teampulse/packages/widget-core/src/styles/widget.css)
- [ ] respect `reply_expectation_visibility`
  - [HomeView.tsx](/root/teampulse/packages/widget-core/src/components/HomeView.tsx)
  - [ConversationView.tsx](/root/teampulse/packages/widget-core/src/components/ConversationView.tsx)

Settings preview:
- [x] reflect availability in preview config
  - [WidgetPreview.tsx](/root/teampulse/frontend/src/components/settings/WidgetPreview.tsx)
  - [ChatGeneralTab.tsx](/root/teampulse/frontend/src/components/settings/ChatGeneralTab.tsx)

App UI:
- [x] rename `Business Hours` to `Availability`
  - [ChatGeneralTab.tsx](/root/teampulse/frontend/src/components/settings/ChatGeneralTab.tsx)
- [ ] add availability-specific copy and structure in chat settings
  - [ChatGeneralTab.tsx](/root/teampulse/frontend/src/components/settings/ChatGeneralTab.tsx)
- [ ] surface effective availability in support inbox context / assignment UI
  - support inbox components under `frontend/src/components/support/`

Verification:
- [x] frontend typecheck passes for widget/app config changes
- [x] widget-core typecheck passes for availability config changes
- [ ] add widget-core rendering tests for online/offline copy
  - `packages/widget-core/src/__tests__/`

### Phase 3: Support Notification Taxonomy and Preferences

Status:
- done

Backend model/service:
- [x] define support notification event taxonomy
  - [notification.go](/root/teampulse/server/internal/model/notification.go)
- [x] add support-specific channel preference keys
  - [notification.go](/root/teampulse/server/internal/model/notification.go)
  - [notification_preference.go](/root/teampulse/server/internal/repository/notification_preference.go)
- [x] decide and encode support categories vs exact-event fallbacks in `ShouldNotify`
  - [notification_preference.go](/root/teampulse/server/internal/repository/notification_preference.go)

Frontend:
- [x] add support notification types to notification settings UI
  - [NotificationPreferencesPanels.tsx](/root/teampulse/frontend/src/components/settings/NotificationPreferencesPanels.tsx)
- [x] extend notification type definitions for support channels/keys
  - [notificationTypes.ts](/root/teampulse/frontend/src/lib/notificationTypes.ts)

Tests:
- [x] add `ShouldNotify` coverage for support event/channel preferences
  - `server/internal/repository/notification_preference_integration_test.go`

### Phase 4: Unread Teammate Email Notifications

Status:
- partially done

Trigger path:
- [x] emit support notification events from customer-message creation paths
  - support message creation service(s) in `server/internal/service/`
  - inbound email path in [email_fallback.go](/root/teampulse/server/internal/service/email_fallback.go)

Delivery logic:
- [x] add delayed support-customer-reply notification job / worker
  - existing notification service in [notification.go](/root/teampulse/server/internal/service/notification.go)
- [x] suppress on team read or teammate reply before delay fires
  - support conversation/message repositories and support services
- [x] select one recipient using ownership + preferences
  - support repos/services + notification preference repo
- [x] send email as delayed fallback after immediate notification channels
  - [notification.go](/root/teampulse/server/internal/service/notification.go)
  - email client under `server/internal/email/`

Frontend:
- [ ] add support-notification preference controls before enabling delivery broadly
  - [NotificationPreferencesPanels.tsx](/root/teampulse/frontend/src/components/settings/NotificationPreferencesPanels.tsx)

Tests:
- [x] event emission tests
- [x] recipient-selection tests
- [x] suppression tests
- [x] delivery idempotency tests

### Phase 5: Availability-Aware Routing and Escalation

Status:
- not started

Backend:
- [ ] use availability resolver in support routing/assignment decisions
  - support inbox service(s) in `server/internal/service/`
- [ ] support fallback recipient pools / available-team routing
- [ ] later add reminder / escalation workflow layer

Frontend:
- [ ] expose availability/routing state in support inbox assignment surfaces
  - support inbox components under `frontend/src/components/support/`

Open implementation rule:
- do not start this phase before Phase 3 and Phase 4 definitions are stable

## 14. Open Questions

- Should resolved conversations automatically reopen when a customer replies outside office hours?
- Do we want immediate browser notifications as part of the first support notifications release, or email-only first with the data model already prepared for browser later?

## 15. Recommendation

Yes, we should plan this as:

1. **Availability**
2. **Wire availability properly into the widget and app**
3. **Agent email notifications for unread customer replies**
4. **Routing and reply expectation refinement**
5. **Reminders / SLA-style escalation**

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
