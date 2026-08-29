# CRM Conversation Signal Ingestion

This is the implementation-detail reference for conversation and email signal
extraction. The canonical cross-domain CRM-signals architecture, rule
catalogue, scoring, and activation model lives in
[`crm-signals.md`](crm-signals.md). The canonical platform model for
agents, built-in automations, and automation rules lives in
[`AGENTS_AND_AUTOMATION.md`](AGENTS_AND_AUTOMATION.md).

This document explains how synced CRM email becomes stored LLM-extracted CRM
signals, where idempotency is enforced, what gets skipped, and which product
surfaces consume those signals. It does not describe deterministic Postgres or
ClickHouse rules.

## Purpose

Conversation ingestion is one producer within the broader CRM intelligence
system.

The implemented pipeline does four things:

1. take already-stored CRM email messages
2. decide whether each message is eligible for signal detection
3. enqueue one Temporal workflow per eligible message
4. persist auditable CRM signals with provenance and duplicate suppression

This is a background system automation. It is not a user-visible agent class and it does not run through PM/support explicit agent queues.

Contact, company, and deal summaries consume these stored CRM signals and are
documented separately in
[`crm-entity-summaries.md`](crm-entity-summaries.md).

## Source of truth

Gmail is the source of truth for mailbox contents.

CRM-signal ingestion does **not** read raw Gmail payloads directly. It runs from CRM-owned normalized email records already stored in Postgres:

- `crm_email_messages`
- `crm_email_threads`
- `crm_email_message_contacts`

The CRM-signal records themselves are stored in:

- `crm_signals`

## Main components

- `crm_email.go` persists manual outbound and manually created CRM email messages.
- `email_sync_activities.go` persists Gmail-synced messages.
- `crmsignal.IngestionService` decides whether a stored message should trigger signal detection and starts the workflow.
- `SignalDetectionWorkflow` runs the async detection path on the `automation-default` queue.
- `SignalDetectionService` calls the LLM and stores resulting CRM signals.
- `CRMSignalRepository` enforces source-level idempotency and repeated-thread suppression checks.

Primary implementation files:

- `server/internal/crmsignal/ingestion.go`
- `server/internal/service/crm_signal_detection.go`
- `server/internal/temporalapp/signal_detection_workflow.go`
- `server/internal/service/crm_email.go`
- `server/internal/temporalapp/email_sync_activities.go`
- `server/internal/repository/crm_signal.go`
- `server/internal/model/crm_signal.go`
- `server/internal/model/crm_signal_source.go`

## Trigger points

CRM-signal ingestion is triggered only after a CRM email message has been successfully stored.

Current trigger points:

- Gmail sync path in `EmailSyncActivities.storeMessage`
- manual outbound send in `CRMEmailService.SendEmail`
- manual message create in `CRMEmailService.CreateMessage`

It is intentionally **not** triggered from:

- duplicate-message skips
- filtered/internal-message skips
- maintenance rebuild of email-contact associations

That keeps signal detection downstream from email durability and avoids replaying LLM work during repair operations.

## Queue and workflow model

This automation uses Temporal.

Current behavior:

- task queue: `automation-default`
- workflow: `SignalDetectionWorkflow`
- workflow ID: `crm-signal-email-{message_id}`
- source unit: one workflow per eligible CRM email message

The workflow input is a list of `SignalSourcePayload`, but the current CRM email implementation always enqueues a single payload per message.

Idempotency at the workflow-start layer is handled by deterministic workflow IDs. If the same message is enqueued again while that workflow identity is already in use, the start is treated as a no-op.

## Eligibility rules

`crmsignal.IngestionService` evaluates each stored CRM email message before starting the workflow.

The message is skipped when:

- the message cannot be loaded
- the preferred body is empty
- there are no associated external contacts and no linked deal
- there are multiple associated contacts and no linked deal
- the ingestion starter is not configured

Current precision-first policy:

- single-contact messages are eligible
- deal-linked messages are eligible, even if multi-contact
- multi-contact, no-deal messages are skipped in Phase 1a

That suppression is deliberate. The current implementation optimizes for trust and lower false positives over broad B2B buying-committee coverage.

## How the payload is built

The ingestion service builds the signal payload from stored CRM data, not provider payloads.

For email messages it includes:

- `source_type = email`
- `source_id = crm_email_messages.id`
- `source_thread_id = crm_email_messages.thread_id`
- `source_thread_external_id = crm_email_threads.thread_external_id` when available
- `workspace_id`
- `contact_id` when exactly one associated contact exists
- `deal_id` when already linked
- subject from the message, or thread subject as fallback
- preferred message body
- normalized participants
- message direction
- occurrence timestamp
- bounded thread context from recent earlier messages

### Preferred body

The body selection rule is:

1. `body_text` if present
2. otherwise `body_html`
3. truncate to 3000 characters

