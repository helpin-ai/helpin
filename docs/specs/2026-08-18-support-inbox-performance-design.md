# Support inbox performance design

**Status:** Partially superseded
**Superseded on:** 2026-09-03
**Authoritative follow-up:** [First-Class Support Inbox State Design](2026-09-02-first-class-support-inbox-state-design.md)

> The September design and its implementation record are authoritative for inbox list projections, internal personal unread, blue-dot and human-attention semantics, core counters, database indexes, read-triggered invalidation, and rollout. This document remains useful for bounded message history, page-scoped hydration, lazy secondary surfaces, and other thread-loading improvements that the September work did not replace. Do not use this document's narrower index-only or shared read-invalidation assumptions to design new inbox counter work.

## Implementation review (2026-09-17)

For contributors maintaining thread loading, the bounded-history portions of this
historical design are implemented. The September follow-up above remains the
reference for list state and counters; this review does not claim measured
latency improvements or a new browser regression run.

The [message-page service](../../server/internal/service/support_message_page.go)
checks conversation access, defaults to 20 messages, and accepts limits from 1
to 100. The [repository](../../server/internal/repository/support_inbox.go) uses
a `(created_at, id)` boundary, fetches one extra row, and returns chronological
pages. Attachment hydration and email-log queries are scoped to returned message
IDs, with hydration errors logged rather than failing the page. Other full-history
and offset-based methods still exist for separate callers.

The cursor is unsigned Base64url JSON containing a timestamp and ID. The
[handler](../../server/internal/handler/support_inbox.go) returns a sanitized 400
for malformed cursor data; the design's “tampered” wording overstates this check.
A syntactically valid changed cursor is accepted as a different pagination
boundary. Access enforcement comes from the scoped conversation check and query,
not cryptographic cursor integrity.

The [infinite-query hook](../../frontend/src/hooks/queries/useSupport.ts) requests
20-message pages under conversation-specific cache keys. The
[thread](../../frontend/src/components/support/MessageThread.tsx) loads older pages
near the top, restores scroll position, stops automatic loads after a page error,
and exposes **Load earlier messages** for retry. Shared
[cache helpers](../../frontend/src/lib/supportMessagePages.ts) handle message-page
updates. Conversation-keyed caches isolate data; they do not themselves prove
that an in-flight network request is cancelled on every conversation switch.

[SupportInboxLayout](../../frontend/src/components/support/SupportInboxLayout.tsx)
conditionally mounts lazy detail/agent sidebars; the thread lazily loads its
composer and create-task dialog. These code boundaries support the loading design,
but do not establish bundle-size or p50/p95 results. Duration logging is present
in the page service. The original bottleneck list below is the August baseline,
not a claim that those same bottlenecks remain today.

## Goal

Make the Support inbox substantially faster on its first visit and when switching
conversations, while preserving the current agent workflow and all existing full-transcript
callers. The thread initially renders the newest 20 messages and loads older history
automatically at the top, with a manual retry affordance after failures.

## Confirmed bottlenecks

- The Support route statically includes heavy secondary features that are not required to
  render the first inbox screen.
- Selecting a conversation eagerly starts requests owned by hidden sidebars and closed
  drawers.
- The agent thread endpoint loads the complete message history, then hydrates attachments
  and every conversation email log.
- The inbox list performs several correlated lookups per row and lacks indexes aligned with
  its default filters and ordering.
- Opening an unread conversation can issue duplicate mark-read work and repeated count
  invalidations.

## Architecture

### Bounded message history

Add a dedicated admin message-page contract rather than changing the existing full-history
service contract. The page request accepts a stable cursor and a bounded limit; the inbox
uses 20. The repository queries newest-first with `LIMIT 21`, uses the extra row to determine
`has_more`, then returns the selected page in chronological order. The cursor contains the
oldest returned message's ordering tuple so older pages do not use increasingly expensive
offsets or a separate total-count query.

Existing widget, transcript, triage, automation, and internal-tool consumers continue to use
the unpaginated method. This isolates the behavior change to the agent inbox.

