# Availability-Aware AI → Human Escalation

> Source review, 2026-09-17

Historical availability design. Current escalation resolves teammate presence and
office hours, chooses a handoff state, and renders state-specific reply-time and
next-opening text in [support_ai_escalate.go](../../server/internal/service/support_ai_escalate.go)
using [handoff helpers](../../server/internal/service/support_handoff_state.go).
The original missing-behavior diagnosis is not the current implementation.
Broader notification/assignment proposals below are not certified by that check.
For schema work, follow the [migration runbook](../ops/database-migrations.md):
Community disables AutoMigrate, so additive columns also require ledger migrations.

**Date:** 2026-07-02
**Status:** Approved design, pending implementation plan
**Owner:** Support module

## Problem

When the support AI escalates a conversation, it always says the same thing —
"Let me connect you with a team member who can help further." — regardless of
whether anyone can actually respond. If no teammate is online, or it is outside
business hours, the customer is left waiting with a false expectation and no
way to be reached later. The backend already knows the answer at escalation
time (it silently sets `flow_state = after_hours_queue`), but the customer is
never told.

## Goals

- Never over-promise: the escalation message must reflect real availability.
- Set concrete expectations: reply time when live/busy, return time when away.
- Close the loop for offline visitors via email capture and reply-by-email.
- Surface availability *before* the customer invests in a conversation.
- Give teammates visibility into conversations queued while nobody was around.

## Non-Goals

- Per-mailbox business hours (workspace-wide hours remain, as today).
- Holiday/date-override schedules.
- SLA automation (auto-email if no human reply within X). Possible follow-up.
- Changing how escalation is *detected* (heuristics in
  `support_ai_escalation.go` are untouched).

## Current State (verified 2026-07-02)

| Piece | Where | Status |
|---|---|---|
| Escalation orchestration | `server/internal/service/support_ai.go` (`escalateToHuman`, ~L959) | Exists; message hardcoded/overridable via single `EscalationMessage` setting (L1019-1022) |
| Business hours | `SupportInboxSettings` in `server/internal/model/support_inbox.go` (L1058-1062): `BusinessHoursEnabled`, `BusinessHoursTimezone`, `BusinessHoursSchedule`, `OutsideHoursMessage` | Exists; JSONB on `support_widget_installations.settings`; disabled by default |
| Hours enforcement | `support_availability_resolver.go`, `isWithinBusinessHours` / `nextBusinessHoursStart` in `support_inbox_settings.go` (L449-468) | Exists; used at escalation to set `after_hours_queue` |
| Teammate presence | `websocket/presence_provider.go`, `redis_presence.go`; `resolveSupportTeammatePresenceStatuses` → online/away/offline + manual overrides | Exists |
| Conversation flow states | `SupportConversation.FlowState`: `ai_handling`, `waiting_for_human`, `queued_for_human`, `after_hours_queue`, `assigned_to_human`, `resolved_by_ai`, `resolved_by_human` | Exists |
| Widget availability object | `packages/widget-core/src/types.ts` L17-24: `isOnline`, `replyTimeText`, `outsideHoursMessage`, `nextOnlineAt`, etc. | Exists; not driven by escalation outcome |
| Email fallback | `EmailFallbackEnabled` setting + `email_fallback.go` + `WidgetTranscriptRequest` | Exists; not offered at escalation |
| Settings UI | `frontend/src/components/settings/ChatGeneralTab.tsx` (business-hours section, escalation message field) | Exists |

## Design

### 1. Handoff state resolution (backend)

At the moment `escalateToHuman` executes, resolve a single
**`handoff_state`** using the existing availability resolver:

| State | Condition |
|---|---|
| `live` | ≥1 teammate effectively online (presence status `online`, after manual overrides; `away` does not count) |
| `after_hours` | Nobody online AND business hours enabled AND outside schedule |
| `busy` | Nobody online AND (within hours OR hours disabled) |

Resolution order: presence first, then hours. **Presence trumps hours** — an
agent online outside scheduled hours yields `live`.

The resolved state is:
- persisted on the conversation (drives `flow_state`: `live` →
  `waiting_for_human`/`assigned_to_human` per existing routing; `busy` →
  `queued_for_human`; `after_hours` → `after_hours_queue`),
- recorded on the `AgentHandoff` analytics row,
- included in the escalation WebSocket event payload.

The state is resolved **once** at escalation. Later presence changes are
handled by re-engagement (§4), not by rewriting the message.

### 2. Per-state escalation messages (settings)

`SupportInboxSettings` gains two fields alongside the existing one:

| Field | Used for | Default |
|---|---|---|
| `EscalationMessage` (existing) | `live` | "Let me connect you with a team member — they typically reply in {reply_time}." |
| `EscalationMessageBusy` (new) | `busy` | "I've notified the team. Everyone's helping other customers right now — expect a reply within {reply_time}." |
| `EscalationMessageAfterHours` (new) | `after_hours` | "I've passed this on to the team. We're away right now and back {next_open}." |