Note: the current implementation falls back to stored HTML string directly. It does **not** yet sanitize or convert HTML to plain text before LLM use.

### Participants

Participants come from the stored normalized message fields:

- `from_address`
- `to_addresses`
- `cc_addresses`

Each participant is represented with:

- email
- optional name
- participant role

### Thread context

If the message belongs to a CRM email thread, the ingestion service loads up to 3 earlier messages from that thread and formats a bounded context block.

Current limits:

- up to 3 prior messages
- up to 1500 characters aggregate context

This context is intended to improve signal detection without turning the workflow into whole-thread summarization.

## Detection and persistence

The Temporal workflow calls `SignalDetectionService.DetectSignals`.

### LLM contract

The LLM is asked to return a JSON array of detected signals with:

- `signal_type`
- `summary`
- `confidence`
- `raw_evidence`

Very low-confidence signals are dropped before persistence.

Current threshold:

- ignore detections with confidence `< 0.3`

### Stored provenance

When a signal is written, the following provenance fields are persisted:

- `source_type`
- `source_id`
- `source_thread_id`
- `evidence_excerpt`
- `metadata`

Current metadata contents:

- `message_direction`
- `participant_count`
- `thread_external_id`
- `ingestion_version = phase1a`

This makes runtime-generated signals auditable back to the CRM message and thread that produced them.

## Idempotency and duplicate suppression

There are two layers of duplicate control.

### 1. Source-level idempotency

`CRMSignalRepository.CreateSignalIfAbsent` prevents storing the same signal type twice for the same source message.

Logical uniqueness is:

- `(workspace_id, source_type, source_id, signal_type)`

This protects against:

- workflow retries
- reconnect/recovery reprocessing
- repeated enqueue attempts for the same message

### 2. Same-thread short-window suppression

Before storing a detected signal, the detector also checks whether the same signal type already exists on the same thread in a recent time window.

Current suppression window:

- 24 hours

This is implemented conservatively and only checks:

- same `workspace_id`
- same `source_thread_id`
- same `signal_type`
- `detected_at >= now - 24h`

This reduces repeated budget/timeline/champion/risk events from noisy active threads.

## Failure behavior

CRM-signal ingestion is downstream from CRM email storage.

Important rules:

- if enqueue fails, the CRM email message is still stored
- if workflow execution fails, mailbox sync still succeeds
- if LLM detection fails, no signal row is written, but the email remains durable
- retries should not create duplicate CRM signals because persistence is idempotent

This separation is intentional. CRM email sync is the durability path. Signal detection is derived automation.

## Product surfaces that consume the signals

Current CRM surfaces reading stored CRM signals:

- CRM contact detail CRM-signal panel
- CRM company detail CRM-signal panel
- CRM deal detail CRM-signal panel
- ranked workspace feed on CRM Insights
- account and meeting signal briefs

The frontend now reads and renders provenance fields including:

- source type
- confidence and business priority
- evidence excerpt
- detector, domain, polarity, and rule version
- identity method and trust
- score factors and activation blockers

Rule feedback is available through the precision endpoint and evaluator runs are
persisted. There is no separate system-automation diagnostics UI for the
conversation workflow itself.

## Scope boundary

This document only covers conversation extraction from stored CRM email. It
does not describe:

- support, PM, relationship, and deal-state rules;
- ClickHouse behavioral rules;
- ranking and composition;
- routing and activation;
- external evidence ingestion; or
- contact, company, and deal summaries.

Those implemented paths are covered by
[`crm-signals.md`](crm-signals.md) and
[`crm-entity-summaries.md`](crm-entity-summaries.md).

## Current limitations and follow-ups

These are real current constraints, not future aspirations:

- multi-contact/no-deal threads are skipped entirely
- skipped-message instrumentation currently relies on structured logs, not durable counters
- repeated-thread suppression remains deliberately conservative; evidence
  fingerprints provide an additional material-change boundary
- workflow execution is async and auditable through code/logs, but not yet exposed through a dedicated admin automation UI

## How to debug this pipeline

When a signal is missing:

1. verify the CRM email message was actually stored
2. verify the message has a non-empty body
3. verify the message has exactly one associated contact or an attached deal
4. verify it was not a multi-contact/no-deal skip
5. check logs for:
   - `crm CRM signal ingestion skipped`
   - `crm CRM signal ingestion enqueued`
   - `failed to enqueue crm CRM signal detection`
6. check whether a same-thread same-type signal already exists in the last 24 hours

When duplicate signals appear:

- check whether they are from different source messages
- check whether the thread ID was missing, which weakens thread-level suppression
- check whether they are different signal types, which is allowed

## Related docs

- [`crm-signals.md`](crm-signals.md)
- [`crm-email-sync.md`](crm-email-sync.md)
- [`crm-entity-summaries.md`](crm-entity-summaries.md)