### Page-scoped hydration

Hydrate attachments only for the messages in the requested page. Add an email-log repository
query scoped to the page's message IDs, and use it for page hydration so large email threads
do not load raw and HTML bodies for unseen messages. The existing conversation-wide email-log
query remains available to callers that genuinely need a full transcript.

### Frontend data flow

Replace the agent thread's single query with a TanStack infinite query:

1. Fetch the newest 20 messages.
2. Store pages in API fetch order but expose one chronological flattened list to rendering.
3. When the thread viewport reaches the top and `has_more` is true, request the next older
   page.
4. Measure the pre-fetch scroll height and restore the viewport delta after prepending the
   page so the visible message does not jump.
5. If automatic loading fails, stop automatic retries for that failure and render a
   **Load earlier messages** button. Clicking it retries; success restores automatic loading.

Optimistic sends, deletes, undo, and realtime message events update the newest page while
preserving already loaded older pages. Cache helpers will encapsulate this behavior so each
mutation and realtime handler does not manipulate `InfiniteData` independently.

### Request and bundle reduction

- Enable customer-profile contact, association, and support-history queries only while the
  drawer is open.
- Mount the detail sidebar only when a conversation is selected and the viewport is at least
  `xl`; CSS-hidden panels must not own active data hooks.
- Keep the Ask Agents sidebar inactive and unmounted until opened.
- Lazy-load secondary Support surfaces at existing component boundaries, prioritizing the
  reply composer, task creation, customer profile, Ask Agents, and document-preview paths.
- Preserve the selected-row response, thread skeleton, composer readiness rules, and all
  existing desktop/mobile navigation behavior.

### Database improvements

Add idempotent indexes aligned with:

- support-message cursor scans by workspace, conversation, creation time, and ID;
- default conversation listing by workspace and descending update time;
- common status/mailbox list filters where the existing query shape benefits.

Inbox SQL changes beyond indexes must be justified by focused repository tests or query-plan
evidence. This design does not replace the whole list query with a new aggregation endpoint.

### Duplicate work and observability

Make the opened thread the single owner of mark-read behavior and remove the redundant list
click request. Consolidate related cache invalidation so one successful transition does not
refresh the same count families repeatedly.

Add structured duration logging around the inbox list and paginated message service paths,
including workspace and conversation identifiers but never message or email content. This
provides production evidence for p50/p95 analysis using existing request logs.

## Compatibility and failure handling

- Existing full-history APIs and service callers retain their behavior.
- Invalid or tampered cursors return a sanitized `400` response.
- A page hydration failure remains non-fatal, matching current attachment/email hydration
  behavior, and is logged with identifiers only.
- A failed older-page request leaves the currently rendered messages intact and exposes the
  retry button.
- A conversation switch cancels or isolates in-flight history work through conversation-keyed
  query caches, preventing pages from one conversation appearing in another.
- Realtime arrivals during older-page loading are deduplicated by message ID.

## Testing strategy

Work proceeds test-first in small, independently verifiable changes:

- repository ordering, cursor boundaries, `has_more`, internal-message filtering, and stable
  ties on timestamps;
- handler cursor validation and response shape;
- page-scoped attachment and email-log hydration;
- infinite-cache append, reconcile, delete, undo, invalidation, and deduplication behavior;
- initial 20-message rendering, automatic top loading, viewport preservation, exhausted
  history, and retry-button fallback;
- drawer/sidebar request gating at closed and narrow states;
- single mark-read ownership;
- migration validation, focused Go and Vitest suites, frontend production build, Go build,
  and broad relevant test suites.

Bundle output and focused endpoint/repository measurements will be compared with the recorded
baseline. No performance claim will be made solely from code inspection.

## Non-goals

- Redesigning the inbox UI.
- Changing widget transcript pagination.
- Combining every conversation panel into one aggregate endpoint.
- Rewriting the entire inbox list query without measured evidence.
- Removing or delaying realtime delivery of newly received messages.
