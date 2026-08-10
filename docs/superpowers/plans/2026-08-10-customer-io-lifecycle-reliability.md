# Customer.io Lifecycle Reliability Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver billing lifecycle events to Customer.io once from Helpin's perspective, with durable retries and complete workspace context.

**Architecture:** Add a Customer.io-specific PostgreSQL outbox created in the same transactions as billing transitions. A fenced, replica-safe poller delivers snapshotted recipients through Track v2 using stable per-recipient ULIDs. Browser analytics and unrelated identity synchronization remain unchanged.

**Tech Stack:** Go 1.24, GORM, PostgreSQL, SQLite tests, Customer.io Track v2, controlled SQL migrations.

**Design:** `docs/superpowers/specs/2026-08-10-customer-io-lifecycle-reliability-design.md`

---

## File Map

- Create `server/internal/model/customer_io_outbox.go` for persisted states, recipients, and rows.
- Create `server/internal/dbmigrate/sql/202608100001_customer_io_lifecycle_outbox.sql` for production schema/indexes.
- Create `server/internal/repository/customer_io_outbox.go` and tests for enqueue/claim/fencing.
- Modify `server/internal/repository/billing.go` for atomic billing transitions and enqueue.
- Modify `server/internal/service/customer_io.go` and tests for payloads, typed failures, and ULIDs.
- Create `server/internal/service/customer_io_outbox.go` and tests for delivery/retries.
- Modify `server/internal/service/billing.go` and tests to enqueue instead of sending synchronously.
- Modify Stripe webhook normalization under `server/internal/handler/` to propagate provider time.
- Modify `server/cmd/api/main.go` to run and stop the poller.
- Modify `docs/customer-io/workspace-lifecycle-data-contract.md` to match actual fields.

### Task 1: Outbox Model And Migration

**Files:**
- Create: `server/internal/model/customer_io_outbox.go`
- Create: `server/internal/dbmigrate/sql/202608100001_customer_io_lifecycle_outbox.sql`
- Create: `server/internal/repository/customer_io_outbox_test.go`

- [ ] Write failing SQLite round-trip and unique-semantic-key tests.
- [ ] Run `cd server && go test ./internal/repository -run TestCustomerIOLifecycleOutbox -count=1`; verify compilation fails because the model is missing.
- [ ] Define `pending`, `processing`, `delivered`, and `failed` states; store attributes and recipient snapshots as `json.RawMessage`.
- [ ] Add idempotent DDL with JSONB defaults, status/attempt checks, unique `semantic_key`, nullable workspace FK with `ON DELETE SET NULL`, and a partial due-work index.
- [ ] Re-run the focused tests and `cd server && go run ./cmd/migrate validate`; expect PASS.
- [ ] Commit with `git commit -m "feat(customerio): add lifecycle outbox schema"`.

### Task 2: Idempotent Enqueue And Fenced Claims

**Files:**
- Create: `server/internal/repository/customer_io_outbox.go`
- Modify: `server/internal/repository/customer_io_outbox_test.go`

- [ ] Write failing tests for duplicate enqueue, exclusive live claims, expired-lease reclaim, attempt increments, stale-token no-ops, current-token completion/retry/failure, and 2 KiB error truncation.
- [ ] Run `cd server && go test ./internal/repository -run TestCustomerIOLifecycleOutbox -count=1`; verify behavioral failures.
- [ ] Implement narrow `Enqueue`, `ClaimDue`, `MarkDelivered`, `ScheduleRetry`, and `MarkFailed` operations. PostgreSQL uses `FOR UPDATE SKIP LOCKED`; all result updates require row ID, processing status, and claim token.
- [ ] Re-run the focused repository tests; expect PASS.
- [ ] Commit with `git commit -m "feat(customerio): add fenced outbox claims"`.

### Task 3: Track Client Contract And Stable IDs

**Files:**
- Modify: `server/internal/service/customer_io.go`
- Modify: `server/internal/service/customer_io_test.go`
- Modify: `server/go.mod`
- Modify: `server/go.sum`

- [ ] Write failing tests proving workspace events include name/ID/slug/organization/role/status; reserved override keys are stripped; HTTP failures expose status and `Retry-After`; stable IDs parse as ULIDs, embed occurrence milliseconds, survive retries, and differ by recipient.
- [ ] Run `cd server && go test ./internal/service -run 'TestCustomerIO.*(WorkspaceEvent|DeliveryError|ULID)' -count=1`; verify RED.
- [ ] Add typed `CustomerIODeliveryError`, safe response-body handling, and `Retry-After` parsing.
- [ ] Derive canonical ULIDs from occurrence milliseconds and the first 80 SHA-256 bits of the domain-separated outbox/recipient tuple, using a standards-correct ULID package.
- [ ] Add `workspace_name` and strip only `recipient`, `from_address`, and `reply_to` at the final boundary.
- [ ] Run `gofmt` and the focused tests; expect PASS.
- [ ] Commit with `git commit -m "fix(customerio): harden lifecycle event delivery"`.

