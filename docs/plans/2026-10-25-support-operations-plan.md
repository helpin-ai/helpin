# Support operations: measurement, SLAs, inbox productivity, integrations

Date: 2026-10-25. Branch: `feat/support-operations`.

This plan fills the operational gaps a support team notices in its first week on
Helpin: there is no way to measure the team, no response-time targets, several
inbox basics are missing, and nothing outside Helpin can react to support
activity. Each phase ships independently, is tested on its own, and leaves the
product working if later phases never land.

Shared Community scope. Nothing here touches billing. SLA policies stay behind
the existing `sla_policies` entitlement, which Community grants by default.

## Principles

- Derive metrics from the source tables (`support_conversations`,
  `support_messages`) rather than from best-effort event logs, so reports are
  correct for history that predates this work.
- Do not change customer-facing semantics without a reason. Snooze is a
  teammate-side state; the customer still sees an open conversation.
- Every timer that matters (SLA due times, snooze wake-ups, webhook retries) is
  driven by a durable row and a sweeper, never by an in-memory timer.
- Respect mailbox access everywhere: reports, exports, bulk actions and
  webhooks never expose conversations a teammate cannot open.

## Phase 1: Measurement (CSAT and support reports)

### CSAT

- New table `support_csat_responses`: one row per conversation (unique),
  rating 1-5, optional comment (1,000 chars), channel (`widget`, `portal`),
  `resolved_by` (`ai` or `human`) and the assigned teammate at submission time.
  Customers may change their rating while the conversation stays resolved; the
  row is updated, not duplicated.
- The submission adds an internal timeline entry (system event `csat_rated`)
  so teammates see the rating and comment in the conversation.
- Widget: `POST /widget/conversations/{id}/csat` authenticated by the widget
  session token, allowed only for the session's own resolved conversation and
  only when CSAT is enabled. The widget learns whether a rating exists from the
  server, not only from `localStorage`.
- Portal: `POST /requests/{id}/csat` on the portal API with the same rules; the
  request page shows the rating card once a request is resolved.
- Settings: the CSAT switch in Messenger settings becomes functional.
- The CRM `support_csat_deterioration` signal reads the new table instead of
  `csat_survey` messages, which nothing ever wrote.

### Reports

- `GET /api/workspaces/{ws}/support/reports/overview?from&to&tz&mailbox_ids`
  returns summary metrics with the previous equal-length period, a daily series,
  a channel breakdown, a teammate breakdown, CSAT distribution and recent
  comments.
- `GET .../support/reports/conversations.csv` exports one row per conversation
  in the range with the same derived fields.
- Metric definitions (documented in `docs/support-reports.md`):
  - Conversations: created in range, excluding spam and conversations merged
    into another.
  - First response time: from the first customer message to the first public
    teammate reply, for conversations the customer started. Median and p90.
  - Time to resolution: from creation to `resolved_at` for conversations
    resolved in range. Median and p90.
  - AI involvement: conversations with at least one public AI reply.
  - AI resolution rate: AI-resolved conversations over AI-involved
    conversations, split into confirmed and assumed.
  - Handoff rate: AI-involved conversations escalated to a teammate.
  - CSAT: average, count and share of 4-5 ratings, split by AI and human.
- Aggregation happens in Go over per-conversation fact rows, which keeps one
  implementation for Postgres, SQLite tests and the CSV export. The range is
  capped at 366 days.
- Frontend: `/support/reports` with range presets, an inbox filter, KPI cards
  with period deltas, volume and response-time charts, AI and CSAT sections,
  the teammate table and CSV export.

## Phase 2: Priority, snooze and SLAs

### Priority

- `PUT /inbox/conversations/{id}/priority` with an internal timeline event and
  a realtime update. Priority becomes editable in the conversation sidebar.

### Snooze

- Columns `snoozed_until` and `snoozed_by_user_id` on conversations.
- `POST /inbox/conversations/{id}/snooze` and `DELETE` to wake early.
- A snoozed conversation leaves Inbox, Mine, Unassigned and Waiting (lists and
  counters, including the materialized counter functions) and appears in a new
  Snoozed view.
