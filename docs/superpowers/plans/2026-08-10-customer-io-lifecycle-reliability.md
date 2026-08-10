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
- Modify `server/internal/dbmigrate/migrator_test.go` to assert the controlled migration contract.
- Modify `server/internal/repository/billing.go` for atomic billing transitions and enqueue.
- Modify `server/internal/service/customer_io.go` and tests for payloads, typed failures, and ULIDs.
- Create `server/internal/service/customer_io_outbox.go` and tests for delivery/retries.
- Modify `server/internal/service/billing.go` and tests to enqueue instead of sending synchronously.
- Modify `server/internal/handler/billing.go` and `server/internal/handler/billing_test.go` to propagate provider time.
- Modify `server/cmd/api/main.go` to run and stop the poller.
- Modify `docs/customer-io/workspace-lifecycle-data-contract.md` to match actual fields.

### Task 1: Outbox Model And Migration

**Files:**
- Create: `server/internal/model/customer_io_outbox.go`
- Create: `server/internal/dbmigrate/sql/202608100001_customer_io_lifecycle_outbox.sql`
- Create: `server/internal/repository/customer_io_outbox_test.go`
- Modify: `server/internal/dbmigrate/migrator_test.go`

- [ ] Write failing SQLite round-trip and unique-semantic-key tests. SQLite tests create their compatible table through GORM `AutoMigrate`; they do not execute PostgreSQL JSONB/index syntax.
- [ ] Write a failing migration-contract test in `server/internal/dbmigrate/migrator_test.go` that loads the controlled SQL and asserts JSONB defaults, status/attempt checks, unique semantic key, nullable `ON DELETE SET NULL` FK, and the partial due-work index.
- [ ] Run `cd server && go test ./internal/repository -run TestCustomerIOLifecycleOutbox -count=1`; verify compilation fails because the model is missing.
- [ ] Define `pending`, `processing`, `delivered`, and `failed` states; store attributes and recipient snapshots as `json.RawMessage`.
- [ ] Add idempotent DDL with JSONB defaults, status/attempt checks, unique `semantic_key`, nullable workspace FK with `ON DELETE SET NULL`, and a partial due-work index.
- [ ] Re-run the focused repository tests, `cd server && go test ./internal/dbmigrate -run CustomerIO -count=1`, and `cd server && go run ./cmd/migrate validate`; expect PASS.
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
- [ ] Add `PaidWorkspaceCount` to `CustomerIOOrganizationIdentity` and `customerIOOrganizationSummary`, increment it for paid active workspaces, and update `customerIOOrganizationAttributes` to emit `paid_workspace_count` and `monthly_due_cents`. Retain `estimated_monthly_due_cents` only as a documented temporary alias. Test the canonical names.
- [ ] Run `gofmt` and the focused tests; expect PASS.
- [ ] Commit with `git commit -m "fix(customerio): harden lifecycle event delivery"`.

### Task 4: Atomic Trial Initialization And Expiry

