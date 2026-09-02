# First-Class Support Inbox State Design

**Date:** 2026-09-02
**Status:** Independent written-spec review approved; awaiting user review
**Target branch:** `waqar-fixes`
**Scale target:** 300,000 conversations and 3,000,000 messages in one workspace, ten times the measured baseline

## Summary

Replace the support inbox's workspace-wide read cursor and read-triggered query fan-out with four explicit state domains:

1. shared conversation workflow state;
2. personal agent read state;
3. shared customer-response state, represented by the existing blue dot;
4. durable, incrementally maintained counters and targeted realtime patches.

The release preserves saved-view membership and the existing blue-dot interaction. It changes unread state from shared to personal, prevents teammate replies from creating unread state for other agents, keeps AI Handling membership independent of unread state, and removes message-table scans from normal inbox list and counter requests.

The production cutover is one user-visible release, but deployment is additive and reversible: create schema, dual-write, backfill, shadow-compare, switch reads, and retain the legacy cursor until the new path is proven.

## Current System and Failure Mode

`support_conversations.team_last_seen_at` is the only internal read cursor. A customer reply is unread when its message timestamp is later than this shared value. Any agent opening the conversation advances the cursor for the entire team.

Conversation lists obtain the last-message preview, last public sender, unread count, awaiting-reply state, and visitor country through correlated subqueries. `GetUnreadStats` scans every unresolved conversation and repeats unread and mention subqueries across multiple aggregates. Mailbox scopes execute separate total and unread queries per mailbox, while custom-view counts execute separate total and unread queries per view.

Opening a conversation updates the shared cursor and publishes a generic `support_conversation` event to every internal workspace client. Each client invalidates conversation lists, details, unread stats, inbox scopes, and custom-view counts. The initiating browser also invalidates those counters after its mutation succeeds.

The latest `waqar-fixes` base has already improved the legacy write so the cursor advances to the latest readable customer message rather than wall-clock time. This closes one race but does not change the shared semantics or invalidation fan-out.

## Product Semantics

### Shared response state and the blue dot

The existing blue dot remains the shared response/action indicator. Its authoritative field will be `customer_awaiting_response`.

`customer_awaiting_response` becomes true when a public customer reply is committed. It becomes false when:

- a public teammate or AI reply is committed;
- the conversation is resolved or marked spam;
- deletion of the relevant message causes the projection to recompute to a non-customer terminal state.

Reading or opening a conversation never clears this field. Internal notes never clear it. Reopening a resolved conversation does not set it unless an unanswered customer message exists.

While AI owns the conversation, the blue dot can remain visible while the customer waits for AI. That state does not count as human workload until AI hands off, fails, or a human takes over.

### Human workload

`needs_human_reply` is shared workflow state. It is true only when `customer_awaiting_response` is true and the effective response owner is human. It is false during normal AI Handling.

AI handoff, AI failure, explicit human takeover, or routing into a human queue sets `needs_human_reply` when the customer is still awaiting a response. A public teammate reply or resolution clears it.

The Support rail and team-inbox attention indicators use `needs_human_reply`, not personal unread.

### Personal unread

Personal unread belongs to one workspace user and one conversation.

Customer replies create or increment personal unread state only for relevant agents:

- the current `assigned_user_id`;
- the current `opened_by_user_id`;
- users historically mentioned in the conversation, preserving current Mine-view membership.

Team or mailbox membership alone does not create personal unread rows. An unassigned or team-assigned conversation is discoverable through shared workload and the blue dot without creating unread state for every team member.

Opening a conversation clears unread only through the newest customer message actually rendered by that agent. The client sends `through_message_id`; the server never assumes that every message existing at request time was seen. The operation is monotonic and concurrency-safe.

Opening a conversation by a non-relevant agent records their read position but does not subscribe them to future unread updates. A future assignment or mention makes them relevant.

`Mark unread` affects only the current agent. It sets `manually_unread` without rewriting shared conversation state. A subsequent successful personal-read operation clears it.

A public teammate or AI reply does not create unread state for other agents. An internal note creates personal attention only for explicitly mentioned users, through the existing notification category and mention membership.

### Customer/widget unread

