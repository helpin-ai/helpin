# CRM email synchronization

This document explains how CRM email sync currently works in Helpin as implemented in the backend today. It is an internal engineering reference for people debugging mailbox lifecycle, contact creation, message association, and inbox freshness.

## Source of truth

Gmail is the external source of truth for mailbox contents and mailbox history cursors.

Postgres stores Helpin-owned normalized records for:

- connected mailbox accounts and lifecycle state
- durable Gmail history checkpoints
- synced threads and messages
- message-to-contact participant associations
- thread-level contact caches used for filtering and UI reads

The CRM UI reads from those CRM tables. It does not read directly from Gmail.

`crm_email_accounts.sync_state` is now also the persisted operational diagnostics record for sync runtime state. It stores the current phase, last attempt/success/failure timestamps, consecutive failure count, last error, last cycle summary, and the most recently persisted Gmail history checkpoint.

The canonical taxonomy and platform model for agents, built-in automations, and automation rules now lives in `docs/agents-and-automation.md`.

This document focuses only on mailbox lifecycle, contact creation, message association, and inbox freshness.

## Main subsystems

- `CRMEmailService` handles mailbox connect, reconnect, disconnect, purge, and manual send flows.
- `EmailSyncWorkflow` is the long-running Temporal workflow per mailbox.
- `EmailSyncActivities` does first-sync backfill, incremental sync, checkpoint persistence, and message storage.
- `GmailSyncClient` wraps Gmail APIs for mailbox profile, messages, history, token refresh, and send.
- `crmemail.Resolver` normalizes participants, matches contacts, and auto-creates contacts when allowed.
- `crmsignal.IngestionService` evaluates stored CRM email messages and enqueues CRM-signal detection for eligible messages.
- `CRMSummaryService` requests and generates contact/deal summaries after email and signal activity.
- `CRMEmailRepository` persists accounts, threads, messages, message-contact links, and thread contact caches.

Primary implementation references:

- `server/internal/service/crm_email.go`
- `server/internal/temporalapp/email_sync_workflow.go`
- `server/internal/temporalapp/email_sync_activities.go`
- `server/internal/crmemail/resolver.go`
- `server/internal/repository/crm_email.go`
- `server/internal/sync/gmail.go`
- `server/internal/crmsignal/ingestion.go`
- `server/internal/service/crm_summary.go`

## Mailbox lifecycle

`CRMEmailAccount` is the mailbox record. The important lifecycle fields are:

- `status`
- `disconnected_at`
- `normalized_email_address`
- `last_history_id`
- `last_synced_at`
- `has_synced_data`

Supported mailbox states:

- `pending_oauth`
- `connected`
- `disconnected`
- `error`

Important distinction:

- `CRMEmailAccount.status` is the mailbox lifecycle state
- `sync_state.status` and `sync_state.phase` are the runtime sync diagnostics state

### Connect and reconnect

1. OAuth initiation creates a pending mailbox row with:
   - `status = pending_oauth`
   - `is_active = false`
   - a generated `oauth_state`
2. OAuth completion exchanges the auth code for tokens.
3. The service calls Gmail profile API to fetch:
   - canonical mailbox email address
   - current Gmail `historyId`
4. The mailbox email is normalized and used as the workspace mailbox identity.
5. If the same normalized mailbox already exists in the same workspace and provider, that existing row is reused instead of creating a duplicate mailbox.
6. The reused or newly completed mailbox row is updated with fresh tokens, active state, normalized email, and connected status.
7. For a new mailbox, the OAuth-time Gmail `historyId` is saved as `sync_state.initial_history_id`, leaving `last_history_id` empty so the first historical import runs. Reconnected mailbox checkpoints are preserved.
8. The sync workflow is started for that mailbox account ID.

Important consequence: reconnect is mailbox reuse, not mailbox recreation. The same mailbox record can be reactivated by another member in the same workspace.

### Disconnect

Disconnect is non-destructive to synced history.

On disconnect, the service:

- clears stored OAuth tokens
- clears token expiry metadata
- clears OAuth state
- sets `is_active = false`
- sets `status = disconnected`
- sets `disconnected_at`
- preserves synced threads, messages, associations, calendar artifacts, and `last_history_id`
- requests cancellation of the running Temporal sync workflow; cancellation errors are logged and do not prevent saving disconnected state