- It wakes when the time passes (sweeper), when the customer replies, or when a
  teammate wakes it. Resolving clears the snooze. Wake-ups add a timeline event.

### SLA policies

- Table `support_sla_policies`: name, enabled, position, conditions
  (priorities, inboxes, channels; empty means any), targets in minutes for
  first response, next response and resolution (each optional), business-hours
  mode, and whether the resolution clock pauses while waiting on the customer
  or snoozed.
- Table `support_conversation_slas`: the applied policy and, per metric, due
  time, achieved time and breach time; accumulated pause; `next_due_at` for
  sorting and the sweeper.
- Evaluation is a pure function of the conversation, its public messages, the
  policy and the business-hours schedule. A Postgres trigger enqueues changed
  conversations into `support_sla_work`; a worker evaluates queued
  conversations every few seconds and sweeps due timers every minute.
- First matching policy (by position) applies. A priority or inbox change
  re-selects the policy; achieved metrics keep their result.
- Breaches write an internal timeline event, publish a realtime update and
  notify the assignee (or inbox members when unassigned).
- Frontend: SLA settings page (policy list, editor, ordering); SLA chip in the
  conversation list and header with live countdowns; "Next SLA due" sort and an
  "SLA breached" filter; SLA hit rates in reports.

## Phase 3: Inbox productivity

- Macros: canned responses gain optional actions (set status, priority,
  assignee, inbox, add or remove tags, snooze). Actions run on the server
  through the same service methods as manual changes. A macro with text inserts
  it into the composer and runs its actions when the reply is sent; a macro
  without text runs immediately.
- Bulk actions: `POST /inbox/conversations/bulk` for status, assign, priority,
  snooze, inbox, add and remove tag, with per-conversation results (max 100).
  The toolbar gains assign, priority, tag and snooze.
- Keyboard shortcuts: `j`/`k` move through the list, `e` resolves, `z`
  snoozes, `p` sets priority, `a` assigns, `t` tags, `r` replies, `n` writes a
  note, `?` opens a cheatsheet. Shortcuts never fire while typing.

## Phase 4: Webhooks and public API writes

- Workspace webhook endpoints: URL (HTTPS, SSRF-guarded), event subscription,
  signing secret, enable/disable. Events: `conversation.created`,
  `conversation.status_changed`, `conversation.assigned`,
  `conversation.priority_changed`, `conversation.snoozed`,
  `message.created` (public customer and teammate messages),
  `sla.breached`, `csat.submitted`.
- Durable outbox (`webhook_deliveries`) with signed payloads
  (`Helpin-Signature: t=<unix>,v1=<hmac-sha256>`), exponential retries for 24
  hours, and auto-disable after repeated failures. Settings page with a test
  send and the recent delivery log.
- Public REST and MCP: send a reply or note, change status, change priority,
  snooze, and create a conversation; read SLA and CSAT on conversations.

## Phase 5: Intercom importer

- Import Help Center collections and articles and closed or open conversations
  (with contacts) from an Intercom access token, following the HelpScout
  importer's job model, idempotent on Intercom IDs. Tested against a recorded
  fake Intercom API.

## Deferred

- Slack and Slack Connect as a channel: large (42-55 days estimated in the
  August plan). Webhooks and the API cover notifications until a customer asks.
- Native Linear or Jira sync: webhooks plus the existing Linear MCP preset cover
  the first need; build two-way sync with a design partner.
- Company-wide request visibility in the portal: waits for a B2B customer that
  needs it.

## Verification for every phase

1. Go unit tests (SQLite) for services and repositories; Postgres tests for
   migrations, triggers and counter functions; `fresh-ledger` parity.
2. Frontend Vitest for new components and hooks; widget-core and sdk-js tests
   where the widget changes; TypeScript and production builds for both editions.
3. Browser tests on the dev environment with injected data: inbox, reports,
   settings, widget and portal flows.
4. An independent review of the phase diff, fixes, and a re-run of 1-3.