Customer-facing read state remains a separate domain. The widget continues to use the contact/session read position and receives agent/AI replies through the visitor-scoped realtime path. Internal agent read-state changes never alter customer unread state.

### AI queues

AI Handling membership remains based on effective AI ownership (`flow_state = ai_handling`, with the existing `ai_state = pending` fallback, and no human takeover). It never depends on personal unread or `needs_human_reply`.

State transitions are:

| Event | AI Handling | Blue dot | Human attention |
|---|---:|---:|---:|
| Customer replies while AI owns | remains | on | off |
| AI replies publicly | remains | off | off |
| AI hands off/fails with customer waiting | leaves | on | on |
| Human takes over with customer waiting | leaves | on | on |
| AI resolves | leaves | off | off |

## Compatibility Invariants

The following invariants are release blockers:

- Existing saved-view JSON remains readable without migration or user action.
- View membership for state, assignment, opened-by, mention, mailbox, tag, AI, search, and sort filters is unchanged.
- `total_count` for a saved view has the same membership semantics before and after cutover.
- `unread_count` becomes personal to the requesting user.
- `needs_human_reply_count` is additive in API responses and drives shared attention UI.
- Shared views may return different `unread_count` values to different users.
- The blue dot is never cleared by reading and is never conflated with personal unread.
- Team replies never create agent unread state.
- Unauthorized mailbox conversations never enter a user's list, counters, or realtime cache.
- Resolved and spam conversations do not contribute human-attention counters.
- A duplicate message, duplicate read request, duplicate outbox delivery, or retry cannot double-increment a counter.
- A delayed or out-of-order websocket event cannot overwrite a newer client state.

## Data Model

### Conversation projection

Add these projection columns to `support_conversations`:

| Column | Purpose |
|---|---|
| `list_last_message_id` | Message represented by the inbox preview |
| `list_last_message_at` | Stable ordering/input to preview projection |
| `list_last_message_preview` | Sanitized/truncated list preview |
| `list_last_message_is_internal` | Preserve the `Note:` presentation without a message lookup |
| `last_public_message_id` | Latest non-internal reply |
| `last_public_message_at` | Stable public-message ordering |
| `last_public_sender_type` | Drives sender indicator and projection rules |
| `last_public_sender_display_name` | Preserves the agent-replied label |
| `last_customer_message_id` | Latest readable customer reply |
| `last_customer_message_at` | Cursor tuple component |
| `unanswered_customer_message_count` | Current customer-message burst awaiting a response |
| `customer_awaiting_response` | Authoritative blue-dot state |
| `needs_human_reply` | Authoritative shared human-attention state |
| `state_version` | Monotonic version for counters and realtime ordering |
| `visitor_country_code` / `visitor_country_name` | Removes widget-session subqueries from list fetches |
| `view_search_document` | Derived normalized subject/customer identity input for saved-view search maintenance; legacy search truth-table semantics remain authoritative |

Message ordering uses the tuple `(created_at, id)` so equal timestamps are deterministic. Normal create/update projection writes compare tuples and never move backwards. Message deletion, visibility changes, and moderation are explicit exceptions: they lock the conversation, recompute the affected projection from the newest surviving eligible message, and increment `state_version` even when the resulting message tuple is older.

The existing `team_last_seen_at` remains during rollout as a legacy compatibility field. It is not removed in this release.

### Agent conversation state

Create `support_conversation_user_states`:

| Column | Definition |
|---|---|
| `workspace_id` | Required tenant key |
| `conversation_id` | Required conversation key, cascade on conversation deletion |
| `user_id` | Required agent key |
| `last_read_customer_message_id` | Newest customer message confirmed rendered |
| `last_read_customer_message_at` | Cursor tuple component |
| `unread_customer_message_count` | Event-maintained exact count |
| `manually_unread` | Personal manual reminder independent of message count |
| `mentioned_at` | Durable replacement for historical metadata scans in Mine/mention filters |
| `relevance_mask` | Current assignee, opener, and historical-mention relevance bits |
| `version` | Monotonic personal-state/event version |
| `created_at` / `updated_at` | Audit fields |

Primary key: `(conversation_id, user_id)`. Required indexes:

