# Trial Lifecycle Campaigns

These campaigns target owners and admins of the affected workspace. They should not target every person in the parent organization.

## Campaigns

| Campaign | Entry | Exit/suppression |
|---|---|---|
| Welcome | `trial_started` or workspace created | Workspace reaches first value or trial ends |
| Activation reminder | Workspace has no first-value milestone after 2 days | Any first-value event |
| Trial day 7 | Trial has 7 or fewer days remaining | Paid subscription, canceled workspace, or unsubscribe |
| Trial day 11 | Trial has 3 or fewer days remaining | Paid subscription or trial expired |
| Trial day 13 | Trial has 1 or fewer days remaining | Paid subscription or trial expired |
| Trial expired | `trial_expired` | Subscription started or reactivated |
| Win-back | Trial expired for 3, 7, and 14 days | Subscription started or unsubscribe |
| Payment recovery | `payment_failed` | Payment succeeded, canceled, or workspace revoked |

Audience rules:

- Recipient has an active relationship to the affected workspace.
- Relationship role is `owner` or `admin`.
- Workspace is still eligible for the campaign.
- Organization-level aggregates may personalize expansion messaging but must not replace workspace targeting.

Keep the first launch low-frequency: no more than one commercial email per workspace recipient every 48 hours, with lifecycle priority over general education.
