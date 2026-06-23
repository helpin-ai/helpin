# Helpin Pricing Strategy

## Current Model

Helpin uses flat workspace pricing with included monthly AI usage. There is no Free plan; unpaid workspaces are locked until they choose Starter or Growth.

| Plan | Price | Annual | Seats | Teams | CRM contacts | AI usage/month | Trial |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | --- |
| Starter | $99/month | $948/year | Unlimited | 10 | 5,000 | 5,000 units | No separate trial |
| Growth | $299/month | $2,868/year | Unlimited | Unlimited | Unlimited | 25,000 units | 14-day no-card trial on new workspaces |

There are no Business, Enterprise, or higher plans in v1.

## Trial Behavior

- New workspaces start on a 14-day Growth trial without requiring a card.
- Trial workspaces receive the Growth monthly AI usage allocation: 25,000 units.
- The trial has a hard 25,000-unit cap. After that cap or the 14-day trial expires, users must upgrade to continue using paid AI features.
- If the workspace does not upgrade before the trial ends, it becomes locked.
- Locked workspaces keep their data, but only billing/reactivation flows are available.

## Plan Changes

- Paid plan and interval changes apply immediately from the billing page plan selector with prorated billing.
- Cancellation is handled through Stripe and locks the workspace at the end of the current billing period.
- Users keep their current paid limits until a scheduled cancellation takes effect.
- Locked workspaces do not delete data, but product access is blocked except for billing and reactivation.

## AI Usage

AI usage is consumed by AI work, not by seats. It resets each billing period. Customer-facing surfaces should say "AI usage"; backend code may continue to store the allowance and ledger as credits.

Internal metering uses the formula and feature floors in [AI Usage Metering](./ai-usage-metering.md). The highest feature floor is 100 usage units; larger runs may consume more than the floor based on token usage.

Implementation status: direct LLM calls are preflighted before provider execution and metered through the shared metered provider context. Metered providers fail closed unless the call has usage context or an explicit setup/free exemption. Native SDK, Codex, and OpenCode agent runs are preflighted before they are queued; if the workspace is locked or AI usage is exhausted, no run row is created and the launch endpoint returns payment required. Completed runs are metered after their token usage is persisted. SLA limits are intentionally deferred until the SLA feature exists.

| Action | Floor usage units |
| --- | ---: |
| AI triage/routing | 2 |
| CRM signal detection | 3 |
| Support reply draft | 8 |
| Help article generation/update | 20 |
| Atlas epic planning run | 80 |
| Forge or Lens run | 100 |

Extra AI usage is available only on paid plans. Workspace admins can enable or disable it. When enabled, Helpin bills $50 per 5,000-unit usage pack after included usage is exhausted.

Usage policy:

- Upgrade: the higher included AI usage allowance is available immediately.
- Paid downgrade: the lower included AI usage allowance is available immediately. If current-period usage is already above the new allowance, remaining usage shows as zero until the next billing period.
- Trial expiry or cancellation: the workspace locks and extra AI usage is disabled.
- Failed payments follow Stripe Billing retry/dunning behavior. While Stripe reports `past_due`, Helpin can warn users. When Stripe moves the subscription to `unpaid`, Helpin locks the workspace and disables extra AI usage until payment is fixed.
- Unused included AI usage does not roll over.

## Plan Packaging

Every plan includes the core Helpin modules: Project Management, Support, Sales/CRM, and Docs. Starter includes Helpin's built-in AI agents. Growth unlocks user-defined automation: custom agents, automation flows, and scheduled agent runs.

Starter:

- Unlimited seats
- 10 teams per workspace
- 5,000 monthly AI usage units
- 500 documents
- 5,000 CRM contacts
- Built-in AI agents
- Tasks custom views
- Shared inbox and team inboxes
- Inbox custom views and saved replies
- Connect support email addresses
- Coverage gap detection
- Custom help center domain
- Internal docs
- GitHub integration
- Extra AI usage packs

Growth:

- Everything in Starter
- Unlimited teams
- 25,000 monthly AI usage units
- Unlimited CRM contacts
- Custom AI agents
- Automation flows
- Scheduled agents and cron
- AI conversation routing
- Multilingual help center and AI article translation
- Round robin assignment
- SLA policies
- Deal automation
- Remove Helpin branding
- Priority support