- `(workspace_id, user_id, conversation_id)`;
- partial `(conversation_id, user_id)` where `unread_customer_message_count > 0 OR manually_unread OR relevance_mask <> 0`, used to enumerate only conversation-specific personal contributors during shared transitions;
- partial `(workspace_id, user_id)` where `unread_customer_message_count > 0 OR manually_unread`;
- partial `(workspace_id, user_id, mentioned_at DESC)` where `mentioned_at IS NOT NULL`.

The effective unread count returned to existing clients is `unread_customer_message_count`, with a minimum display value of one when `manually_unread` is true.

Relevance transitions are explicit:

- assigning a user sets assignee relevance and initializes unread from customer replies after that user's stored cursor, bounded to the current unanswered customer-message burst; a missing cursor treats the whole outstanding burst as unread;
- changing `opened_by_user_id` removes opener relevance from the previous opener and adds it to the new opener using the same outstanding-burst rule;
- mentioning a user sets durable mention relevance and, when a customer response is outstanding, initializes unread after their stored cursor in addition to creating the mention notification;
- removing assignment or opener relevance preserves the read cursor and clears message-derived unread only when no other automatic relevance bit remains; an explicit `manually_unread` reminder is preserved until that user reads or clears it;
- deleting the last message that mentions a user recomputes `mentioned_at` and mention relevance; if no automatic relevance remains, message-derived unread is cleared while `manually_unread` is preserved;
- a user who remains relevant continues receiving future customer-reply unread increments; an irrelevant user retains only cursor history.

Any authorized viewer may explicitly mark a conversation unread even when they have no automatic relevance. That reminder appears in their personal unread totals for views in which the conversation is already a member, but does not add the conversation to Mine and does not subscribe the user to future customer replies. Reading the conversation clears the manual reminder.

### Core counter buckets

Create `support_inbox_counter_buckets`, keyed by workspace, non-null `audience_type`, non-null `audience_id`, bucket type, and non-null bucket ID. `audience_type = shared` uses `audience_id = shared`; `audience_type = user` uses the user's UUID string. Explicit sentinel strings avoid PostgreSQL nullable-uniqueness gaps.

Shared buckets are materialized at the smallest authorization-safe scope: the legacy shared mailbox and each concrete mailbox. Workspace-level Inbox, Waiting, AI, Unassigned, and Support totals are obtained by summing only the shared/mailbox buckets the requesting actor may access. They are never stored as one unrestricted workspace total and never returned before mailbox authorization is resolved.

Core buckets cover:

- human Inbox;
- Mine per user;
- Waiting;
- AI Handling;
- AI Resolved where currently displayed;
- Unassigned;
- each active mailbox/team inbox;
- workspace-level Support attention assembled from authorized mailbox buckets.

Each bucket stores `total_count`, `needs_human_reply_count`, `unread_count`, `version`, and `updated_at`, with non-negative check constraints. Pure membership functions calculate a conversation's before/after contribution. The same database transaction that changes authoritative conversation or user state applies the delta. Personal buckets are also stored at mailbox scope. Every counter endpoint first resolves the requester's current mailbox authorization and sums only those scopes, so granting or revoking access changes the visible result immediately without rebuilding historical buckets or exposing a stale unauthorized total.

Core-counter replacements use the same explicit version-vector contract as custom views: mailbox access version plus a shared and, where applicable, personal version for every contributing mailbox scope. Bucket rows for all categories affected by one scope mutation are updated atomically under the allocated scope version. Clients never compare a per-conversation version to an aggregate and never collapse a multi-mailbox result into a synthetic scalar version.

### Custom-view count snapshots

Arbitrary custom filters are not synchronously reevaluated for every connected client.

Materialize custom-view counts in authorization-safe pieces rather than one unrestricted workspace total:

- `support_inbox_view_scope_counts(view_id, mailbox_scope_id, total_count, needs_human_reply_count, source_version, computed_at)` stores user-independent membership per concrete mailbox, using the legacy-mailbox sentinel where applicable;
- `support_inbox_view_user_counts(view_id, user_id, mailbox_scope_id, total_adjustment, needs_human_reply_adjustment, unread_count, shared_source_version, personal_source_version, computed_at)` stores the personal overlay needed for `me`, `others`, opener, mention, and unread semantics;
- `support_inbox_scope_heads(workspace_id, mailbox_scope_id, shared_version)` provides the database-incremented source head for shared changes;
- `support_inbox_user_scope_heads(workspace_id, user_id, mailbox_scope_id, personal_version)` provides the database-incremented source head for personal changes.