### Purge

Purge is separate from disconnect and is destructive.

On purge, the service:

- requires admin authorization
- requests cancellation of the running sync workflow (errors are logged)
- deletes the mailbox row
- relies on database cascades for related rows; it does not explicitly delete each email/calendar table

Purge does not delete CRM contacts, companies, or deals that were previously linked or created from synced emails.

**Schema limitation:** the current versioned foundation creates email threads,
messages and calendar events without mailbox foreign-key cascades. Older
`server/migrations/027_crm_email_calendar.sql` and lifecycle test fixtures include
those constraints, so their purge behavior is not proof of the foundation
installation’s behavior. Do not treat mailbox deletion as verified removal of all
synced content. Inspect the installation’s constraints and remaining rows; the
schema/cleanup gap needs a separate implementation fix.

## First sync and historical backfill

The Temporal workflow does two things:

1. run initial backfill once
2. then wait up to 5 minutes between incremental cycles, waking earlier for an `email-sync-now` signal. An explicit initial mode can select incremental or historical sync instead of the normal first backfill.

### Backfill flow

When `BackfillEmailsActivity` starts:

1. Load the mailbox account.
2. Exit early if the mailbox is inactive.
3. Load sync settings for the workspace, falling back to defaults if settings do not exist.
4. Get a valid Gmail access token, refreshing if needed.

Then the backfill logic branches:

- If `last_history_id` already exists on the mailbox, the activity does not do a fresh historical import. It persists the checkpoint state and returns.
- Otherwise it computes a historical start time from sync settings:
  - default is 90 days
  - `HistoricalSyncDays` overrides that
  - if `last_synced_at` is more recent than the historical window, that more recent time is used

The activity then queries Gmail with `after:<unix timestamp>` and pages through message results. Each Gmail message is fetched in full and passed through `storeMessage`.

After backfill, the activity fetches the mailbox profile and selects the checkpoint in this order: an existing `last_history_id`, the saved `sync_state.initial_history_id`, then the latest profile `historyId`. Keeping the OAuth-time cursor allows incremental sync to catch messages arriving during the initial import.

A manual `historical` sync uses `HistoricalBackfillEmailsActivity`: it processes the configured history window even when a checkpoint exists, without shortening that window to `last_synced_at` or replacing a healthy checkpoint. Existing message IDs still prevent duplicate inserts.

## How messages are ingested on sync

Every synced Gmail message goes through `storeMessage`.

### Pre-ingestion filtering

Before anything is stored:

- duplicate messages are skipped if a message with the same `(email_account_id, message_external_id)` already exists
- allow/block filtering is applied to external participants through sync settings, including outbound recipients
- internal-email exclusion is applied if all participants share the mailbox domain and the workspace setting says to exclude internal email

### Direction detection

Direction is determined from Gmail labels, not from comparing `From` to the mailbox address:

- message with `SENT` label => `outbound`
- otherwise => `inbound`

This avoids the old alias-based direction bug and makes direction depend on Gmail’s mailbox classification.

### Thread handling

If Gmail provides a thread ID:

- the code finds or creates the CRM email thread for `(email_account_id, thread_external_id)`
- updates thread counters
- updates `last_message_at`

## How contacts are created on first sync and later syncs

Contact resolution is shared by Gmail sync and manual send. There is one resolver pipeline.

### Participant extraction and normalization

The resolver receives:

- one `from` participant
- zero or more `to` participants
- zero or more `cc` participants
- mailbox self addresses
- sync direction
- workspace sync settings

Participants are normalized as follows:

- email addresses are parsed and normalized to lowercase mailbox addresses
- invalid addresses are dropped
- duplicate role+email combinations are collapsed
- self mailbox addresses are excluded from CRM contact creation and CRM contact association

That means malformed headers do not become raw CRM contact emails.

### Contact matching

For all unique external participant emails, the resolver performs workspace-scoped exact matching by normalized email. This is not a fuzzy search flow.

### Auto-create rules

If a participant does not match an existing contact, the resolver may auto-create a contact depending on sync settings:

- `disabled`: do not auto-create, only match existing contacts
- `selective`: auto-create only for outbound external participants
- `always`: auto-create for any external participant

Auto-create is also blocked when the email local-part matches a configured blocked/system prefix.

### Contact creation details

When a contact is created from email sync:

- a valid CRM `display_id` is allocated first
- the normalized email is stored on the contact
- the name is derived from participant display name when present
- otherwise the name is derived from a humanized version of the email local-part
- the contact source is recorded as `email_sync`
- the contact is created in the same workspace as the mailbox

This same resolver path applies:

- during first backfill
- during ongoing incremental sync
- during manual outbound send persistence

So new contacts can continue to appear after first sync whenever new external participants arrive and the workspace creation mode allows it.

## How emails are associated to contacts

The system no longer treats an email as belonging to exactly one contact.

### Message-level association model

For each synced message, the resolver returns:

- normalized participants
- all resolved CRM contact links
- `contact_ids` for the message
- `primary_contact_id` only when there is exactly one associated external contact

The message is then stored with:

- `contact_id` as a backward-compatible convenience field
- `contact_ids` populated in the API response layer
- participant associations persisted in `crm_email_message_contacts`

Important rule:

- if exactly one external contact is associated, `contact_id` is set to that contact
- if multiple contacts are associated, `contact_id` is `null`

The join table is the real source of truth for multi-contact emails.

### Thread-level denormalization

After message associations are written, the repository rebuilds the thread’s `contact_ids` cache from all message-contact links in that thread.

That cache exists to support thread filtering and fast UI reads, but it is derived state. Message-contact association rows remain the source of truth. Cache refresh is a separate step whose failures are logged; it is not atomic with the message insert.

### Query behavior

Inbox/contact filtering must not rely only on legacy `contact_id`.

Current filtering uses:

- message-contact association rows for message-level membership
- thread `contact_ids` cache plus association existence checks for thread-level membership

This is why one email can legitimately appear under multiple contacts when multiple external participants were involved.

## How the inbox stays in sync

After initial backfill, the workflow enters a perpetual incremental sync loop.

### Incremental sync flow

Each sync cycle:

1. loads the mailbox account
2. exits early if the mailbox is inactive
3. loads workspace sync settings
4. obtains a valid Gmail token
5. reads `last_history_id`
6. calls Gmail History API with that checkpoint
7. collects all changed message IDs since that checkpoint
8. fetches full Gmail message details for each changed message ID
9. passes each message through the same `storeMessage` pipeline used by backfill
10. persists the newest Gmail `historyId` after the cycle completes

The workflow waits up to 5 minutes between incremental cycles.
`POST /api/crm/email/accounts/{id}/sync` accepts `incremental` (the default) or
`historical` mode. It requires CRM edit permission plus mailbox ownership or
admin access, and the mailbox must be connected. The request signals the workflow
or starts it if needed; queued status does not mean synchronization has completed.

### Recovery when Gmail history expires

Gmail history cursors can become invalid or too old. When Gmail returns a stale-history style error:

1. the sync activity runs a bounded recovery backfill
2. the recovery start time is the later of:
   - historical sync window start
   - existing `last_synced_at`
3. the activity reprocesses messages from that bounded window
4. it fetches the latest mailbox profile again
5. it resets `last_history_id` to the current mailbox `historyId`

This lets sync resume without dropping the mailbox permanently into a broken state.

## Sync diagnostics and repair

Phase 0 added backend-only diagnostics and repair tooling for mailbox sync.

### Persisted diagnostics

Every backfill, incremental sync, recovery run, disconnect, and reconnect updates normalized fields inside `sync_state`.

Key diagnostics fields:

- `status`
- `phase`
- `last_attempt_at`
- `last_success_at`
- `last_failure_at`
- `consecutive_failures`
- `last_history_id`
- `last_error`
- `last_cycle`

`last_cycle` stores aggregate counters for the most recent completed cycle:

- mode
- started/completed timestamps
- messages seen
- messages stored
- duplicates skipped
- filtered skipped
- internal skipped
- contacts created
- associations written
- threads touched
- recovery triggered

The diagnostics payload intentionally avoids storing email subject/body or raw participant lists.

### Diagnostics and repair endpoints

- `GET /api/crm/email/accounts/{id}/diagnostics` requires CRM read permission and mailbox ownership or admin access.
- `POST /api/crm/email/accounts/{id}/maintenance/rebuild-associations` requires CRM admin permission.

The diagnostics endpoint returns:

- mailbox lifecycle fields
- normalized sync diagnostics from `sync_state`
- synced data counts for threads, messages, and calendar events
- association health summary for missing message-contact rows and stale thread caches