**Backward compatibility:** an existing custom `EscalationMessage` keeps
applying to the `live` state only. Empty new fields fall back to the defaults
above (not to the live message — a custom "connecting you now" must never
render while nobody is available).

**Template tokens**, substituted server-side before the message is stored as
the AI's chat message:

- `{reply_time}` — humanized from the mailbox `ReplyTimePreset` /
  `ReplyTimeMinutes` (e.g. "a few minutes", "1 hour"). If unset: "as soon as
  possible" and surrounding copy still reads naturally.
- `{next_open}` — humanized next business-hours start in the workspace
  timezone with timezone label (e.g. "tomorrow at 9:00 AM PST", "on Monday at
  9:00 AM CET"), from `nextBusinessHoursStart`. If no next open time exists
  (hours disabled, or every day disabled), substitute "as soon as possible".

Unknown tokens render literally (no error). Token substitution lives in one
service-level helper with unit tests.

### 3. Widget experience

The escalation WS event and conversation snapshot carry:

```
handoff_state: "live" | "busy" | "after_hours"
next_online_at: string | null      // ISO, for after_hours
expected_reply_text: string | null // pre-rendered reply-time copy
```

Widget behavior (`packages/widget-core`, `ConversationView`):

- **live** — escalation message renders as today, plus the existing
  human-handoff state shows a "a team member is joining" indicator with the
  online presence dot.
- **busy / after_hours** — escalation message renders; beneath it, if the
  visitor has no known email, an inline **email-capture card**: "Leave your
  email and we'll reply there too." Submitting stores the email on the
  conversation contact (reusing the email-fallback plumbing) and confirms:
  "We'll reply to you at {email}." Dismissable; replies still appear in the
  widget either way.
- **Pre-chat transparency** — the widget `availability` object
  (`isOnline`, `nextOnlineAt`, `statusText`) is corrected to reflect the
  *combined* resolution (presence AND hours) rather than hours alone, so the
  home view and conversation header set expectations before escalation ever
  happens. Same resolver, same rules.

### 4. Re-engagement (closing the loop)

- When a teammate is assigned or sends their first reply, the widget shows a
  system line: "{Name} joined the conversation" (avatar + name), driven by the
  existing assignment/reply WS events.
- If the visitor is no longer connected and an email is known, the human reply
  also goes out by email (reply-by-email via existing email fallback), so the
  after-hours promise "we'll reply there too" is kept.

### 5. Teammate-side visibility

- Inbox list surfaces `queued_for_human` and `after_hours_queue` conversations
  with a "Waiting for human" badge and waiting-since time.
- Escalation notifications to the mailbox team fire for `busy` and
  `after_hours` states too (not only when routing assigns someone), so queued
  conversations are seen, not discovered.

### 6. Settings UX (`ChatGeneralTab`)

- The escalation section shows three message fields (Live / Busy /
  After-hours) with token hints and a rendered preview per state using current
  settings (real reply-time and next-open values).
- If business hours are disabled, an inline nudge under the After-hours field:
  "Enable business hours so customers see an accurate return time." linking to
  the business-hours section.

## Edge Cases

- **Presence staleness:** `away` (past `supportTeammateAwayThreshold`) counts
  as not available; manual overrides respected — matches existing resolver.
- **Hours enabled, all days disabled:** always `after_hours` when nobody is
  online; `{next_open}` degrades to "as soon as possible".
- **Agent goes offline right after a `live` escalation:** state is not
  re-resolved; routing already uses `RequireAvailability`, and re-engagement
  (§4) plus reply-by-email cover the gap. Accepted trade-off for v1.
- **Visitor already identified (known email):** no capture card; busy /
  after-hours copy appends "We'll also reply to you by email."
- **Timezone rendering:** `{next_open}` uses the workspace business-hours
  timezone with an explicit label; no attempt to guess visitor timezone in v1.

## Phasing

1. **Phase 1 — honest escalation:** state resolution + persisted
   `handoff_state`, two new settings fields + token rendering, escalation
   event payload, widget three-state rendering + email-capture card.
2. **Phase 2 — first-class polish:** "joined" system line, reply-by-email on
   human reply, teammate queue badge + notifications, settings previews and
   business-hours nudge, corrected pre-chat availability.

## Testing

- Unit: state-resolution matrix (presence × hours-enabled × in/out of hours ×
  manual overrides), token substitution (all tokens, missing values, unknown
  tokens), backward-compat fallback of message fields.
- Widget (vitest/jsdom): three escalation states, email-capture submit and
  confirm, identified-visitor variant.
- Integration: after-hours escalation → `after_hours_queue` + correct message
  + email captured; busy escalation fires team notification.