Shared and personal versions are independent integer sequences allocated while their head row is locked. A projection-affecting mutation increments the shared head for its scope; a mailbox move increments both old and new shared heads. A personal-state mutation increments the affected user/scope head. Versions never derive from wall-clock timestamps and a per-conversation user-state version is never reused as an aggregate version.

A tested view-filter compiler decomposes each existing serialized filter into a shared mailbox predicate plus bounded personal adjustments. For example, `assigned = others` is the shared assigned-non-null count minus the current user's assigned count; `assigned = me`, `opened_by = me`, `mentioned = me`, and personal unread are user overlays. Boolean combinations preserve the current filter evaluator's truth table. A golden corpus containing every supported filter key and operator proves that compiled list membership and count membership agree with the legacy evaluator. V2 has no partially correct fallback: every filter accepted by the current API must compile, an uncompiled existing view blocks its workspace's cutover, and create/edit rejects no filter that the legacy evaluator accepts.

The authoritative mutation transaction appends one logical conversation change and one ordered log entry for each touched shared or user/mailbox scope. Each log entry has a unique mutation ID, its allocated scope version, old/new values, and the bounded set of users whose personal contribution can change. Personal mutations include their direct actors/recipients. Shared membership mutations query the indexed conversation-state set and include every row with unread, `manually_unread`, or relevance contribution, including manual-only users from earlier actions. A mailbox move emits old-scope removal and new-scope addition entries for each such contributor. Work may scale with state contributors to this conversation, but never enumerates unrelated workspace members, shared-view users, or connected clients.

A post-commit view projection worker leases scopes with `FOR UPDATE SKIP LOCKED`, but processes each leased scope strictly in contiguous version order. It applies `(scope, version, mutation_id)` idempotently, evaluates the changed conversation against affected view predicates once per view, applies shared-scope deltas and directly affected personal overlays in one transaction, and only then advances that scope's applied watermark. A gap stops the scope; a higher version can never be published while a lower version is unapplied. Parallel workers may process different scopes, never the same scope concurrently. Thus foreground work is constant with respect to view count and workspace membership; background work is bounded by affected views plus directly affected users, never all users of a shared view.

Creating or editing a view builds its mailbox-scope snapshots and the creator's personal overlays with the same watermark-and-replay protocol used by migration. First access by another user lazily builds only that user's overlay: the endpoint may execute the existing exact count once while establishing the baseline, but it never returns a guessed or incomplete V2 count. Access grants may prewarm this work asynchronously.

The count endpoint first resolves the requester's current mailbox authorization, sums only authorized scope rows, and applies only that user's matching overlays. Access revocation therefore excludes the unauthorized scope synchronously even when projection jobs are stale. There is deliberately no scalar aggregate version. Each response carries `(mailbox_access_version, {mailbox_scope_id: {shared_applied_watermark, personal_applied_watermark}})`. A replacement is accepted only for the same access version when every component is greater than or equal to the cached component; incomparable vectors trigger a refetch. A delta event names exactly one scope, audience, and next contiguous version. Recount completion publishes a small count-patch event. Conversation-list membership remains authoritative and immediate; only sidebar count refresh is eventually consistent, with a target convergence under two seconds.

## Transaction and Projection Boundary

Introduce a focused support-conversation state projector rather than adding more responsibilities to the existing large service/repository files.

For every authoritative message or conversation transition, one PostgreSQL transaction will:

1. lock the conversation row;
2. initialize its V2 projection/contribution generation exactly once if needed;
3. apply the idempotent message or conversation mutation;
4. update the denormalized conversation projection;
5. update affected personal states;
6. compute before/after bucket membership and apply counter deltas from the contribution ledger;
7. append one logical conversation change plus its ordered entries for only the touched shared and personal scopes;
8. insert typed realtime outbox records;
9. maintain the documented legacy cursor mapping and commit.

