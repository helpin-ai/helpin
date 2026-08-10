# Customer.io Lifecycle Reliability Design

## Goal

Make Customer.io billing lifecycle delivery reliable across multiple API replicas without creating a general-purpose event platform.

## Scope

This change covers server-originated Customer.io billing lifecycle events:

- `trial_started`
- `trial_will_end`
- `trial_expired`
- `payment_failed`
- `payment_succeeded`

It also aligns the shared workspace event payload with the documented campaign contract. Browser-only module activation analytics remain unchanged.

## Architecture

Add a Customer.io-specific PostgreSQL outbox. Each row represents one workspace lifecycle event and contains:

- an internal UUID primary key;
- a unique semantic event key;
- workspace ID and event name;
- occurrence time and JSON attributes;
- status, attempt count, next-attempt time, claim time, last error, and timestamps.

The semantic event key prevents the application from enqueueing the same lifecycle transition twice. The worker derives a stable, valid Customer.io ULID for each recipient from the outbox row, recipient ID, and occurrence time. Retries therefore reuse the same Customer.io event ID.

The outbox is deliberately integration-specific. It does not introduce a generic event bus, NATS topic, workflow engine, or reusable analytics framework.

## Trial Expiry

Replace the separate “list overdue” and bulk update operations with one repository transaction:

1. Atomically update only currently eligible trial rows using `UPDATE ... RETURNING`.
2. Insert one outbox row per returned workspace using a unique key derived from the workspace and trial end.
3. Commit the state transition and outbox records together.

Only rows returned by that transaction count as expired. Concurrent API replicas may run the sweep, but a workspace can be transitioned and enqueued once.

## Other Billing Events

Stripe webhook handlers continue to use their existing webhook idempotency records. After persisting the billing transition, they enqueue a lifecycle event using a semantic key based on the Stripe event ID. Enqueueing is idempotent. Customer.io is no longer called synchronously from these handlers.

`trial_started` uses a workspace-and-trial-end semantic key because it can originate from Helpin rather than Stripe.

## Delivery Worker

The API starts a small poller dedicated to the Customer.io outbox:

1. Claim a bounded batch of due rows using PostgreSQL row locking with `SKIP LOCKED` semantics.
2. Load the workspace and its currently active members.
3. Deliver one person event per active member with workspace and role context.
4. Mark the row delivered only when all intended recipients succeed.
5. On failure, release it with capped exponential retry timing and a recorded error.

Claims have an expiry so a process crash does not strand work. Rows that exceed the retry limit remain failed for inspection rather than being deleted. Successful rows are retained for deduplication and audit.

The first implementation uses the existing Track v2 endpoint. It does not add batching until volume demonstrates a need.

## Event Contract

Every server-side workspace event includes:

- `workspace_id`
- `workspace_name`
- `workspace_slug`
- `organization_id` when present
- `workspace_role`
- `membership_status`
- relevant billing attributes

The organization object uses the documented names `paid_workspace_count` and `monthly_due_cents`. Existing aliases may be retained temporarily if current Customer.io campaigns depend on them. Workspace object documentation will only claim fields actually supplied by the backend; browser-only module traits are explicitly identified as browser enriched.

Event attributes pass through a small denylist for Customer.io-reserved delivery override keys (`recipient`, `from_address`, and `reply_to`) and obvious credential/content keys. This is boundary protection, not a schema framework.

## Failure Semantics

Customer.io outages never roll back billing state or fail Stripe webhook processing. They leave durable outbox work for retry. Database failures while atomically expiring trials and creating their outbox rows fail the entire transaction.

If a member is removed before delivery, the worker excludes that member because it resolves active membership at delivery time. A newly added member does not receive an old event: the outbox stores the active recipient IDs captured when the event is enqueued, and the worker intersects that snapshot with current active membership.

## Testing

Tests will prove:

- concurrent-style repeated expiry sweeps transition and enqueue once;
- expiry state and outbox insertion are atomic;
- duplicate semantic keys do not create duplicate rows;
- retry claims reuse stable per-recipient ULIDs;
- failed delivery becomes retryable and successful delivery becomes final;
- removed recipients are suppressed and newly added recipients are not backfilled;
- required workspace context is present and reserved/sensitive attributes are removed;
- existing billing, Customer.io, repository, and migration tests remain green.

## Non-goals

- Replacing browser analytics.
- Making every module milestone server-authoritative.
- Building a generic outbox or organization-wide event bus.
- Configuring Customer.io campaigns themselves.
- Retrofitting unrelated Customer.io identity synchronization in this pass.