### Task 4: Atomic Trial Initialization And Expiry

**Files:**
- Modify: `server/internal/repository/billing.go`
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/service/billing_test.go`

- [ ] Write failing tests showing repeated/concurrent-style trial initialization creates one trial and one `trial_started` row; repeated expiry transitions/enqueues once; forced enqueue failure rolls billing back.
- [ ] Run `cd server && go test ./internal/service -run 'TestBillingService.*(TrialInitialization|ExpireOverdue.*Outbox|OutboxRollback)' -count=1`; verify RED.
- [ ] Implement insert-only trial initialization with `ON CONFLICT DO NOTHING RETURNING` or a dialect-safe locked equivalent. Only the creator transaction snapshots recipients and enqueues.
- [ ] Replace list-then-update expiry with one transaction that conditionally transitions eligible rows, snapshots recipients, and enqueues only returned rows.
- [ ] Remove synchronous trial event calls and run the focused service/repository tests; expect PASS.
- [ ] Commit with `git commit -m "fix(billing): atomically enqueue trial lifecycle events"`.

### Task 5: Atomic Stripe Billing Events

**Files:**
- Modify: `server/internal/model/billing.go`
- Modify: the Stripe normalization file located under `server/internal/handler/`
- Modify: `server/internal/repository/billing.go`
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/service/billing_test.go`

- [ ] Write failing payment-failed, payment-succeeded, and trial-will-end tests asserting billing mutation, snapshot, outbox insert, and webhook processed marker commit together.
- [ ] Add failure-injection tests proving enqueue failure rolls the complete transaction back, duplicate webhook processing does not enqueue twice, and provider occurrence time reaches the outbox.
- [ ] Run `cd server && go test ./internal/service -run 'TestBillingService.*CustomerIOOutbox' -count=1`; verify RED.
- [ ] Add `OccurredAt` to normalized Stripe inputs and populate it from the provider event creation time.
- [ ] Implement repository transaction methods that mutate billing, snapshot active recipients, insert with `ON CONFLICT DO NOTHING`, and mark the webhook processed atomically.
- [ ] Remove synchronous billing lifecycle delivery and run focused service plus Stripe handler tests; expect PASS.
- [ ] Commit with `git commit -m "fix(billing): durably enqueue Stripe lifecycle events"`.

### Task 6: Delivery Worker And API Lifecycle

**Files:**
- Create: `server/internal/service/customer_io_outbox.go`
- Create: `server/internal/service/customer_io_outbox_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] Write failing worker tests covering successful delivery, stable retry IDs, snapshot/current-membership intersection, no newly-added recipients, occurrence-time roles, empty snapshots, 400 terminal behavior, transport/408/429/5xx retry, `Retry-After`, max attempts, disabled credentials, and stale-token fencing.
- [ ] Run `cd server && go test ./internal/service -run TestCustomerIOLifecycleOutboxWorker -count=1`; verify RED.
- [ ] Implement deterministic `ProcessDueOnce` plus `Run(ctx, interval)` with bounded batches, five-minute leases, ten attempts, capped exponential backoff, and bounded jitter.
- [ ] Do not claim while credentials are disabled. Mark successful empty snapshots delivered.
- [ ] Wire construction, cancellation, and shutdown in `server/cmd/api/main.go` without adding another deployment or worktree.
- [ ] Run focused tests plus `go test ./cmd/api ./internal/service ./internal/repository`; expect PASS.
- [ ] Commit with `git commit -m "feat(customerio): deliver lifecycle outbox events"`.

### Task 7: Contract And Full Verification

**Files:**
- Modify: `docs/customer-io/workspace-lifecycle-data-contract.md`
- Modify only directly related files required by verification failures.

- [ ] Document `workspace_name`, align `paid_workspace_count` and `monthly_due_cents`, distinguish browser-enriched modules, and describe durable billing events.
- [ ] Run `gofmt` on every changed Go file.
- [ ] Run `cd server && go vet ./...`; expect exit 0.
- [ ] Run `cd server && go build ./...`; expect exit 0.
- [ ] Run `cd server && go test ./...`; expect zero failures.
- [ ] Run `cd server && go run ./cmd/migrate validate`; expect exit 0.
- [ ] Run `git diff --check` and inspect `git status --short`; expect only intentional changes.
- [ ] Commit documentation with `git commit -m "docs: align Customer.io lifecycle contract"`.
