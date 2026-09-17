# Support system-message event types

> Historical plan, source-compared on 2026-09-17. This document explains the
> original typed-message rollout for contributors. Typed persistence and transport
> exist, but removal of the admin legacy keyword fallback remains incomplete.

## Current implementation and remaining differences

The [backend constants](../../server/internal/model/support_system_event.go)
now cover additional events, including mailbox moves, delayed replies, email
recipient changes, tags, and task creation. Validation is named
`IsValidSupportSystemEventType`, not the proposed `IsValidSystemEventType`.
The [shared frontend tuple](../../packages/shared/src/types/message.ts) is narrower:
it omits backend values such as `customer_requested_human`, `email_recipients_updated`,
`tag_added`, `tag_removed`, and `task_created`. It is not a complete cross-layer
single source of truth as the original plan describes.

The actual migrations are
[202604150004_support_message_system_event_type.sql](../../server/internal/dbmigrate/sql/202604150004_support_message_system_event_type.sql)
and [202604150005_backfill_support_message_system_event_type.sql](../../server/internal/dbmigrate/sql/202604150005_backfill_support_message_system_event_type.sql).
The backfill leaves unclassified rows null. The
[admin renderer](../../frontend/src/components/support/MessageBubble.tsx) still
contains the explicitly temporary keyword-matching branch; migration files alone
do not prove deployment or completion of the removal pass.

Visibility is no longer governed solely by `is_internal`.
[Backend widget visibility](../../server/internal/model/support_delivery.go),
the [SDK](../../packages/sdk-js/src/core/widget.ts), and the
[widget renderer](../../packages/widget-core/src/components/MessageBubble.tsx)
also restrict explicit event types to `teammate_joined` and `delayed_team_reply`.
The first renders as a pill; delayed replies render through the normal message
path. Other explicit event types are hidden even when sender context exists,
contrary to the original general fallback description. A legacy system message
without an event type can reach the normal bubble path.

[Websocket mapping](../../server/internal/websocket/support_events.go) carries
the event field. The [service test source](../../server/internal/service/support_message_system_event_test.go)
checks repository rejection of missing/invalid system types and named constants;
that invariant is not an exhaustive execution of every future emitter or every
possible database writer. No live null-row audit, migration application, or new
runtime test run was performed for this documentation comparison.

## Original rollout plan

## Problem
Today the admin `MessageBubble` keyword-matches content ("joined", "assigned", "resolved") to pick a render style. That breaks under i18n, copy tweaks, or third-party callers. The widget does the same implicitly via the `role === 'system'` path. We need a first-class `system_event_type` on support messages so each surface can branch on intent instead of prose.

## End State (non-negotiable)
Content-keyword matching is **migration-only** and removed after the backfill lands and rollout is stable. The final architecture renders system messages by explicit `system_event_type` in both admin and widget, with `is_internal` governing visibility.

## Event Taxonomy
Single source of truth, stored on the message row:

| event_type          | Fires from                                               | Admin style | Widget style |
|---------------------|----------------------------------------------------------|-------------|--------------|
| `teammate_joined`   | First non-internal reply by a given user | muted pill  | **flat system-message pill (only widget-visible system event)** |
| `assigned`          | `assignConversationUser` cross-assign                    | muted pill  | none (is_internal) |
| `unassigned`        | `assignConversationUser` with nil user                   | muted pill  | none (is_internal) |
| `took`              | `assignConversationUser` self-assign                     | muted pill  | none (is_internal) |
| `agent_assigned`    | `assignConversationAgent`                                | muted pill  | none (is_internal) |
| `triage_routed`     | `SupportInboxTriageService` auto-move                    | muted pill  | none (is_internal) |
| `triage_dismissed`  | Triage suggestion dismissed                              | muted pill  | none (is_internal) |
| `resolved`          | Conversation resolved                                    | slate pill  | none (Intercom does not surface these) |
| `reopened`          | Conversation reopened                                    | slate pill  | none |
| `closed`            | Conversation closed                                      | slate pill  | none |

`teammate_left` is **dropped** — redundant with `unassigned` on admin, and widget never surfaces it.

Visibility is governed by `is_internal`. `system_event_type` only describes *what* happened, not *who sees it*.

## Constants (centralized)
All event types are defined as named constants in one place on each layer:
- Backend: `internal/model/support_system_event.go` — `SystemEvent...` string constants + an exhaustive `IsValidSystemEventType(string) bool`.
- Shared: `packages/shared/src/types/message.ts` — `SystemEventType` union + `SYSTEM_EVENT_TYPES` const tuple.
- Widget / admin: import the shared union. No freeform string literals in emitters or renderers.

