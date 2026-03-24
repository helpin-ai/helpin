# CRM Entity Summaries

Taxonomy note: the canonical platform model for agents, built-in automations, and automation rules now lives in `docs/AGENTS_AND_AUTOMATION.md`. This file remains the implementation detail reference for CRM summary generation.

This document explains the Phase 1b CRM summary pipeline as implemented today. It covers how contact and deal summaries are stored, what triggers refreshes, how generation runs, what data is included, and where the results appear in the product.

The scope of this document is intentionally narrow:

- `contact` summaries
- `deal` summaries

It does not cover company summaries, support summaries, or summary-driven write-back automation because those do not exist in this phase.

## Purpose

Phase 1b turns CRM activity into durable, system-owned summary artifacts.

The implementation goal is:

1. generate a current summary for a contact or deal from stored CRM evidence
2. refresh that summary automatically as new CRM email activity and buyer signals arrive
3. preserve the last good summary when a refresh fails
4. surface the summary on the existing CRM detail pages

This is a background system automation. It is not:

- a PM/support explicit agent
- a user-configurable custom agent
- a suggestion feed item
- a record update mechanism

## What shipped in Phase 1b

Phase 1b added the following concrete capabilities:

- durable summary storage for `contact` and `deal` entities
- event-driven summary refresh requests from:
  - CRM email persistence
  - buyer-signal persistence
- deterministic Temporal workflows for per-entity summary generation
- daily reconciliation for:
  - open deals
  - recently touched contacts
- read APIs for contact and deal summaries
- CRM detail-page UI cards for viewing summary state and content
- failure handling that preserves the last good summary

Files added or primarily introduced for this phase:

- `server/internal/model/crm_summary.go`
- `server/internal/repository/crm_summary.go`
- `server/internal/repository/crm_summary_schema.go`
- `server/internal/service/crm_summary.go`
- `server/internal/temporalapp/crm_summary_workflow.go`
- `server/internal/handler/crm_summary.go`
- `server/migrations/041_crm_entity_summaries.sql`
- `frontend/src/components/crm/EntitySummaryCard.tsx`

Files updated to trigger or consume summary refreshes:

- `server/internal/service/crm_email.go`
- `server/internal/temporalapp/email_sync_activities.go`
- `server/internal/service/crm_signal.go`
- `server/internal/service/crm_signal_detection.go`
- `server/cmd/api/main.go`
- `server/cmd/temporal-worker/main.go`
- `frontend/src/hooks/queries/useCRM.ts`
- `frontend/src/lib/services/crmService.ts`
- `frontend/src/lib/crmTypes.ts`
- `frontend/src/pages/crm/ContactDetail.tsx`
- `frontend/src/pages/crm/DealDetail.tsx`

## Architecture at a glance

The summary system is a downstream intelligence layer on top of CRM email sync and buyer-signal ingestion.

High-level flow:

1. CRM email sync or manual CRM email persistence stores a message.
2. Buyer-signal ingestion may later store structured signals from that message.
3. Either of those events requests a summary refresh for the affected contact or deal.
4. The request upserts `crm_entity_summaries` and starts a deterministic Temporal workflow.
5. The workflow waits through a short debounce window.
6. The summary service assembles fresh evidence from CRM-owned data.
7. One LLM-backed generation step produces:
   - `summary_markdown`
   - `highlights`
8. The generated artifact is persisted and exposed through read APIs and CRM detail views.

This means summaries are:

- derived from stored CRM data, not provider payloads
- eventually consistent, not inline blocking work
- owned by the CRM system, not by the agent UI layer

## Runtime architecture

### Data plane

The summary data plane is:

- source records:
  - `crm_email_messages`
  - `crm_email_message_contacts`
  - `crm_buyer_signals`
  - CRM contacts, deals, companies, and associations
- derived record:
  - `crm_entity_summaries`

The summary row is the only persisted summary artifact. Prompt inputs and assembled evidence are reconstructed at generation time and are not durably stored as separate documents.

### Control plane

The control plane is:

- refresh request APIs inside `CRMSummaryService`
- deterministic Temporal workflow IDs per entity
- daily reconciliation cron workflow
- status transitions on the summary row

This keeps triggering, debouncing, and recovery behavior separate from the summary content itself.

### Execution model

The execution model is:

- request refresh cheaply and often
- coalesce bursts with workflow debounce
- recompute from source-of-truth CRM state
- overwrite the durable summary artifact
- preserve prior good content if refresh fails

That is the right shape for a system artifact that should stay current without becoming another mutable record type.

## Stored artifact

Summaries are stored in `crm_entity_summaries`.

There is exactly one row per:

- `workspace_id`
- `entity_type`
- `entity_id`

Current supported `entity_type` values:

- `contact`
- `deal`

The stored fields are:

- `summary_markdown`
- `highlights`
- `status`
- `computed_at`
- `source_window_start`
- `source_window_end`
- `last_triggered_at`
- `last_error`
- `metadata`

The table is modeled by:

- `server/internal/model/crm_summary.go`

The repository is:

- `server/internal/repository/crm_summary.go`

Reference migration:

- `server/migrations/041_crm_entity_summaries.sql`

## Summary statuses

The summary row uses four statuses:

- `pending_refresh`
- `ready`
- `stale`
- `error`

Current meaning:

- `pending_refresh`: a newer trigger exists than the latest satisfied computation
- `ready`: the latest computation satisfied the latest trigger
- `stale`: the latest refresh failed, but a prior usable summary still exists
- `error`: the latest refresh failed and there is no usable summary content

Important behavior:

- refresh failures do not delete the prior good summary
- the UI can keep rendering the last good summary while showing that refresh is stale

## Main components

- `CRMSummaryService` owns refresh requests, evidence assembly, generation, and persistence.
- `CRMSummaryRepository` stores summary rows and daily reconciliation inputs.
- `CRMEntitySummaryWorkflow` debounces per-entity refreshes and recomputes summaries.
- `CRMSummaryDailyReconciliationWorkflow` requests refreshes for open deals and recently touched contacts.
- `CRMSummaryHandler` exposes read APIs.

Primary implementation files:

- `server/internal/service/crm_summary.go`
- `server/internal/repository/crm_summary.go`
- `server/internal/model/crm_summary.go`
- `server/internal/temporalapp/crm_summary_workflow.go`
- `server/internal/handler/crm_summary.go`

## End-to-end sequence

### Email-driven path

1. A Gmail sync cycle or manual CRM email flow stores a CRM email message.
2. Message associations are resolved and written first.
3. The email service or sync activity decides whether a summary refresh should be requested:
   - always for linked deals
   - only for exactly one associated external contact
4. `CRMSummaryService` upserts the summary row with `pending_refresh`.
5. Temporal runs the per-entity workflow after debounce.
6. The workflow calls back into `CRMSummaryService.RefreshSummary`.
7. The service gathers entity snapshot + recent evidence.
8. The LLM returns JSON output.
9. The summary row is updated to `ready` or remains `pending_refresh` if a newer trigger landed during execution.

### Signal-driven path

1. Buyer-signal ingestion persists a new `crm_buyer_signals` row.
2. The signal service requests summary refresh for any linked contact and/or deal.
3. The same summary workflow path runs as above.

### Daily reconciliation path

1. The cron workflow runs once per day.
2. It enumerates:
   - open deals
   - recently touched contacts
3. It requests refresh for each entity through the same request service.
4. The normal debounced workflow pipeline takes over.

## Trigger points

Summary refreshes are requested from exactly two runtime sources:

1. after CRM email message persistence and association resolution
2. after successful buyer-signal persistence

Current email trigger points:

- Gmail sync path in `EmailSyncActivities.storeMessage`
- manual outbound send in `CRMEmailService.SendEmail`
- manual message create in `CRMEmailService.CreateMessage`

Current signal trigger points:

- detected buyer signals in `SignalDetectionService`
- manually created buyer signals in `CRMSignalService`

## Trigger rules

### From CRM email

When a CRM email message is stored:

- if `message.deal_id` is present, request a `deal` summary refresh
- if the message has exactly one associated external contact, request a `contact` summary refresh
- if the message has multiple contacts and no linked deal, skip contact refresh for now

That last rule is the current precision-first policy. It avoids ambiguous contact attribution at the summary layer.

### From buyer signals

When a buyer signal is persisted:

- if `contact_id` exists, request a contact summary refresh
- if `deal_id` exists, request a deal summary refresh

Signals are therefore one of the main ways a summary stays current even if the raw email body is older than the most recent structured intelligence.

## Refresh request flow

Refresh requests go through:

- `CRMSummaryService.RequestContactRefresh`
- `CRMSummaryService.RequestDealRefresh`

The request flow does two things:

1. upsert the summary row and set `status = pending_refresh`
2. start a deterministic Temporal workflow for that entity

Workflow IDs:

- `crm-contact-summary-{contact_id}`
- `crm-deal-summary-{deal_id}`

Queue:

- `automation-default`

If the workflow is already running, the start attempt is treated as a no-op. The newer request is still preserved by updating `last_triggered_at` on the row.

## Workflow behavior

The Temporal workflow is intentionally simple.

`CRMEntitySummaryWorkflow`:

1. sleeps for a fixed `2 minute` debounce window
2. calls `CRMSummaryActivities.RefreshSummaryActivity`
3. if a newer refresh request arrived while the workflow was running, it `continue-as-new`s and recomputes again

