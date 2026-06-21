# Helpin Pricing Strategy

## Current Model

Helpin uses flat workspace pricing with included AI credits. Free is intentionally constrained for small teams; paid plans remove seat limits.

| Plan | Price | Annual | Seats | AI credits/month | Trial |
| --- | ---: | ---: | ---: | ---: | --- |
| Free | $0 | $0 | 2 | 1,000 | New workspaces fall back here after trial |
| Starter | $99/month | $948/year | Unlimited | 5,000 | No separate trial |
| Growth | $299/month | $2,868/year | Unlimited | 25,000 | 14-day no-card trial on new workspaces |

There are no Business, Enterprise, or higher plans in v1.

## Trial Behavior

- New workspaces start on a 14-day Growth trial without requiring a card.
- Trial workspaces receive the Growth monthly credit allocation: 25,000 credits.
- If the workspace does not upgrade before the trial ends, it automatically moves to Free.
- Free includes 2 seats and 1,000 credits/month, and keeps all modules available with constrained AI usage.

## Plan Changes

- Paid plan and interval changes apply immediately from the billing page plan selector with prorated billing.
- Cancellation moves the workspace to Free at the end of the current billing period.
- Moving to Free is scheduled at renewal.
- Users keep their current paid limits until a scheduled cancellation to Free takes effect.
- If a workspace becomes Free while over the 2-seat limit, no data is deleted. Existing data remains readable, but new invitations and AI usage are blocked until the workspace removes seats or upgrades again.

## AI Credits

Credits are consumed by AI work, not by seats. They reset each billing period.

| Action | Credits |
| --- | ---: |
| Support AI reply | 5 |
| CRM/deal action | 10 |
| Document generation | 20 |
| Planning run | 50 |
| Coding/review run | 100 |

On-demand credits are available only on paid plans. Workspace admins can enable or disable them. When enabled, Helpin bills $50 per 5,000-credit block after included credits are exhausted.

Credit policy:

- Upgrade: the higher included credit allowance is available immediately.
- Paid downgrade: the lower included credit allowance is available immediately. If current-period usage is already above the new allowance, remaining credits show as zero until the next billing period.
- Trial expiry or cancellation to Free: credits reset to Free and on-demand credits are disabled.
- Unused included credits do not roll over.

## Plan Packaging

Every plan includes the core Helpin modules: Project Management, Support, Sales/CRM, and Docs. Paid plans unlock higher limits and advanced AI automation.

Free:

- 2 seats
- 1,000 AI credits/month
- Tasks and epics
- 200 documents
- Live chat widget
- Shared inbox
- CRM contacts and deals
- Public help center
- Limited built-in AI agents
- Helpin branding remains

Starter:

- Unlimited seats
- 5,000 AI credits/month
- 1,000 documents
- Basic automations and built-in agents
- Tasks and inbox custom views
- Team inboxes
- Inbox saved replies
- Email forwarding
- Sender addresses
- GitHub integration
- Custom help center domain
- On-demand credit blocks

Growth:

- Everything in Starter
- 25,000 AI credits/month
- Unlimited deals in CRM
- Multilingual help center
- AI article translation
- Custom AI agents
- Advanced automations
- Agent scheduling and cron
- Round robin assignment
- SLA policies
- AI conversation routing
- Coverage gap detection
- Deal automation
- Remove Helpin branding
- Priority support

## Stripe Implementation

Use Stripe Billing with Checkout Sessions and the Customer Portal.

- Create Products/Prices in Stripe for Starter monthly, Starter annual, Growth monthly, Growth annual, and the 5,000-credit block.
- Store Stripe Price IDs in server environment variables.
- Keep Stripe secret keys server-side only.
- Use Checkout Sessions for initial paid subscription checkout.
- Use the in-app billing page plan selector for upgrades, downgrades, and cancellation scheduling.
- Use the Customer Portal for payment method, billing address, invoices, and account-level subscription management.
- Process subscription webhooks idempotently.

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
```

Sandbox Price creation helper:

```bash
export STRIPE_SECRET_KEY
node scripts/create-stripe-billing-prices.mjs
```

The helper creates:

- Helpin Starter product with monthly and annual recurring Prices
- Helpin Growth product with monthly and annual recurring Prices
- Helpin AI credit block product with a one-time $50 Price

It prints the five Price IDs needed by the server environment.

## Customer-Facing Copy

Primary positioning:

> Pay for output, not headcount.

Pricing page commitments:

- Every plan includes PM, Support, Sales/CRM, and Docs.
- Paid plans unlock higher limits and advanced AI automation.
- Free includes 2 seats; Starter and Growth include unlimited seats.
- Growth trial lasts 14 days and requires no card.
- On-demand credit blocks are optional and admin-controlled.