## Backend Changes
1. **Migration** (`internal/dbmigrate/sql/2026041501_support_message_event_type.sql`):
   - `ALTER TABLE support_messages ADD COLUMN IF NOT EXISTS system_event_type VARCHAR(40)`
   - `CREATE INDEX IF NOT EXISTS support_messages_conversation_event_idx ON support_messages(conversation_id, system_event_type) WHERE system_event_type IS NOT NULL`
   - Optional backfill: derive event_type from existing content via keyword match, one-shot.

2. **Model** (`internal/model/support_inbox.go`): add `SystemEventType *string `json:"system_event_type,omitempty"``. Extend `WidgetMessageReceivedPayload` with the same field.

3. **Service layer** — every system-message emitter sets the field:
   - `emitAssignmentSystemMessage`: `assigned` / `unassigned` / `took` / `agent_assigned` based on `assignmentTargetKind`.
   - `emitTeammateJoinedIfFirstReply`: `teammate_joined`.
   - `SupportInboxTriageService.createSystemMessage`: caller passes in `triage_routed` or `triage_dismissed`.
   - Resolve/reopen/close emitters (find + audit): `resolved`, `reopened`, `closed`.

4. **WebSocket payload** (`internal/websocket/support_events.go`): include `SystemEventType` in `WidgetMessageReceivedPayload`.

## Frontend Changes
1. **Shared `Message` type** (`packages/shared/src/types/message.ts`): add `systemEventType?: SystemEventType` (union of the constants). The field lives on the core shared model — not on a transport-only DTO — so the UI doesn't depend on raw payload shape.

2. **SDK `mapSupportMessage`** (`packages/sdk-js/src/core/widget.ts`): map `raw.system_event_type` → `message.systemEventType`.

3. **Widget MessageBubble** (`packages/widget-core/src/components/MessageBubble.tsx`):
   - `role === 'system' && systemEventType === 'teammate_joined'` → render the flat system-message pill.
   - Any other `role === 'system'`:
     - if `is_internal` — don't render (shouldn't arrive anyway; belt + braces).
     - else if the message has sender context (name/avatar) — render as a normal incoming bubble (avoids the handoff regression where a legitimate system-ish message collapsed to nothing).
     - else — skip rendering.

4. **Admin MessageBubble** (`frontend/src/components/support/MessageBubble.tsx`):
   - Dispatch on `message.system_event_type`. Three groups: routing (muted pill), state (slate pill), legacy (content-keyword fallback for pre-migration rows only).
   - The legacy branch is annotated `// TODO: remove after system_event_type backfill rollout completes — see plan 2026-04-15` so it can't quietly become permanent.

## Test Coverage
- `support_events_test.go`: payload round-trip includes `system_event_type`.
- **Exhaustive emitter test**: one table-driven test asserts every emitter that produces `message_type='system'` also sets a non-empty, valid `system_event_type`. New emitters added later will fail this test until they register their event type.
- Service tests: `emitAssignmentSystemMessage` sets each target type correctly; `emitTeammateJoinedIfFirstReply` sets `teammate_joined` exactly once per user per conversation.
- Widget test: system message with `systemEventType='teammate_joined'` renders the flat pill; other values with sender context render as a normal bubble; internal system messages don't render.
- Admin test: each `system_event_type` picks the right renderer. Legacy-fallback branch has its own test so it doesn't rot silently.

## Rollout
1. Ship migration (idempotent, nullable column, no drops).
2. Ship backend writes — every emitter sets `system_event_type`.
3. Ship frontend readers with legacy-content fallback clearly marked temporary.
4. **One-time backfill migration** that classifies historic rows by content with high-confidence keyword rules; ambiguous rows stay NULL and fall through the legacy path.
5. Monitor: after rollout, confirm no system rows in the last N days have NULL `system_event_type`.
6. **Removal pass**: delete the legacy content-match branch in admin `MessageBubble` and the plan TODO comment. File a follow-up ticket now so it's tracked.

## Decisions (from review)
- **Resolve / reopen / close on widget**: not shown — Intercom does not surface them either. Out of scope.
- **`teammate_left`**: dropped. `unassigned` covers the admin case; widget never needs it without a fuller presence model.
- **Historic backfill**: yes, one-time, with fallback purely transitional.
- **Shared type placement**: `systemEventType` belongs on the shared `Message` type, not a transport-only DTO.