The maintenance endpoint repairs mailbox consistency by:

- finding stored messages missing participant associations
- re-running participant resolution from stored message data
- rewriting `crm_email_message_contacts`
- recomputing legacy `contact_id`
- rebuilding affected thread `contact_ids`

This repair selects messages missing associations and refreshes threads touched
by those repairs. It is not a general rebuild of every stale thread cache, and
participant resolution can create contacts under the workspace’s creation policy.

## What happens after sync

After a successful message store or sync cycle, there is more bookkeeping than just inserting rows.

### Per-message effects

- the message is inserted once per mailbox external message ID
- participant-contact association rows are replaced for that message
- the thread contact cache is refreshed if the message belongs to a thread

Message insertion and association replacement are separate writes. If association
replacement fails after insertion, a retry can skip the existing message; use
the association repair endpoint to investigate missing links. Thread-count/cache
updates, signal enqueue and summary-refresh failures are logged separately and
do not necessarily fail the sync cycle. A stored email is not proof that these
downstream effects completed.

Summary refresh chooses a linked deal first, otherwise a single associated
contact. Multi-contact synced mail without a deal does not automatically request
a summary for every participant.

### Per-account effects

When checkpoints are persisted:

- `last_synced_at` is updated
- `last_history_id` is updated when a history cursor is available
- `status` is moved or kept at `connected`
- `is_active` is kept true
- `disconnected_at` is cleared on active sync
- `sync_state` is updated with:
  - current sync phase/status
  - current history checkpoint
  - last attempt/success/failure timestamps
  - failure count and last error
  - last cycle aggregate stats

### Product-visible effects

Once the message and associations are stored:

- the email becomes visible in CRM inbox and contact/deal email views
- multi-contact messages are queryable from all associated contacts
- thread contact membership is reflected in thread filters
- eligible stored CRM email messages can enqueue CRM-signal detection as a downstream automation
- reconnect resumes the same mailbox history if the mailbox was disconnected but not purged

## Troubleshooting and debugging notes

### A contact was not auto-created

Check these first:

- the participant email may have been invalid and dropped during normalization
- the participant may have been the mailbox’s own email and therefore excluded
- sync settings may be `disabled`
- sync settings may be `selective` and the message may have been inbound
- the address may match a blocked/system prefix
- the email may have been filtered out or classified as internal before resolver logic ran

### An email appears under multiple contacts

That is expected for multi-party emails. The message-contact join table stores all external CRM participants. The legacy single `contact_id` field is not the source of truth.

### `contact_id` is null even though contacts are associated

That is expected when more than one external contact is associated to the message. Use association rows or `contact_ids`, not `contact_id`, to reason about membership.

### Disconnect and reconnect behavior

Disconnect keeps mailbox history and checkpoint. Reconnect reuses the same workspace mailbox record when the normalized Gmail address matches. Sync then resumes from the preserved checkpoint when possible.

### Gmail history expired

If Gmail rejects the saved `last_history_id`, the system falls back to bounded recovery backfill and then captures a fresh mailbox `historyId`.

### How to debug a failing mailbox now

Start with the diagnostics endpoint, not direct database inspection.

Look at:

- `sync.phase`
- `sync.last_error`
- `sync.consecutive_failures`
- `sync.last_success_at`
- `sync.last_cycle`

If stored messages are missing associations, use the admin rebuild-associations
endpoint and inspect its repaired-message and refreshed-thread counts. A stale
cache on a thread with complete associations may not be touched by this repair.

## Related implementation files

- `server/internal/service/crm_email.go`
- `server/internal/temporalapp/email_sync_workflow.go`
- `server/internal/temporalapp/email_sync_activities.go`
- `server/internal/crmemail/resolver.go`
- `server/internal/crmemail/diagnostics.go`
- `server/internal/repository/crm_email.go`
- `server/internal/sync/gmail.go`
- `server/internal/model/crm_email.go`
- `server/internal/model/crm_email_sync_settings.go`

Source comparison: [mailbox lifecycle](../server/internal/service/crm_email.go),
[workflow scheduling](../server/internal/temporalapp/email_sync_workflow.go),
[sync activities](../server/internal/temporalapp/email_sync_activities.go), and
[route permissions](../server/internal/router/router.go).
