# CRM conversation signal ingestion

This is the implementation-detail reference for conversation and email signal
extraction. The canonical cross-domain CRM-signals architecture, rule
catalogue, scoring, and activation model lives in
[`crm-signals.md`](crm-signals.md). The canonical platform model for
agents, built-in automations, and automation rules lives in
[`agents-and-automation.md`](agents-and-automation.md).

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

- `crm_signal_observations` for detector evidence
- `crm_signals` for interpreted commercial meaning

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
- manual outbound send and thread replies through `CRMEmailService`
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
2. otherwise convert `body_html` to plain text, excluding script, style and head content
3. collapse whitespace and truncate to 3,000 Unicode code points

The shared `crmtext` helpers normalize HTML and evidence text. This text
conversion is not a guarantee against prompt injection.

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
- `source_type` and `source_id` matching an input source (filled from the input when omitted for a single payload)
- `commercial`, including relevance, event, offering match and commercial consequence
- optional `timeline_date` for timeline signals

The detector loads seller/product and customer relationship context before
calling the model. A signal must qualify as commercially relevant, match the
offering, and have a specific commercial consequence. Missing product context
or an uncertain/irrelevant assessment prevents persistence. English narrative
validation applies to summaries and relevant commercial consequences.

Low-confidence signals are dropped before persistence.

Current threshold:

- ignore detections with confidence `< 0.6`
- require a nonempty evidence excerpt that matches normalized subject, body or
  thread context; the evidence excerpt is capped at 500 bytes before comparison

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
- `ingestion_version` and `detector_version` = `commercial-v4-en`
- `source_content_hash` and `evidence_verified`
- commercial relevance, event, consequence, offering match, relationship, motion
  and suggested action
- a valid parsed `timeline_date`, when supplied for a timeline signal

This makes runtime-generated signals auditable back to the CRM message and thread that produced them.

## Idempotency and duplicate suppression

There are two layers of duplicate control.

### 1. Source-level idempotency

`CRMSignalRepository.CreateSignalIfAbsent` uses the current observation and
interpretation path when the schema contains `commercial_motion`. It stores
fingerprinted detector evidence, resolves detection-time motion and a matching
versioned interpretation, then inserts interpreted signals with conflict
suppression. An observation without a matching interpretation can remain stored
without a visible signal.

Current uniqueness includes meaning fingerprints and entity identity, plus
rule/version, motion and evidence-fingerprint dimensions. Changed evidence or
interpretation can therefore produce a distinct result for the same source.
The older `(workspace_id, source_type, source_id, signal_type)` lookup belongs
to the compatibility path for schemas without `commercial_motion`; it is not
the full current contract.

### 2. Same-thread short-window suppression

Before storing a detected signal, the detector also checks whether the same signal type already exists on the same thread in a recent time window.

Current suppression window:

- 24 hours

This is implemented conservatively and only checks:

- same `workspace_id`
- same `source_thread_id`
- same `signal_type`
- `detected_at >= now - 24h`
- the current conversation-extraction rule/version and matching commercial event
- a different source message (the current source ID is excluded)

This reduces repeated events from noisy active threads.

After successful analysis, source reconciliation can supersede automated signal
types that were not retained. It preserves manual signals. Missing seller
context, uncertain relevance, unverified evidence or thread suppression can
disable reconciliation for that source rather than removing earlier evidence.

## Failure behavior

CRM-signal ingestion is downstream from CRM email storage.

Important rules:

- if enqueue fails, the CRM email message is still stored
- if workflow execution fails, mailbox sync still succeeds
- an LLM call or response-validation failure happens before signal writes; later
  persistence/reconciliation errors can occur after earlier signals were written
- mailbox email remains stored independently of detection success
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
persisted. Detection activities also record success/failure through the automation
health observer under `crm.buyer_signal_ingestion`; the automation overview
displays built-in CRM health. This is aggregate health, not a per-message
Temporal execution inspector.

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
- workflow execution is asynchronous; the automation overview exposes aggregate
  health, while individual execution diagnosis still requires workflow details
  and logs

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
6. verify commercial context, qualification, confidence and evidence matching
7. check same-thread suppression and whether a matching rule interpretation exists
8. inspect automation health and the Temporal execution; queued detection alone
   does not prove a signal was stored

When duplicate signals appear:

- check whether they are from different source messages
- check whether the thread ID was missing, which weakens thread-level suppression
- check whether they are different signal types, which is allowed

## Related docs

- [`crm-signals.md`](crm-signals.md)
- [`crm-email-sync.md`](crm-email-sync.md)
- [`crm-entity-summaries.md`](crm-entity-summaries.md)

Implementation references: [ingestion](../server/internal/crmsignal/ingestion.go),
[text conversion](../server/internal/crmtext/plain.go),
[detector](../server/internal/service/crm_signal_detection.go),
[commercial qualification](../server/internal/crmsignal/commercial.go), and
[Temporal workflow](../server/internal/temporalapp/signal_detection_workflow.go).

Persistence references: [signal repository](../server/internal/repository/crm_signal.go),
[interpretations](../server/internal/repository/crm_signal_interpretation.go), and
[automation overview](../frontend/src/components/automation/AutomationOverviewPanel.tsx).