## Stripe Implementation

Use Stripe Billing with Checkout Sessions and the Customer Portal.

- Create Products/Prices in Stripe for Starter monthly, Starter annual, Growth monthly, Growth annual, and the 5,000-unit AI usage pack.
- Store Stripe Price IDs in server environment variables.
- Keep Stripe secret keys server-side only.
- Use Checkout Sessions for initial paid subscription checkout.
- Use the in-app billing page plan selector for upgrades and downgrades.
- Use the Customer Portal for payment method, billing address, invoices, and account-level subscription management.
- Process subscription webhooks idempotently.

Billing management access is owner-only in v1. Workspace admins and delegated billing owners must not be able to change plans, open billing management flows, toggle extra AI usage, or manage organization billing. A later release may add explicit ownership transfer or workspace-to-organization reassignment.

Billing correctness rules:

- Stripe webhook event IDs are recorded before processing and marked processed only after the billing mutation succeeds. A duplicate event with `processed=false` must be retried; a duplicate event with `processed=true` must be ignored.
- Subscription ID is the authoritative workspace resolver for subscription and invoice webhooks. Customer ID may be used only when it maps to exactly one workspace; customer-only events for an organization with multiple workspace subscriptions must not update an arbitrary workspace.
- Extra AI usage billing must be atomic with usage recording. When a usage event crosses into a new paid 5,000-unit pack, the Stripe charge runs while the workspace billing row is locked and is bounded by a short request timeout; if Stripe billing fails or times out, the usage ledger and usage counters must not commit. A durable charge/reconciliation queue can replace the in-transaction Stripe call later if higher throughput requires shorter row locks.
- Customer-facing "add payment method" flows should route to the Stripe-hosted Customer Portal. Helpin does not collect cards with an in-app SetupIntent flow in v1.

Required server environment variables:

```bash
STRIPE_PUBLISHABLE_KEY=
STRIPE_SECRET_KEY=
STRIPE_WEBHOOK_SECRET=
STRIPE_STARTER_MONTHLY_PRICE_ID=
STRIPE_STARTER_ANNUAL_PRICE_ID=
STRIPE_GROWTH_MONTHLY_PRICE_ID=
STRIPE_GROWTH_ANNUAL_PRICE_ID=
STRIPE_CREDIT_BLOCK_PRICE_ID=
BILLING_TEST_SCENARIOS_ENABLED=false
```

`BILLING_TEST_SCENARIOS_ENABLED=true` enables the dev-only Billing Settings scenario switcher API. Use it only against local/dev databases. The switcher can set the current workspace to:

- Starter baseline with billing-test data removed
- Growth trial with all 25,000 trial AI units used
- Growth past_due grace state
- Growth unpaid locked state
- Starter with all 5,000 AI units used
- Starter near overage with extra AI usage enabled
- Starter with 500 billing-test documents
- Starter with 5,001 billing-test CRM contacts

The seeded documents and contacts are marked as billing test data so the reset scenario can remove them without deleting normal workspace data.

Sandbox Price creation helper:

```bash
export STRIPE_SECRET_KEY
node scripts/create-stripe-billing-prices.mjs
```

The helper creates:

- Helpin Starter product with monthly and annual recurring Prices
- Helpin Growth product with monthly and annual recurring Prices
- Helpin extra AI usage pack product with a one-time $50 Price

It prints the five Price IDs needed by the server environment.

## Customer-Facing Copy

Primary positioning:

> Pay for output, not headcount.

Pricing page commitments:

- Every plan includes PM, Support, Sales/CRM, and Docs.
- Paid plans unlock higher limits and user-defined AI automation.
- Starter and Growth include unlimited seats.
- Starter includes up to 10 teams; Growth includes unlimited teams.
- Starter includes up to 5,000 CRM contacts; Growth includes unlimited CRM contacts.
- Growth trial lasts 14 days and requires no card.
- Expired trials and ended subscriptions lock the workspace until reactivated.
- Extra AI usage packs are optional and admin-controlled.
