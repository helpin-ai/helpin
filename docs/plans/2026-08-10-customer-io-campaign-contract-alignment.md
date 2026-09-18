# Customer.io Campaign Contract Alignment Implementation Plan

> Historical implementation plan (2026-08-10), source-compared on 2026-09-17.
> Signup now calls `TrackUserSignedUp` (not the proposed `TrackPersonEvent`), and
> the outbox calls `RefreshWorkspaceForOutbox` before delivery. Billing source
> and tests moved to `server/ee`; the old `internal/service/billing_test.go` path
> and generic test commands below do not describe that edition's current layout.
> Checkboxes and expected RED/GREEN results are original planning text, not a
> fresh verification report. Campaign/segment IDs and draft states have not been
> checked remotely. Do not apply the old mutation checklist as current setup.
> `module_first_value` remains without a production emitter; see the current
> [campaign guides](../customer-io/README.md) and
> [identity service](../../server/internal/service/customer_io.go).

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans to implement this plan locally. Do not use subagents. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make Helpin's emitted Customer.io events, refreshed workspace state, draft campaign triggers, waits, and lifecycle segment conditions agree on one canonical contract.

**Architecture:** Keep billing events in the existing durable, fenced outbox. Add a narrow best-effort signup event at the identity boundary, and make the outbox worker perform an error-returning workspace identity refresh before fan-out. Update only the existing Helpin draft campaigns and lifecycle segments, previewing every API mutation before applying it.

**Tech Stack:** Go, GORM, Customer.io Track v2, Customer.io Fly API through `cio`, Go tests.

---

### Task 1: Signup event contract

**Files:**
- Modify: `server/internal/service/customer_io.go`
- Modify: `server/internal/service/auth.go`
- Test: `server/internal/service/customer_io_test.go`
- Test: `server/internal/service/auth_test.go`

- [ ] Write a failing test proving a newly created user emits `user_signed_up` after identification with a stable timestamp and safe signup attributes.
- [ ] Run `go test ./internal/service -run 'Test.*UserSignedUp' -count=1` and confirm the missing event failure.
- [ ] Add an error-returning `TrackPersonEvent` identity method and call it only in new-user signup paths; keep delivery best-effort so signup cannot fail.
- [ ] Run the focused test and existing Customer.io/auth tests.
- [ ] Commit with `fix(customerio): emit signup lifecycle event`.

### Task 2: Refresh workspace state before outbox fan-out

**Files:**
- Modify: `server/internal/service/customer_io.go`
- Modify: `server/internal/service/customer_io_outbox.go`
- Test: `server/internal/service/customer_io_outbox_test.go`

- [ ] Write a failing worker test proving workspace identity refresh happens before event delivery.
- [ ] Write a failing worker test proving a typed refresh failure schedules a fenced retry and does not mark the row delivered.
- [ ] Run `go test ./internal/service -run 'TestCustomerIOOutbox.*Workspace' -count=1` and confirm both fail for the expected missing behavior.
- [ ] Add an error-returning workspace refresh method that identifies the current workspace object and active relationships without swallowing Customer.io errors.
- [ ] Invoke it before recipient fan-out; treat a deleted workspace as a successful no-op.
- [ ] Run focused and full Customer.io service tests.
- [ ] Commit with `fix(customerio): refresh workspace state during delivery`.

### Task 3: Lock the canonical event contract in tests

**Files:**
- Modify: `server/internal/service/billing_test.go`
- Modify: `server/internal/service/customer_io_test.go`

- [ ] Add table-driven tests covering `trial_started`, `trial_will_end`, `trial_expired`, `payment_failed`, and `payment_succeeded` names and required workspace context.
- [ ] Run the new tests and confirm any missing context fails.
- [ ] Add only the minimal missing event properties.
- [ ] Run `go test ./internal/service -run 'Test(CustomerIO|BillingService)' -count=1`.
- [ ] Commit with `test(customerio): lock lifecycle event contract`.

### Task 4: Update Customer.io draft campaigns

**External resources:**
- Helpin environment: `214520`
- Campaigns: 3, 4, 5, 6

- [ ] Read back each campaign immediately before mutation and save the complete recipients/trigger field group required by Fly API replacement semantics.
- [ ] Preview campaign 4 entry change from `workspace_created` to `trial_started` with `cio ... --dry-run`.
- [ ] Preview campaign 4 conditional-wait changes: `subscription_started` to `payment_succeeded`, `subscription_canceled` to `payment_failed`, retain `trial_expired`, and keep `workspace_id` correlation while preserving the existing four-edge graph.
- [ ] Preview campaign 5 recovery wait change from `payment_recovered` to `payment_succeeded` with `workspace_id` correlation.
- [ ] Preview campaign 6 entry change from `subscription_started` to `payment_succeeded`.
- [ ] Apply the previewed mutations without changing campaign state.
- [ ] Read back campaigns 3–6, confirm all remain `draft`, and run each `/validate` endpoint.

### Task 5: Update Customer.io lifecycle segments

**External resources:**
- Segments: 16, 17, 18, 19, 20, 21

- [ ] Read each segment immediately before mutation.
- [ ] Preview segment 16 rename/condition change to `Lifecycle — Trial started` / `trial_started`.
- [ ] Preview segment 17 condition change to `trial_will_end`.
- [ ] Preview segment 19 rename/condition change to `Lifecycle — Payment succeeded` / `payment_succeeded`.
- [ ] Apply the previewed mutations; do not change segments 18, 20, or 21.
- [ ] Read back all six segments and confirm their exact conditions and finished states.

### Task 6: Final verification

**Files:**
- Modify if needed: `docs/specs/2026-08-10-customer-io-campaign-contract-alignment-design.md`

- [ ] Run `gofmt` on changed Go files.
- [ ] Run `go test ./...`, `go vet ./...`, and `go build ./...` from `server/`.
- [ ] Run `git diff --check` and confirm a clean worktree after commits.
- [ ] Read back Customer.io campaigns 3–6 and segments 16–21 with narrow `--jq` projections.
- [ ] Confirm campaigns are still draft, campaign validation is clean, and no unsupported event names remain in their triggers/waits or lifecycle segment conditions.
- [ ] Report commits, verification evidence, Customer.io changes, and any environment-limited checks.