Attachments, email delivery, push delivery, analytics, triage execution, and other external side effects run only after the authoritative transaction commits. Existing idempotency keys remain the source of duplicate-message protection.

All paths that can change any saved-view membership input must pass through the projector. This explicitly includes status, mailbox, assignment, opener, AI ownership, human takeover, message visibility/deletion, tags, subject, customer name, customer email, and the derived search document; immutable `display_id` is initialized through the same path. `UpdateConversationSubject`, customer/contact identity synchronization, imports, merges, and direct `UpdateFields` callers are audited and migrated.

A single declarative filter-field registry maps every accepted serialized view key to its source columns, normalization function, projector transition, scope-version impact, and count predicate. Both the view compiler and a mutation-coverage test consume this registry. Adding a mutable filter field without a registered projector transition fails the test, preventing future direct writes from silently making saved-view counts stale. Subject or customer-identity changes lock the conversation, update source and derived fields, increment the mailbox shared head, update the contribution ledger, and append the ordered view-log entry in the same transaction.

## Personal Read Concurrency

The read request becomes:

```json
{
  "through_message_id": "uuid"
}
```

Within a transaction, the server validates that the message belongs to the accessible conversation and is a public customer reply, locks/upserts the user's state row, and advances the cursor only when the supplied `(created_at, id)` tuple is newer.

It then counts customer replies after that tuple for this one conversation using the partial message index. This preserves a customer reply committed concurrently after the rendered message. The personal unread bucket changes only if effective unread transitions between zero and nonzero. The response returns the authoritative count and state version.

Requests without `through_message_id` are accepted temporarily for old clients and use the conversation's currently projected latest customer message. This compatibility path is removed only after all supported clients send the explicit cursor.

## Realtime Protocol

Use typed support events with an `event_id`, conversation `state_version`, optional user-state `version`, reason, minimal patch, and exact counter deltas.

Event classes:

- `support.conversation_state_changed`: shared projection/status/assignment/AI patch;
- `support.message_created` / `support.message_updated` / `support.message_deleted`;
- `support.personal_read_changed`: targeted to one user;
- `support.counter_delta`: shared or targeted bucket delta;
- `support.view_counts_refreshed`: shared or targeted snapshot replacement.
- `support.authorization_changed`: targeted access-version change with removed mailbox scopes.

Add an optional target user to websocket events and enforce it in the hub. Personal-read events are delivered only to the same user's sessions. Shared events contain no mailbox-sensitive customer content; clients patch only cached rows they already possess and refetch the detail only when the selected conversation requires it.

Mailbox access changes increment a per-user `mailbox_access_version` and insert a targeted authorization event in the same transaction. On receipt, every session for that user cancels in-flight support requests, removes conversation lists, details, messages, core counters, view counts, and search results for removed scopes, rotates all remaining support query keys to the new access version, and refetches authorization before processing later support events. Server APIs and websocket subscriptions always recheck current authorization; reconnect, window focus, and a bounded background refresh reconcile a missed event. No response generated after revocation may include the removed scope.

Support realtime outbox rows are inserted in the authoritative transaction. A leased worker publishes them through the existing multi-pod relay, retries with bounded exponential backoff, and records delivery state. Clients deduplicate by `event_id` and ignore versions older than their cached version.

Reading a conversation never invalidates workspace conversation lists, mailbox scopes, or other users' custom-view counts. New messages patch the row, messages, applicable core counters, and only the selected detail. Status/assignment/AI transitions patch the row and exact affected counters; a list refetch is a recovery mechanism, not the normal path.

## API Contracts

Keep existing routes while evolving responses additively.

- Conversation list rows retain `unread_count` and `awaiting_reply` for compatibility. `unread_count` becomes personal; `awaiting_reply` is sourced from `customer_awaiting_response`.
- Add `needs_human_reply`, `state_version`, and `personal_state_version` to conversation DTOs.
- Add `needs_human_reply_count` and `version` to inbox-scope and view-count DTOs.
- Remove unread aggregate computation from the list endpoint after clients stop consuming `meta.unread`; return a compatibility-shaped zero/omitted object during the transition.
- The dedicated counter endpoint reads core buckets rather than scanning conversations/messages.
- The read endpoint accepts `through_message_id` and returns authoritative personal state.
- Mark-unread returns authoritative personal state.
- `/support/workspace-unread` reads the requesting user's authorized personal-unread core buckets. Its badge means "you have personally unread, relevant support conversations in this workspace" and never scans `support_messages`.

