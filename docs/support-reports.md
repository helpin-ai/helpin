# Support reports, CSAT and SLA semantics

This page is for contributors changing support measurement. It defines every
metric on the Support reports page and in its CSV export, how customer
satisfaction (CSAT) ratings are collected, and how SLA policies judge a
conversation. The code that implements it lives in
`server/internal/service/support_reports.go`, `support_csat.go`,
`support_sla.go` and `support_sla_evaluate.go`.

## Reports

`GET /api/workspaces/{ws}/support/reports/overview` returns the metrics below
for a date range (at most 366 days) in the viewer's time zone. It also returns
the previous period of equal length, used for the trend arrows. Both the
overview and `conversations.csv` include only conversations in inboxes the
viewer can open.

Metrics come from `support_conversations` and `support_messages`, so they
cover history from before reports existed. Aggregation runs in Go over one fact
row per conversation. Postgres, the SQLite tests and the CSV export therefore
share one implementation.

| Metric | Definition |
| --- | --- |
| Conversations | Created in the range. Spam and conversations merged into another are excluded. |
| First response time | From the first customer message to the first public teammate reply. Counted only for conversations the customer started. Reported as median and p90. |
| Time to resolution | From creation to `resolved_at`, for conversations resolved in the range. Reported as median and p90. |
| Resolved | Conversations resolved in the range, attributed to the teammate who resolved them last. |
| AI involvement | Conversations with at least one public AI reply. |
| AI resolution rate | AI-resolved conversations over AI-involved ones, split into confirmed and assumed resolutions. |
| Handoff rate | AI-involved conversations escalated to a teammate. |
| Backlog | Open and waiting conversations right now, and how many have no assignee. Not tied to the range. |
| CSAT | Average rating, number of ratings, and share of 4 and 5 ratings, split by AI and human resolution. |
| SLA hit rate | Per target: met over met plus missed. Cancelled targets are left out. |

Values in the CSV that a spreadsheet could read as a formula are escaped.

## CSAT

A conversation has at most one rating (`support_csat_responses`): 1 to 5 with
an optional comment of up to 1,000 characters. The rating records its channel
(`widget` or `portal`), who resolved the conversation (`ai` or `human`), and
the assignee at the time it was submitted.

- Customers can rate only a resolved conversation of their own, and only while
  CSAT is enabled in Messenger settings.
- Submitting again while the conversation stays resolved updates the rating
  instead of adding a second one. A submission that changes the rating adds an
  internal `csat_rated` timeline entry.
- Ratings never cross a merge between different customers.

## SLA policies

Each policy has optional targets in minutes: first response, next response and
resolution. Its conditions are priorities, inboxes and channels; an empty list
matches every conversation. The first enabled matching policy, by position,
applies. Only teammate replies meet response targets.

### What each target measures

- **First response**: from the customer's first message to the first teammate
  reply. A conversation a teammate started has no first-response target.
- **Next response**: each customer message after a teammate reply starts a
  cycle, and the next teammate reply ends it. Every missed cycle counts.
- **Resolution**: from creation to the first resolution. While the
  conversation waits on the customer or is snoozed, the clock pauses, unless
  the policy turns pausing off.
- **Business hours**: the clock counts only the workspace's business hours,
  handling daylight-saving changes. Without a schedule, it counts calendar
  time.

### Conversations the AI handles

SLA targets measure the team. While the AI owns a conversation, its targets
are paused. They start at the handoff: the escalation, the customer's request
for a human, a teammate pausing the AI, or the first teammate reply,
whichever comes first. If the AI resolves the conversation without a handoff,
its targets are cancelled and stay out of hit rates.

When a teammate hands a conversation back to the AI after the handoff:
- the resolution clock pauses;
- the open next-response cycle waits;
- when the team takes the conversation back, that cycle starts again from
  that moment.

### Results are final

- A record keeps a snapshot of the targets the conversation started with.
  Editing a policy affects only conversations that start it afterwards.
- A met target stays met, and a recorded breach stays recorded, even if a
  message is deleted later. Neither is judged again, whatever happens to the
  policy.
- When a priority, inbox or channel change selects a different policy:
  - Recorded results carry over.
  - Targets still running are judged by the new policy from their original
    start, so a stricter policy can mark one overdue at once.
  - Next-response cycles that ended before the change keep their earlier
    result.
- When no policy applies any more (the policy was disabled, deleted or stopped
  matching, or the workspace lost the SLA feature), running targets are
  cancelled. The record stays, so reports keep its results. A conversation
  closed while no policy applies records nothing more.
- When a policy applies again after none did, its targets count from that
  moment, so a break in SLA tracking never back-dates breaches.
- When a policy's conditions change, a conversation that starts matching
  because of the change is measured from the time of the change. It picks up
  the policy at its next update.
- A policy applies only to conversations created after it was enabled.
  Re-enabling it moves that point forward.
- Reopening a resolved conversation keeps its resolution result. Response
  targets continue with the customer's new message.

### Evaluation and breaches

The evaluation queue (`support_sla_work`) is filled by database triggers when a
conversation's state changes:

- status, priority, inbox, channel or snooze
- AI ownership
- public replies, including deleted ones

A worker in the API evaluates queued conversations every few seconds. It
claims rows with a two-minute lease and deletes a row only after its
evaluation succeeds, so a crash or deploy loses no work. Each conversation's
`wake_at` is its next due time; the worker re-evaluates the conversation then,
to record the breach on time. Evaluations of one conversation are serialized
with an advisory lock.

A new breach:

- writes an internal `sla_breached` timeline entry;
- notifies the assignee, or the inbox's members when nobody is assigned, or
  the workspace's support teammates for conversations without an inbox.

Breaches on resolved or spam conversations are recorded but send no
notification.

## Snooze

A snoozed conversation leaves Inbox, Mine, Unassigned and Waiting, and their
counters, and appears in the Snoozed view. It wakes:

- when the time passes;
- when the customer replies (the reply itself is the visible wake-up);
- when a teammate wakes it.

Waking at the set time or by a teammate adds a timeline entry. Resolving or
marking a conversation as spam clears its snooze, whatever code path made the
change. Customers never see the snooze.