This gives contact/deal summaries a coalescing behavior without introducing a separate queue or aggregation table.

There is also a daily cron workflow:

- workflow: `CRMSummaryDailyReconciliationWorkflow`
- cron schedule: `0 3 * * *`

It requests refreshes for:

- all open deals
- contacts with CRM email or buyer-signal activity in the last `30 days`

The daily job does not generate summaries directly. It reuses the same request flow as event-driven refreshes.

## Evidence assembly

Summary generation works only from CRM-owned stored data.

It does not read Gmail payloads directly and it does not depend on frontend state.

### Contact summaries

Contact summaries assemble:

- contact snapshot
- linked companies
- up to `5` open associated deals
- up to `12` recent associated emails from the last `45 days`
- up to `10` buyer signals from the last `60 days`

### Deal summaries

Deal summaries assemble:

- deal snapshot
- linked contacts
- linked companies
- up to `16` recent associated emails from the last `60 days`
- up to `10` buyer signals from the last `60 days`

### Email evidence rules

For each stored CRM email used as evidence:

- prefer `body_text`
- otherwise fall back to simplified `body_html`
- strip HTML tags
- normalize whitespace
- truncate the body snippet before prompt assembly

The summary artifact itself does not store raw email bodies or long excerpts. Only derived output and compact operational metadata are persisted.

## LLM contract

The LLM receives a JSON payload describing the entity and recent evidence.

The required output is JSON only:

- `summary_markdown`
- `highlights`

Allowed highlight kinds:

- `momentum`
- `risk`
- `next_step`
- `stakeholder`
- `signal`

Generation rules enforced in code and prompt:

- summarize current momentum and key recent changes
- highlight risks and likely next step
- stay grounded in provided evidence
- do not invent CRM state changes
- do not copy long verbatim email excerpts
- keep at most `5` highlights

The service also normalizes the LLM output:

- trims markdown length
- drops invalid highlight kinds
- trims overly long highlights

## Metadata and source windows

Each generated summary stores operational metadata:

- `source_email_count`
- `source_signal_count`
- `generation_version`

It also stores a source window:

- `source_window_start`
- `source_window_end`

Those bounds are derived from the timestamps of emails and buyer signals actually used in the generation run.

## Failure behavior

Summary generation is downstream from CRM email sync and buyer-signal ingestion.

Important rules:

- summary refresh failure does not block email persistence
- summary refresh failure does not block buyer-signal persistence
- if a previous summary exists, failure marks the row `stale`
- if no usable summary exists yet, failure marks the row `error`

This is deliberate. Summaries are derived artifacts, not system-of-record data.

## API surface

Read endpoints:

- `GET /api/crm/contacts/{id}/summary`
- `GET /api/crm/deals/{id}/summary`

Behavior:

- return `200` with `null` when no summary row exists yet
- return the stored summary row otherwise

These endpoints are read-only in Phase 1b. There is no manual edit API.

## Frontend surface

The frontend consumes summaries through:

- `useContactSummary`
- `useDealSummary`

Main UI component:

- `frontend/src/components/crm/EntitySummaryCard.tsx`

Current placement:

- contact detail page: top of Overview tab
- deal detail page: between Stage Progress and Activity

Supported UI states:

- no summary yet
- refreshing
- stale with last good summary
- error with last error detail

The query layer polls every 15 seconds while a summary is:

- `pending_refresh`
- `stale`

## Relationship to other CRM intelligence phases

Phase order now looks like:

- Phase 1a: buyer-signal ingestion
- Phase 1b: entity summaries
- Phase 1c: automation taxonomy / trigger framework
- Phase 2: deal automation and review feed consuming the signal + summary layer

That means summaries are not the intelligence foundation by themselves. They depend on:

- CRM email sync
- buyer-signal ingestion

And later work should depend on them rather than rebuilding their own ad hoc context layer.

## Known current limitations

- only `contact` and `deal` summaries exist
- multi-contact emails without a linked deal do not trigger contact summaries
- summaries are read-only derived artifacts
- there is no admin diagnostics surface yet for summary runs
- there is no company-level or support-level summary model yet
- there is no summary-driven CRM write-back logic

## Implementation references

- `server/internal/service/crm_summary.go`
- `server/internal/repository/crm_summary.go`
- `server/internal/model/crm_summary.go`
- `server/internal/temporalapp/crm_summary_workflow.go`
- `server/internal/handler/crm_summary.go`
- `frontend/src/components/crm/EntitySummaryCard.tsx`
- `frontend/src/hooks/queries/useCRM.ts`
- `frontend/src/pages/crm/ContactDetail.tsx`
- `frontend/src/pages/crm/DealDetail.tsx`