Mailbox access and Support module permissions are applied before reading or mutating personal state and before returning counters.

A successful personal read marks support-reply notifications read only for the requesting user. It never clears the opener's, assignee's, mentioned user's, or any other recipient's notification state.

## Frontend State and Indicators

The existing indicator vocabulary is retained but its sources become explicit:

- blue customer-replied dot: `customer_awaiting_response`;
- bold row and unread-message badge: personal `unread_count`;
- Support rail and team-inbox attention: shared `needs_human_reply_count`;
- AI Handling total: shared AI state bucket;
- AI handoff/failure attention: shared human-attention bucket after transition;
- visitor-online, typing, viewing, draft, selected-row, resolved, and AI-resolved indicators: their existing domains.

The client sends the last rendered public customer message ID when the thread is at the read boundary. Optimistic clearing is allowed only for that user's row and counter; the mutation response reconciles the exact value. A newer message event with a higher version restores/increments unread instead of being overwritten by a delayed read response.

TanStack Query keys remain compatible. The realtime handler gains pure patch functions and reason-specific routing. Generic support-conversation invalidation is removed only after typed events have deterministic coverage tests.

## Migration and Cutover

### Schema and indexes

Use separate migrations:

1. transactional schema creation with nullable projection columns and new tables;
2. non-transactional concurrent index creation;
3. validated constraints/defaults after backfill.

The current migration runner wraps SQL in a transaction, so it must first gain an explicit, tested no-transaction directive before any `CREATE INDEX CONCURRENTLY` migration is added.

Add the specialized message, mention GIN, conversation, session, user-state, counter, view-job, and outbox indexes identified by the performance diagnosis. Never edit an applied migration.

### Backfill

Provide an idempotent, resumable command that processes conversations in bounded primary-key batches and records progress. It computes conversation projections, country fields, counter membership, mention membership, and initial user states.

Initial user-state recipients are assigned users, opened-by users, and historically mentioned users. `team_last_seen_at` becomes the initial read boundary. If an existing personal support-reply notification is unread but the shared cursor would yield zero, seed `manually_unread = true` so rollout does not erase known personal attention.

Initialization is atomic per conversation. Add `support_inbox_conversation_projection_states(conversation_id, generation, initialized_at, contribution_hash)` with a unique conversation/generation key, plus a contribution ledger containing the last shared and personal core-bucket contribution emitted for that conversation. Every dual-write mutation and every backfill batch follows the same `ensureV2Initialized` routine:

1. lock the conversation row;
2. if its current generation is already initialized, do not reconstruct or overwrite projection, personal state, or contributions;
3. otherwise derive projection and initial personal states from the locked legacy rows, insert the generation marker and full contribution ledger with `ON CONFLICT DO NOTHING`, and apply the initial core-bucket additions and ordered view-log entries exactly once in that transaction;
4. apply the requested live mutation after initialization, calculate its delta from the ledger, update the ledger, and commit both together.

Therefore a live mutation that reaches an unbackfilled conversation initializes it before applying its delta; a later backfill sees the marker and skips it. A backfill holding the lock finishes initialization before a live mutation can calculate its delta. Reruns never replace a newer read cursor, manual reminder, projection, or contribution. Uninitialized rows use legacy reads in legacy/shadow modes, and no workspace may enter V2 until every conversation in its cutover generation is initialized and counter reconciliation is exact.

Backfill uses `FOR UPDATE SKIP LOCKED` or an equivalent lease so multiple workers can cooperate, commits every batch, rate-limits itself, and can resume after interruption. Generation rollover is reserved for an explicit full rebuild; normal retries always reuse the active generation.

Existing saved views use a race-safe baseline-and-replay bootstrap after dual-write logging is active:

