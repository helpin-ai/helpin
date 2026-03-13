# CRM Email Sync

This document explains how CRM email sync currently works in Teampulse as implemented in the backend today. It is an internal engineering reference for people debugging mailbox lifecycle, contact creation, message association, and inbox freshness.

## Source of truth

Gmail is the external source of truth for mailbox contents and mailbox history cursors.

Postgres stores Teampulse-owned normalized records for:

- connected mailbox accounts and lifecycle state
- durable Gmail history checkpoints
- synced threads and messages
- message-to-contact participant associations
- thread-level contact caches used for filtering and UI reads

The CRM UI reads from those CRM tables. It does not read directly from Gmail.

`crm_email_accounts.sync_state` is now also the persisted operational diagnostics record for sync runtime state. It stores the current phase, last attempt/success/failure timestamps, consecutive failure count, last error, last cycle summary, and the most recently persisted Gmail history checkpoint.

Buyer-signal ingestion from CRM email is documented separately in `docs/crm-buyer-signal-ingestion.md`.
Contact and deal summaries built on top of CRM email plus buyer signals are documented in `docs/crm-entity-summaries.md`.
This document focuses on mailbox lifecycle, contact creation, message association, and inbox freshness.

## Main subsystems

- `CRMEmailService` handles mailbox connect, reconnect, disconnect, purge, and manual send flows.
- `EmailSyncWorkflow` is the long-running Temporal workflow per mailbox.
- `EmailSyncActivities` does first-sync backfill, incremental sync, checkpoint persistence, and message storage.
- `GmailSyncClient` wraps Gmail APIs for mailbox profile, messages, history, token refresh, and send.
- `crmemail.Resolver` normalizes participants, matches contacts, and auto-creates contacts when allowed.
- `crmsignal.IngestionService` evaluates stored CRM email messages and enqueues buyer-signal detection for eligible messages.
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
7. If the mailbox row does not already have a checkpoint, the Gmail profile `historyId` is stored as `last_history_id`.
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
- cancels the running Temporal sync workflow for that mailbox

### Purge

Purge is separate from disconnect and is destructive.

On purge, the service:

- requires admin authorization
- cancels the running sync workflow
- deletes the mailbox row
- relies on database cascades to delete synced mailbox-owned email/calendar data

Purge does not delete CRM contacts, companies, or deals that were previously linked or created from synced emails.

## First sync and historical backfill

The Temporal workflow does two things:

1. run initial backfill once
2. then enter an infinite incremental sync loop that sleeps for 5 minutes between sync cycles

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

After the backfill finishes, the activity fetches the Gmail mailbox profile and persists the latest Gmail `historyId` as `last_history_id`. That becomes the durable checkpoint for future incremental sync.

## How messages are ingested on sync

Every synced Gmail message goes through `storeMessage`.

### Pre-ingestion filtering

Before anything is stored:

- duplicate messages are skipped if a message with the same `(email_account_id, message_external_id)` already exists
- sender-based filtering is applied through sync settings
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

That cache exists to support thread filtering and fast UI reads, but it is derived state. Message-contact association rows remain the source of truth.

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

The workflow currently sleeps 5 minutes between incremental cycles.

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

### Admin backend endpoints

There are now two admin-only backend endpoints for mailbox operations:

- `GET /api/crm/email/accounts/{id}/diagnostics`
- `POST /api/crm/email/accounts/{id}/maintenance/rebuild-associations`

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

## What happens after sync

After a successful message store or sync cycle, there is more bookkeeping than just inserting rows.

### Per-message effects

- the message is inserted once per mailbox external message ID
- participant-contact association rows are replaced for that message
- the thread contact cache is refreshed if the message belongs to a thread

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
- eligible stored CRM email messages can enqueue buyer-signal detection as a downstream automation
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

If sync data exists but message-contact associations are missing or thread caches look stale, run the rebuild-associations maintenance endpoint before considering a broader re-sync.

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
