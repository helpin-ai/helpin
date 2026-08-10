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
- a JSON recipient snapshot containing user ID and workspace role at occurrence time;
- status, attempt count, next-attempt time, claim time, last error, and timestamps.

The semantic event key prevents the application from enqueueing the same lifecycle transition twice. The worker derives a stable, valid Customer.io ULID for each recipient from the outbox row, recipient ID, and occurrence time. Retries therefore reuse the same Customer.io event ID.

The outbox is deliberately integration-specific. It does not introduce a generic event bus, NATS topic, workflow engine, or reusable analytics framework.

## Trial Expiry

Replace the separate “list overdue” and bulk update operations with one repository transaction:

1. Atomically update only currently eligible trial rows using `UPDATE ... RETURNING`.
2. Insert one outbox row per returned workspace using a unique key derived from the workspace and trial end.
3. Commit the state transition and outbox records together.

Only rows returned by that transaction count as expired. Concurrent API replicas may run the sweep, but a workspace can be transitioned and enqueued once.

## Atomic Billing Transitions

Every covered billing transition uses one database transaction for the billing mutation, active-recipient snapshot, outbox insertion, and—when applicable—marking the Stripe webhook processed. Customer.io HTTP remains outside the transaction and request path.

Stripe webhook handlers continue to use their existing webhook idempotency records. They enqueue using a semantic key based on the Stripe event ID. A database error rolls back the billing mutation, outbox record, and processed marker together, allowing Stripe replay to retry the complete operation.

`trial_started` uses an insert-only initialization transaction: `INSERT ... ON CONFLICT (workspace_id) DO NOTHING RETURNING ...` (or an equivalent locked recheck). Only the transaction that returns the newly persisted billing row may snapshot recipients and enqueue the event. Its semantic key uses the workspace ID and the persisted returned `trial_ends_at`, because the event can originate from Helpin rather than Stripe. A transaction that loses the insert race reloads and returns the existing billing state without enqueueing.

Normalized Stripe webhook inputs gain the provider event creation time. That value is the lifecycle occurrence time and the timestamp embedded in Customer.io event IDs. Receipt time is used only when the provider timestamp is unavailable.

## Delivery Worker

The API starts a small poller dedicated to the Customer.io outbox:

1. Claim a bounded batch of due rows using PostgreSQL row locking with `SKIP LOCKED` semantics.
2. Set `status=processing`, increment attempts, assign a new claim token, and set a lease expiry in the same claim transaction.
3. Intersect the snapshotted recipient IDs with currently active workspace membership.
4. Deliver one person event per remaining recipient, using the role captured at occurrence time.
5. Mark the row delivered only when all intended recipients succeed.
6. On failure, release it with capped exponential retry timing and a recorded error.

Claims have a five-minute lease so a process crash does not strand work. Due work is either pending with `next_attempt_at <= now` or processing with an expired lease. Every delivered/retry/failed update includes the row ID, `status=processing`, and claim token; zero affected rows means the worker lost ownership and must not update the row. Empty snapshots and snapshots whose members are all inactive complete as successful no-ops. Newly added members do not receive historical events.

Transient transport failures, HTTP 408, HTTP 429, and HTTP 5xx retry with capped exponential backoff, bounded jitter, and a maximum of ten attempts. HTTP 429 honors `Retry-After` when it is later than the calculated delay. Other HTTP 4xx responses are terminal. Stored error text is capped at 2 KiB. Disabled Customer.io credentials leave rows pending without claiming or incrementing attempts.

The Customer.io client returns a typed delivery error containing HTTP status and `Retry-After` information so the worker can classify failures.

The first implementation uses the existing Track v2 endpoint. It does not add batching until volume demonstrates a need.

For each recipient, the worker derives a stable Track v2 event ULID using the occurrence time in milliseconds for the 48-bit timestamp and the first 80 bits of SHA-256 over the domain-separated tuple `helpin-customerio-event-v1`, outbox UUID, and recipient user ID as entropy. The result is canonical uppercase Crockford Base32. The same outbox row and recipient therefore reuse the same valid ULID across retries while different recipients receive different IDs.

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

Each lifecycle event is constructed from explicit internal fields. At the final boundary, Customer.io-reserved delivery override keys (`recipient`, `from_address`, and `reply_to`) are removed. No generic attribute-schema or fuzzy sensitive-key framework is introduced.

## Failure Semantics

Customer.io outages never roll back billing state or fail Stripe webhook processing. They leave durable outbox work for retry. Database failures while atomically expiring trials and creating their outbox rows fail the entire transaction.

If a member is removed before delivery, the worker excludes that member because it intersects the occurrence-time snapshot with current active membership. A newly added member does not receive an old event. Role context remains the occurrence-time role so delayed delivery describes the state that produced the event.

## Schema And Migration

Add an idempotent controlled migration at `server/internal/dbmigrate/sql/202608100001_customer_io_lifecycle_outbox.sql`. The table uses JSONB defaults for attributes and recipients, a unique constraint on `semantic_key`, checks for valid status and non-negative attempts, and a nullable workspace foreign key with `ON DELETE SET NULL` so audit/deduplication survives workspace deletion. A partial index over due pending/processing rows supports polling by `status`, `next_attempt_at`, and `lease_expires_at`.

The repository exposes narrow operations for atomic billing transitions/enqueue, claiming, fenced completion, fenced retry, and fenced terminal failure. It does not expose a generic outbox abstraction.

## Testing

Tests will prove:

- concurrent-style repeated expiry sweeps transition and enqueue once;
- concurrent-style repeated trial initialization creates billing and enqueues `trial_started` once;
- expiry state and outbox insertion are atomic;
- duplicate semantic keys do not create duplicate rows;
- retry claims reuse stable per-recipient ULIDs;
- generated IDs parse as ULIDs, embed the occurrence timestamp, and differ by recipient;
- failed delivery becomes retryable and successful delivery becomes final;
- HTTP 400 becomes terminal; transport errors, 408, 429, and 5xx retry; 429 honors `Retry-After`;
- disabled credentials do not claim or burn attempts;
- stale claim tokens cannot overwrite a newer claim;
- removed recipients are suppressed and newly added recipients are not backfilled;
- occurrence-time roles survive later role changes and empty recipient sets complete successfully;
- required workspace context is present and reserved delivery-override attributes are removed;
- existing billing, Customer.io, repository, and migration tests remain green.

## Non-goals

- Replacing browser analytics.
- Making every module milestone server-authoritative.
- Building a generic outbox or organization-wide event bus.
- Configuring Customer.io campaigns themselves.
- Retrofitting unrelated Customer.io identity synchronization in this pass.
