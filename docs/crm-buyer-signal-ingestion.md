# CRM Buyer Signal Ingestion

Taxonomy note: the canonical platform model for agents, built-in automations, and automation rules now lives in `docs/AGENTS_AND_AUTOMATION.md`. This file remains the implementation detail reference for the buyer-signal ingestion pipeline itself.

This document explains the Phase 1a buyer-signal ingestion pipeline as it is implemented today. It is an internal engineering reference for understanding how synced CRM email becomes stored buyer signals, where idempotency is enforced, what gets skipped, and which parts of the product surface those signals.

## Purpose

Phase 1a is the first executable CRM intelligence slice.

The implemented pipeline does four things:

1. take already-stored CRM email messages
2. decide whether each message is eligible for signal detection
3. enqueue one Temporal workflow per eligible message
4. persist auditable buyer signals with provenance and duplicate suppression

This is a background system automation. It is not a user-visible agent class and it does not run through PM/support explicit agent queues.

Phase 1b contact/deal summaries consume these stored buyer signals and are documented separately in `docs/crm-entity-summaries.md`.

## Source of truth

Gmail is the source of truth for mailbox contents.

Buyer-signal ingestion does **not** read raw Gmail payloads directly. It runs from CRM-owned normalized email records already stored in Postgres:

- `crm_email_messages`
- `crm_email_threads`
- `crm_email_message_contacts`

The buyer-signal records themselves are stored in:

- `crm_buyer_signals`

## Main components

- `crm_email.go` persists manual outbound and manually created CRM email messages.
- `email_sync_activities.go` persists Gmail-synced messages.
- `crmsignal.IngestionService` decides whether a stored message should trigger signal detection and starts the workflow.
- `SignalDetectionWorkflow` runs the async detection path on the `automation-default` queue.
- `SignalDetectionService` calls the LLM and stores resulting buyer signals.
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

Buyer-signal ingestion is triggered only after a CRM email message has been successfully stored.

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

Buyer-signal ingestion is downstream from CRM email storage.

Important rules:

- if enqueue fails, the CRM email message is still stored
- if workflow execution fails, mailbox sync still succeeds
- if LLM detection fails, no signal row is written, but the email remains durable
- retries should not create duplicate buyer signals because persistence is idempotent

This separation is intentional. CRM email sync is the durability path. Signal detection is derived automation.

## Product surfaces that consume the signals

Current CRM surfaces reading stored buyer signals:

- CRM contact detail buyer-signal panel
- CRM deal detail buyer-signal panel
- CRM Insights page

The frontend now reads and renders provenance fields including:

- source type
- confidence
- evidence excerpt
- message direction
- participant count
- thread-linked state

The product does **not** yet expose a dedicated operational diagnostics surface for this automation. That belongs to a later `System Automations` settings surface.

## What this phase does not do

Phase 1a does not yet implement:

- contact summaries
- deal summaries
- autonomous deal creation
- autonomous stage progression
- review-feed population from runtime signals
- support-triggered signal ingestion
- calendar-triggered signal ingestion
- persisted skip counters for ambiguous multi-contact/no-deal messages

It is the ingestion foundation only.

## Current limitations and follow-ups

These are real current constraints, not future aspirations:

- multi-contact/no-deal threads are skipped entirely
- HTML fallback is not yet sanitized to plain text before prompt construction
- skipped-message instrumentation currently relies on structured logs, not durable counters
- repeated-thread suppression is thread-level only; it does not yet include deeper contact/deal semantic grouping
- workflow execution is async and auditable through code/logs, but not yet exposed through a dedicated admin automation UI

## How to debug this pipeline

When a signal is missing:

1. verify the CRM email message was actually stored
2. verify the message has a non-empty body
3. verify the message has exactly one associated contact or an attached deal
4. verify it was not a multi-contact/no-deal skip
5. check logs for:
   - `crm buyer signal ingestion skipped`
   - `crm buyer signal ingestion enqueued`
   - `failed to enqueue crm buyer signal detection`
6. check whether a same-thread same-type signal already exists in the last 24 hours

When duplicate signals appear:

- check whether they are from different source messages
- check whether the thread ID was missing, which weakens thread-level suppression
- check whether they are different signal types, which is allowed

## Related docs

- `docs/crm-email-sync.md`
- `.claude/plans/crm-phase-1-buyer-signal-ingestion.md`