1. mark each `(view, mailbox scope)` or `(view, user, mailbox scope)` projection as `building` so normal workers retain its log entries without publishing a partial snapshot;
2. in a repeatable-read transaction, capture the applicable shared/personal head, evaluate the exact legacy predicate against that transaction's snapshot, and store the baseline count at the captured version;
3. replay retained log entries strictly from `baseline_version + 1` through the current head and advance only contiguous applied watermarks;
4. compare the caught-up projection to a fresh legacy count, then mark it `ready`;
5. retain change-log rows until every registered projection has passed them, with a time-based safety floor.

The migration enumerates every existing view and builds all mailbox-scope baselines plus overlays for every user who can currently access a relative/personal view. It is resumable and bounded by view/scope/user batches. A workspace cannot enter shadow or V2 while any existing view projection is uncompiled, missing, building, gapped, or mismatched. Users granted access later follow the same exact lazy bootstrap and receive legacy exact counts until their overlay is ready.

### Runtime modes

Use a workspace-aware mode with `legacy`, `shadow`, and `v2` behavior:

- `legacy`: legacy reads; new schema may still be dual-written;
- `shadow`: legacy response remains authoritative while v2 results, counters, and view membership are compared and measured;
- `v2`: personal reads, projected lists, buckets, typed realtime, and snapshot view counts are authoritative.

The release begins dual-writing before backfill, runs the backfill, reaches shadow parity, then switches workspaces to v2. A mode change back to legacy is the rollback. No legacy column or query is removed in the first release.

### Legacy dual-write and rollback semantics

All V2 operations continue maintaining `team_last_seen_at` for the entire V2 operating interval:

- a personal read advances the legacy cursor with the existing `MarkInternalRead` rule to the newest surviving readable customer reply and never moves it backwards, even when `through_message_id` is older;
- personal Mark unread applies the existing legacy `MarkUnread` rule and resets the shared cursor to epoch;
- a public teammate or AI reply continues applying the existing legacy read advancement;
- customer-message deletion leaves the timestamp cursor unchanged, matching the legacy comparison behavior; deleting the conversation removes it normally;
- duplicate reads, manual-unread requests, replies, deletions, and projector retries are idempotent on both schemas.

This mapping deliberately preserves the current shared legacy behavior, not per-user parity: while V2 is active, one user's read can make the hidden legacy state read for everyone and one user's Mark unread can make it unread for everyone. Switching a workspace back to `legacy` therefore restores coherent legacy shared semantics with a continuously maintained cursor; it does not promise to reproduce each user's V2 unread state. The rollback runbook states this user-visible downgrade explicitly, and switching forward again resumes the preserved V2 personal state.

## Observability and Release Gates

Record metrics by workspace-size band and runtime mode:

- list, detail, read, counter, mailbox-scope, and view-count latency;
- rows scanned and database time for support query families;
- counter transition volume, version gaps, and negative-delta rejection;
- counter reconciliation drift;
- personal-state shadow mismatches;
- projection shadow mismatches;
- saved-view membership and count mismatches;
- websocket events by class, target, recipient count, retry, and duplicate;
- list refetches caused by read actions;
- backfill throughput, lag, errors, and remaining rows;
- custom-view projection queue depth, age, per-scope watermark lag, and retained-log floor.

Cutover gates:

- zero negative counters or cross-tenant state rows;
- zero saved-view membership differences excluding the approved personal-unread change;
- no blue-dot mismatch in the shadow sample;
- personal-read concurrency tests pass under race/load execution;
- counter drift below 0.1% before reconciliation and zero after reconciliation;
- custom-view count convergence p99 under two seconds;
- a read action produces no workspace-wide list/count refetch;
- no increase in notification duplication or missed mentions;
- 10x load SLOs pass.

## Performance Acceptance Criteria

Test at minimum 300,000 conversations and 3,000,000 messages in one workspace, 100 connected agent sessions, 20 mailboxes, 50 saved views, and bursty customer replies/read actions.

Targets under representative warm-cache load:

- first conversation-list page: p95 <= 150 ms, p99 <= 300 ms;
- personal read mutation: p95 <= 75 ms, p99 <= 150 ms;
- core counter read: p95 <= 50 ms, p99 <= 100 ms;
- conversation/message commit including projection and core counters: p95 <= 100 ms excluding external delivery;
- websocket commit-to-client patch: p95 <= 250 ms;
- custom-view counter convergence: p95 <= 1 second, p99 <= 2 seconds;
- database work for one read is constant with respect to connected teammate count;
- authoritative-transaction and personal-state work for one customer reply is bounded by indexed personal contributors to that conversation, not workspace membership, saved-view subscribers, or connected clients;
- post-commit custom-view work for one customer reply is bounded by affected views plus directly affected users and never enumerates workspace membership.

## Test Matrix

### State transitions

Cover customer, teammate, AI, internal-note, deletion, resolve, reopen, spam, assignment, mailbox move, human takeover, AI handoff, AI failure, and AI resolution. Assert projection fields, blue dot, human attention, personal state, core buckets, one logical change with exactly the touched ordered scope entries, and outbox records together. Assert that the authoritative transaction never enumerates workspace members or saved-view users.

### Multiple agents

Cover two agents with different read positions, multiple tabs for one agent, assignment changes, opened-by ownership, historical mentions, unauthorized mailboxes, manual unread, teammate replies, and non-relevant viewers. Assert that one user's read never clears another user's state or notification.

### Concurrency and idempotency

Cover customer reply racing a read-through request, two reads in reverse order, duplicate message idempotency keys, duplicate outbox delivery, concurrent assignment/reply, retry after transaction failure, a live mutation racing first-time initialization, a rerun encountering newer personal state, two workers attempting the same scope, out-of-order log visibility, a missing sequence, and two backfill workers. Prove that initialization and contribution insertion happen once, a watermark never advances past a gap, and an idempotent replay never double-applies a delta. Run Go race tests for in-process caches/workers.

### Views and indicators

Use golden fixtures for every existing serialized filter key, operator, boolean combination, and built-in view. Assert unchanged membership, exact agreement between legacy evaluation and compiled shared-plus-personal counts, personal unread differences, shared blue-dot behavior, human-attention counts, AI Handling transitions, Support rail behavior, row typography, mailbox-access revocation with deterministic cache eviction, access-version rotation, component-wise vector comparison, incomparable-vector refetch, stale-version rejection, and count-patch convergence. Mutation tests change subject, customer name, and customer email through every supported write path and prove search-view membership/counts converge; the registry-coverage test fails for any accepted mutable filter field without a projector route.

### Migration and rollback

Test empty, partially backfilled, fully backfilled, interrupted, and rerun databases. Include writes before, during, and after conversation initialization and a saved-view baseline; late access grants; retained-log cleanup; and bootstrap retries. Exercise V2 personal reads, manual unread, teammate/AI replies, customer-message deletion, duplicate requests, rollback to the shared legacy cursor, and forward switch back to preserved personal state. Validate migration checksums, concurrent-index mode, legacy fallback, shadow mode, the initialized/ready/gap cutover gates, V2 cutover, and rollback with dual-written data intact.

## Recovery and Reconciliation

A scheduled reconciler recomputes projections and core counter buckets for sampled workspaces continuously and supports a full workspace repair command. Repairs are idempotent, versioned, and publish replacement snapshots rather than blind deltas.

If realtime delivery is unavailable, TanStack Query's bounded stale refresh restores state. If the custom-view worker is delayed, the last completed counts remain visible and queue-age monitoring alerts. If dual-write fails, the authoritative mutation fails rather than committing a message with inconsistent state; non-authoritative external notifications remain retryable after commit.

## Non-Goals

- Adding a follower/subscription UI.
- Changing saved-view membership or adding a user-facing unread filter in this release.
- Removing the legacy `team_last_seen_at` column during initial rollout.
- Replacing customer/widget read receipts.
- Redesigning presence, typing, drafts, assignment, SLA, or AI-routing UX.
- Making arbitrary custom-view count updates synchronous with every conversation event.

## Delivery Boundary

This is one product release and one coordinated implementation program. Internally, it is delivered in reversible stages so schema, backfill, shadow comparison, client compatibility, and cutover can be verified independently. The implementation plan must preserve these gates and must not collapse them into a destructive big-bang migration.