**Files:**
- Modify: `server/internal/repository/billing.go`
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/service/billing_test.go`

- [ ] Write a failing trial-initialization test showing repeated/concurrent-style calls create one trial and one `trial_started` row.
- [ ] Run `cd server && go test ./internal/service -run TestBillingServiceTrialInitializationOutbox -count=1`; verify RED.
- [ ] Add `CreateTrialWithLifecycleEvent(ctx, billing, event)` using insert-only `ON CONFLICT DO NOTHING RETURNING` or a dialect-safe locked equivalent. Only the creator transaction snapshots recipients and enqueues; a loser reloads existing billing.
- [ ] Re-run `TestBillingServiceTrialInitializationOutbox`; expect PASS.
- [ ] Write a failing expiry test showing two sweeps transition/enqueue once and forced enqueue failure rolls back the transition.
- [ ] Run `cd server && go test ./internal/service -run 'TestBillingServiceExpireOverdue.*Outbox' -count=1`; verify RED.
- [ ] Add `ExpireTrialsWithLifecycleEvents(ctx, now)` using conditional updates and returned rows inside the same transaction, then snapshot/enqueue only those rows.
- [ ] Remove both `syncCustomerIOWorkspace` and `trackCustomerIOWorkspaceEvent` from trial initialization/expiry paths so no Customer.io HTTP remains in those requests.
- [ ] Re-run the focused trial tests plus `go test ./internal/repository -run 'TestCustomerIOLifecycleOutbox|TestBilling' -count=1`; expect PASS.
- [ ] Commit with `git commit -m "fix(billing): atomically enqueue trial lifecycle events"`.

### Task 5: Atomic Stripe Billing Events

**Files:**
- Modify: `server/internal/model/billing.go`
- Modify: `server/internal/handler/billing.go`
- Modify: `server/internal/handler/billing_test.go`
- Modify: `server/internal/repository/billing.go`
- Modify: `server/internal/service/billing.go`
- Modify: `server/internal/service/billing_test.go`

- [ ] Add a failing handler test proving `stripe.Event.Created` reaches `OccurredAt` for invoice and trial-ending inputs; add a service test proving zero provider time falls back to receipt time.
- [ ] Run `cd server && go test ./internal/handler -run TestBillingStripeOccurredAt -count=1`; verify RED.
- [ ] Add `OccurredAt` to normalized Stripe inputs and populate it in `server/internal/handler/billing.go`; re-run the handler test and expect PASS.
- [ ] Add a repository transaction coordinator `InBillingLifecycleTransaction(ctx, outboxRepo, fn)` whose callback receives transaction-bound billing and outbox repositories sharing one `*gorm.DB`, without exposing GORM to the service layer.
- [ ] Add `LockStripeWebhookEvent(ctx, id, eventType)` on the transaction-bound billing repository: insert on conflict, then `SELECT ... FOR UPDATE`; return false when already processed.
- [ ] Write a failing payment-failed test asserting the locked webhook row, billing mutation, recipient snapshot, outbox insert, and processed marker commit together; cover enqueue rollback and duplicate/concurrent-style calls.
- [ ] Run `cd server && go test ./internal/service -run TestBillingServicePaymentFailedCustomerIOOutbox -count=1`; verify RED.
- [ ] Implement the minimal payment-failed transaction flow and remove both synchronous Customer.io sync/event calls; re-run and expect PASS.
- [ ] Repeat the same RED/implementation/GREEN cycle for `TestBillingServicePaymentSucceededCustomerIOOutbox`.
- [ ] Repeat the same RED/implementation/GREEN cycle for `TestBillingServiceTrialWillEndCustomerIOOutbox`.
- [ ] Run all three focused service tests and `go test ./internal/handler -run 'TestBilling.*Stripe' -count=1`; expect PASS.
- [ ] Commit with `git commit -m "fix(billing): durably enqueue Stripe lifecycle events"`.

### Task 6: Delivery Worker And API Lifecycle

**Files:**
- Create: `server/internal/service/customer_io_outbox.go`
- Create: `server/internal/service/customer_io_outbox_test.go`
- Modify: `server/cmd/api/main.go`

- [ ] Write a failing delivery test for snapshot/current-membership intersection, occurrence-time roles, no newly-added recipients, empty snapshots, and stable per-recipient IDs.
- [ ] Run `cd server && go test ./internal/service -run TestCustomerIOLifecycleOutboxWorkerDelivery -count=1`; verify RED.
- [ ] Implement `ProcessDueOnce` for successful delivery only; re-run and expect PASS.
- [ ] Write failing retry-classification tests for 400, transport/408/429/5xx, `Retry-After`, max attempts, and stale-token fencing.
- [ ] Run `cd server && go test ./internal/service -run TestCustomerIOLifecycleOutboxWorkerRetry -count=1`; verify RED.
- [ ] Implement five-minute leases, ten attempts, capped exponential backoff, bounded jitter, and fenced result updates; re-run and expect PASS.
- [ ] Write a failing disabled-credentials test; implement the guard so no rows are claimed or attempts burned; re-run and expect PASS.
- [ ] Add `Run(ctx, interval)` and wire production dependencies in `server/cmd/api/main.go`: construct the outbox repository, inject it into billing transaction coordination and worker delivery, then start the poller only after all dependencies exist.
- [ ] Use a cancellable worker context plus wait group; cancel and wait during graceful shutdown. Do not add another deployment or worktree.
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
