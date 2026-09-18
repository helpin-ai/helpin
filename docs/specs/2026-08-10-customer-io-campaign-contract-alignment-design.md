# Customer.io Campaign Contract Alignment

> Historical design (2026-08-10). Campaign IDs, segment IDs, and draft-state
> assertions below are not a current Customer.io inventory. The signup and
> outbox refresh paths exist, but the proposed browser `module_first_value`
> emission is absent from current production call sites. Use the current
> [campaign guides](../customer-io/README.md) for implementation limits; verify
> hosted state separately before making configuration changes.

## Goal

Align Helpin's Customer.io draft campaigns and lifecycle segments with the events and attributes the application actually emits. Preserve the existing durable billing outbox, keep every campaign in draft, and avoid dual-emitting legacy aliases.

## Canonical contract

Helpin uses these lifecycle event names:

- `user_signed_up`: emitted after a new password or Google account is persisted and identified.
- `trial_started`: emitted by the transactional workspace billing initialization path.
- `trial_will_end`: emitted from Stripe's `customer.subscription.trial_will_end` event.
- `trial_expired`: emitted by the transactional trial-expiry path.
- `payment_failed`: emitted from Stripe's `invoice.payment_failed` event.
- `payment_succeeded`: emitted from Stripe's `invoice.payment_succeeded` event and used as the recovery/upgrade signal.
- `module_first_value`: emitted by the browser after a supported first-value action.

The integration will not emit compatibility aliases such as `workspace_created`, `trial_expiring`, `subscription_started`, `payment_recovered`, or `upgrade_completed`. One name per fact prevents duplicate campaign entry and reduces contract drift.

## Current source comparison

The implemented signup method is `TrackUserSignedUp`, called after new password
or Google signup. `RefreshWorkspaceForOutbox` returns refresh failures to the
worker before recipient delivery. Billing producers now live in
[Enterprise billing](../../server/ee/service/billing.go); signup, payload, stable-ID,
retry classification, repository fencing, and edition billing tests exist, but the
historical test and hosted-validation checklist below is not a current passing
report. No remote campaign or segment state was verified during this docs audit.

## Application changes

### Signup event

After a newly created user has been persisted and identified in Customer.io, the authentication service emits `user_signed_up` for that person. Existing-user sign-in paths do not emit it. The event contains stable person and signup context already available to the service and must not contain credentials or tokens.

Signup tracking remains best-effort like the existing identity synchronization. Account creation must not fail because Customer.io is disabled or unavailable.

### Workspace state refresh during outbox delivery

Before delivering a claimed workspace billing event, the outbox worker refreshes the Customer.io workspace object and active relationships using current database state. The refresh must return delivery errors to the worker instead of swallowing them, so a transient Customer.io failure schedules the same fenced retry as event delivery.

The refresh remains idempotent. The event's recipient snapshot and occurrence-time `workspace_role` still govern event fan-out; newly added members do not receive old events, and removed members are suppressed. Current object attributes (`billing_status`, plan, trial fields) are refreshed separately so relationship-based segments remain accurate.

If the workspace was deleted, delivery succeeds as a no-op after filtering recipients because the outbox row remains for audit and deduplication.

## Customer.io configuration changes

All campaign mutations remain in draft.

### Campaigns

- Account welcome (3): keep `user_signed_up`.
- Workspace trial lifecycle (4): entry event becomes `trial_started`.
- Organization billing recovery (5): keep `payment_failed`; its recovery wait becomes `payment_succeeded`, correlated by `workspace_id` rather than organization-wide state.
- Upgrade confirmation (6): entry event becomes `payment_succeeded`.

The trial lifecycle conditional waits replace `subscription_started` with `payment_succeeded`, replace unsupported `subscription_canceled` with `payment_failed`, and retain `trial_expired`. This preserves the existing four-edge conditional-wait graph: successful payment, failed payment, or expiry exits onboarding, while timeout continues to the next message. All comparisons remain correlated by event `workspace_id`.

### Segments

- Lifecycle — Workspace created (16): rename to Lifecycle — Trial started and use `trial_started`.
- Lifecycle — Trial expiring (17): use `trial_will_end`.
- Lifecycle — Module first value (18): keep `module_first_value`.
- Lifecycle — Upgrade completed (19): rename to Lifecycle — Payment succeeded and use `payment_succeeded`.
- Suppression — Internal and test profiles (20): remain Customer.io-managed through `test_record`; Helpin does not overwrite manually applied suppression.
- Workspace — Trial lifecycle eligible (21): retain the relationship conditions on workspace object type 1, `billing_status=trialing`, `membership_status=active`, and owner/admin roles.

## Error handling and rollout

Every Customer.io mutation is previewed with `--dry-run`, then applied, then read back. Campaign validation must report no warnings. Campaign state must remain `draft`. If a mutation fails, stop rather than applying dependent changes with a partially understood contract.

Application changes land before the configuration is considered deployable. Since campaigns stay draft, a short interval where Customer.io expects the new names before application deployment cannot send messages.

## Testing

- Auth tests prove only newly created users emit `user_signed_up` and Customer.io failure does not fail signup.
- Worker tests prove workspace refresh occurs before event delivery and a refresh failure retries the fenced row.
- Existing recipient snapshot, ULID, retry classification, and atomic billing tests remain green.
- A contract-oriented test enumerates the canonical server event names used by billing lifecycle paths.
- Customer.io campaign validation and read-back confirm triggers, waits, segment conditions, and draft states.

## Non-goals

- Activating campaigns.
- Reworking message copy or visual design.
- Adding a generic event schema framework.
- Reclassifying internal/test users in application code.
- Replacing browser-side `module_first_value` tracking.
