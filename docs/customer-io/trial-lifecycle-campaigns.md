# Trial lifecycle campaigns

This is a proposed campaign schedule for lifecycle operators. It is not a live
campaign inventory. Billing lifecycle events originate in the Enterprise billing
implementation; Community installations do not imply a paid trial lifecycle.
Configure and verify audience rules, delays, and frequency caps in Customer.io.

These campaigns target owners and admins of the affected workspace. They should not target every person in the parent organization.

## Campaigns

| Campaign | Entry | Exit/suppression |
|---|---|---|
| Welcome | `trial_started` | Workspace reaches first value or trial ends |
| Activation reminder | Workspace has no first-value milestone after 2 days | Any first-value event |
| Trial day 7 | Trial has 7 or fewer days remaining | Paid subscription, canceled workspace, or unsubscribe |
| Trial day 11 | Trial has 3 or fewer days remaining | Paid subscription or trial expired |
| Trial day 13 | Trial has 1 or fewer days remaining | Paid subscription or trial expired |
| Trial expired | `trial_expired` | `payment_succeeded` or current paid workspace state |
| Win-back | Trial expired for 3, 7, and 14 days | `payment_succeeded`, current paid workspace state, or unsubscribe |
| Payment recovery | `payment_failed` | Payment succeeded, canceled, or workspace revoked |

Audience rules:

- Recipient has an active relationship to the affected workspace.
- Relationship role is `owner` or `admin`.
- Workspace is still eligible for the campaign.
- Organization-level aggregates may personalize expansion messaging but must not replace workspace targeting.

Keep the first launch low-frequency: no more than one commercial email per workspace recipient every 48 hours, with lifecycle priority over general education.

## Match the emitted contract

The current billing implementation emits `trial_started`, `trial_will_end`,
`trial_expired`, `payment_failed`, and `payment_succeeded` through lifecycle
paths. See the [billing service](../../server/ee/service/billing.go) and
[trial expiry repository](../../server/ee/repository/billing.go). Signup emits
`user_signed_up` separately; it is not workspace trial creation.

Do not configure event triggers for `workspace_created`, `subscription_started`,
`payment_recovered`, or `upgrade_completed` on the assumption that these are
billing lifecycle aliases. Correlate recovery and suppression to the same
`workspace_id`, and use current workspace billing state as well as events.
The day labels above assume a 14-day trial; use `trial_ends_at` for actual timing.

First-value suppression and the activation reminder remain dependent on missing
milestone instrumentation described in [module activation campaigns](module-activation-campaigns.md).
The [outbox worker](../../server/internal/service/customer_io_outbox.go) refreshes
workspace state and filters removed recipients before delivery, but does not
implement the campaign schedules or the 48-hour commercial-email cap.
