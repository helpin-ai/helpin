# Helpin Pricing Strategy

## Current Model

Helpin uses flat workspace pricing with included AI credits. Seats are unlimited on every plan.

| Plan | Price | Annual | AI credits/month | Trial |
| --- | ---: | ---: | ---: | --- |
| Free | $0 | $0 | 1,000 | New workspaces fall back here after trial |
| Starter | $99/month | $948/year | 5,000 | No separate trial |
| Growth | $299/month | $2,868/year | 25,000 | 14-day no-card trial on new workspaces |

There are no Business, Enterprise, or higher plans in v1.

## Trial Behavior

- New workspaces start on a 14-day Growth trial without requiring a card.
- Trial workspaces receive the Growth monthly credit allocation: 25,000 credits.
- If the workspace does not upgrade before the trial ends, it automatically moves to Free.
- Free includes 1,000 credits/month and keeps all modules available with constrained AI usage.

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

## Stripe Implementation

Use Stripe Billing with Checkout Sessions and the Customer Portal.

- Create Products/Prices in Stripe for Starter monthly, Starter annual, Growth monthly, Growth annual, and the 5,000-credit block.
- Store Stripe Price IDs in server environment variables.
- Keep Stripe secret keys server-side only.
- Use Checkout Sessions for upgrades and plan changes.
- Use the Customer Portal for payment method, invoice, cancellation, and subscription management.
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

- Every plan includes PM, Support, Sales/CRM, Docs, and AI agents.
- Every plan includes unlimited seats.
- Growth trial lasts 14 days and requires no card.
- On-demand credit blocks are optional and admin-controlled.
