# Tiered AI Usage Billing Design

## Goal

Replace AI credits, normalized usage units, fixed credit packs, and feature floors with exact
token-based billing while keeping the customer experience simple. Customers see the percentage
of their included AI allowance used and remaining. Helpin records actual provider tokens and
calculates every charge in integer micro-USD. Dollar amounts appear to customers only when they
represent an actual extra-usage charge or invoice.

The complete replacement ships on one branch and activates immediately when deployed. The rate
card in this specification is authoritative for this launch.

## Customer Experience

### Percentage-first allowance

The primary billing meter does not show credits, normalized units, or the dollar value of the
included allowance. It shows:

```text
AI usage
72% remaining
Resets September 12

28% used
```

The percentage is backed by exact monetary accounting:

```text
used percentage = min(posted charged micro-USD, allowance micro-USD) / allowance micro-USD
reserved percentage = active reserved micro-USD / allowance micro-USD
remaining percentage = max(allowance - posted - active reservations, 0) / allowance
```

The UI rounds display percentages without changing stored or billable amounts. It preserves
enough precision to show small activity: individual contributions below 0.01% display as
`<0.01%`.

Founder workspaces have no denominator. Their summary shows `Unlimited`, the reset/reporting
date, action counts, and actual token telemetry without an allowance percentage.

### Consumption breakdown

The default breakdown aggregates the current period by agent or feature so the page remains
useful when individual calls consume tiny fractions:

```text
Forge       12.4%   8 runs
Support AI   8.1%   214 replies
Atlas        3.7%   5 planning runs
Mira         2.3%   3 runs
Tool usage   0.4%
```

Opening a row shows:

- percentage of the current included allowance;
- action count and daily trend;
- Small, Medium, Large, and Flagship distribution;
- actual input, cache-read, cache-write, output, and reasoning tokens;
- hosted and customer-funded orchestration activity;
- provider-native paid-tool activity;
- actual and estimated measurement counts;
- promotional usage marked `Not counted toward your allowance`;
- individual sanitized run/call references and timestamps.

Percentages represent allowance consumption, never token quantities. Token counts always remain
actual provider tokens. Promotional setup usage and absorbed embeddings do not affect the meter.

When included usage reaches 100%, the page shows the exact accrued amount that Stripe may charge:

```text
Included AI usage   100% used
Extra AI usage      $3.37 before tax
```

Dollar amounts remain visible in extra-usage confirmations, accrued extra usage, settlement
history, and invoices because they are real financial obligations.

### Extra usage control

Rename `on-demand` to `Allow extra AI usage` and preserve each workspace's existing setting at
cutover.

- Disabled: Helpin reserves allowance before launch and rejects work that cannot fit.
- Enabled: work may exceed the included allowance and accrues an exact extra-usage charge.
- Founder: unlimited hosted usage; no extra-usage control or invoice.
- Trial: no extra usage unless the workspace has an active paid Stripe subscription.

There are no credit packs, block counters, or minimum extra-usage purchases.

## Commercial Rules

Subscription prices do not change. Internal allowance amounts are:

| Plan | Subscription billing | AI allowance period | Allowance |
| --- | --- | --- | ---: |
| Starter monthly | $99 monthly | Monthly | 99,000,000 micro-USD |
| Starter annual | $948 annually | Monthly | 79,000,000 micro-USD |
| Growth monthly | $299 monthly | Monthly | 299,000,000 micro-USD |
| Growth annual | $2,868 annually | Monthly | 239,000,000 micro-USD |
| Growth trial | 14 days | Trial lifetime | 140,000,000 micro-USD |
| Founder | Managed | Monthly reporting | Unlimited |

The allowance is an accounting denominator and is not presented as a cash balance. It does not
roll over, cannot be refunded, and is before applicable tax.

- Upgrades immediately increase the open period's allowance without resetting posted usage.
- Downgrades and billing-interval changes affect the next AI period.
- Cancellation does not refund or carry forward unused allowance.
- Founder activity is fully metered for reporting but never blocked or overage-billed.

Customer prices are the provider-cost ceiling plus 10%. A fully consumed hosted $99 allowance
costs at most $90 at the catalog ceilings, leaving $9 before non-provider costs. Actual cost can
be lower when the approved route costs less than its ceiling or the allowance is not fully used.

## Canonical Pricing Catalog

The backend owns one immutable, versioned catalog. Frontend and website data are generated from
that catalog and checked for drift in CI. A catalog version contains:

- effective date and immutable pricing version;
- tier names and customer descriptions;
- internal ceilings and published rates for each token class;
- provider, canonical model ID, accepted aliases, and exact allowed route;
- OpenRouter host constraints or maximum-price constraints;
- service, long-context, regional, batch, and flex modifiers;
- context and output limits plus cache capabilities;
- provider-native paid-tool prices;
- capability tags, source URLs, and reviewed date.

Routes that do not have an exact, validated provider path remain visible as unavailable catalog
entries. They cannot execute until a code change supplies the missing route and price dimensions.
The implementation must not invent route identifiers for model names that lack them in current
production configuration.

### Authoritative launch rates

All prices are USD per one million actual tokens.

| Tier | Token class | Provider ceiling | Customer rate |
| --- | --- | ---: | ---: |
| Small | Uncached input | $0.20 | $0.22 |
| Small | Cache read | $0.02 | $0.022 |
| Small | Cache write | $0.25 | $0.275 |
| Small | Output/reasoning | $1.20 | $1.32 |
| Medium | Uncached input | $0.40 | $0.44 |
| Medium | Cache read | $0.04 | $0.044 |
| Medium | Cache write | $0.50 | $0.55 |
| Medium | Output/reasoning | $3.20 | $3.52 |
| Large | Uncached input | $2.00 | $2.20 |
| Large | Cache read | $0.20 | $0.22 |
| Large | Cache write | $2.50 | $2.75 |
| Large | Output/reasoning | $12.00 | $13.20 |
| Flagship | Uncached input | $10.00 | $11.00 |
| Flagship | Cache read | $1.00 | $1.10 |
| Flagship | Cache write | $12.50 | $13.75 |
| Flagship | Output/reasoning | $45.00 | $49.50 |

Catalog validation proves that every published rate is exactly ceiling x 1.10. These launch
values are not changed automatically by external price lookup.

### Initial model classification

| Model or route | Tier |
| --- | --- |
| DeepSeek V4 Flash / 0731 | Small |
| GPT-5.6 Luna, standard context | Small |
| GPT-5 mini | Medium |
| Qwen 3.5 122B approved routes | Medium |
| Qwen 3.6 Plus approved route | Medium |
| Claude Haiku 4.5 | Large |
| GLM 5.1 approved routes | Large |
| GPT-5.6 Terra, standard context and service | Large |
| GPT-5.5 standard and long-context variants | Flagship |
| Claude Opus 4.8 | Flagship |
| GPT-5.5 Pro | Unsupported |

Planning, coding, and review built-ins use approved Large routes. Other built-in production tasks
initially use approved Small routes. Existing recognized aliases canonicalize to catalog entries.
Unknown, unpriced, over-Flagship, or ambiguous routes remain visible on legacy custom agents but
cannot launch.

Custom agents select from a catalog grouped by model size. Built-in agents show a non-editable
size. Free-text model entry is removed.

OpenRouter calls are pinned or maximum-price constrained. Missing cache pricing means caching is
unsupported, not free. Requests above the 272K-input standard-context boundary compact or stop
before launch unless an explicit eligible rate variant exists. Premium fast service is disabled
unless its exact route passes tier validation.

## Metering and Calculation

### Normalized telemetry

Every provider/runtime response becomes:

- `input_tokens_total`
- `uncached_input_tokens`
- `cache_read_tokens`
- `cache_write_tokens`
- `output_tokens`
- `reasoning_tokens`

```text
uncached input = max(total input - cache read - cache write, 0)
```

Reasoning uses the output/reasoning rate. If a provider's completion total includes reasoning,
normalization subtracts reasoning from ordinary output so it is charged exactly once.

### Hosted usage

Each immutable rate is stored as micro-USD per one million tokens. The event calculation uses
checked integer arithmetic and half-up rounding once, after summing all token fractions and
provider-native paid tools:

```text
published charge =
    uncached input * tier input rate
  + cache read     * tier cache-read rate
  + cache write    * tier cache-write rate
  + output         * tier output rate
  + reasoning      * tier output rate
  + paid provider tools
```

The final event is rounded to the nearest micro-dollar. No intermediate token component is
rounded. The maximum event rounding error is below $0.000001.

### Customer-funded usage

For ChatGPT device-code authentication and future customer API keys, the customer pays the model
provider. Helpin records full telemetry and published equivalent value, then charges 10% of the
published token and provider-tool value as orchestration usage. Only that orchestration amount
changes the allowance percentage or becomes extra usage.

### Free and estimated usage

Agent drafting, prompt improvement, automation and flow setup, import setup, and company-context
generation are promotional. Their raw tokens and would-have-been value are recorded but not
charged.

Embeddings record provider, model, input count, and internal cost but remain unbilled.

If successful output lacks telemetry, Helpin preserves it and creates an estimated entry using
the same task nature, tier, and funding mode's 30-day trimmed mean. Estimates require at least ten
successful actual observations, discard the lowest and highest 10% by charge, and exclude prior
estimates. Sparse tasks use a code-owned conservative launch estimate. Operators receive a
content-free alert.

## Persistence Boundaries

Add versioned SQL migrations in `server/internal/dbmigrate/sql/` and matching Go models.

### AI usage periods

`billing_ai_usage_periods` is the transactional allowance source of truth. It stores workspace,
start/end, nullable allowance, posted usage, overage, active reservations, unlimited state,
status, pricing version, and timestamps. A partial unique index permits only one open period per
workspace.

### Immutable usage ledger

`billing_ai_usage_ledger` stores workspace and period references; entry kind; task/feature and
category; tier; provider/model/route/service/funding identity; immutable rate snapshot; five raw
token counts plus total input; provider-tool components; hosted published value; customer-funded
equivalent and orchestration value; final charged micro-USD; actual/estimated status; estimation
method/sample size; idempotency key; sanitized references; and creation time.

Historical entries never consult the current catalog. Updates and deletes are denied at the
application layer and protected by database triggers after activation.

### Reservations

`billing_ai_usage_reservations` stores the workspace, period, task nature, tier, execution,
idempotency key, reserved/consumed values, status, expiry, heartbeat, and timestamps. Active
reservations count against availability. Terminal reconciliation writes the ledger and updates
the period and reservation in one transaction.

### Settlements

`billing_ai_usage_settlements` stores exact overage, rounded cents, rounding adjustment, Stripe
references, stable idempotency key, status, attempts, last sanitized error, and timestamps. It
prevents retries and webhooks from duplicating invoice lines.

### Task estimates

A daily aggregate stores task nature, tier, funding mode, sample size, average token breakdown,
trimmed mean charge, P50, P90, and calculation time. Routing or pricing-version changes trigger
recalculation.

## Reservations and Runtime Budgets

When extra usage is disabled, preflight locks the open period and reserves the greater of the
task's observed 30-day P90 and a deterministic bound derived from input estimate, maximum output,
route modifiers, and paid-tool exposure.

Direct calls release reservations after provider failure and reconcile after success. If an
instrumentation defect produces a final charge above the reservation, Helpin charges no more than
reserved, absorbs the difference, and alerts operators.

Long-running runtimes receive `max_billable_microusd` and emit cumulative usage slices by route
and modifier with monotonic semantic/event sequence numbers. Before each provider call, the
runtime ensures the bounded request fits. It may compact, safely reduce output, or stop cleanly.
Checkpoints resize the reservation transactionally. One terminal event reconciles one ledger
entry idempotently.

A sweeper releases stale reservations only after confirming the execution is terminal or absent;
wall-clock expiry alone never releases an active run.

## API Contracts

Replace credit fields in workspace cards, billing summaries, organization rollups, plan previews,
analytics, and upgrade dialogs with:

- allowance, used, remaining, reserved, and overage micro-USD values;
- used, remaining, and reserved display percentages;
- AI period start/end and unlimited state;
- extra-usage enabled/available state;
- pricing version.

Micro-USD values remain in authenticated API contracts for accurate clients but the Helpin UI
does not render included allowance dollars. Public pricing exposes tier definitions, rates, plan
entitlements, and observed/fallback task examples from the canonical catalog.

Usage history returns daily/cumulative charge, allowance percentages, actual token classes,
tier, actions, estimate counts, promotional usage, tool usage, and period totals. It removes base
credit costs and typical-minimum fields.

Typed failures are:

- AI allowance exhausted;
- extra AI usage disabled;
- extra AI usage unavailable;
- model unavailable under current pricing;
- pricing configuration missing;
- execution stopped before exceeding allowance;
- settlement pending or payment failed.

All launch surfaces continue classifying these through `getUpgradeRequiredReason` and
`UpgradeRequiredDialog`; raw billing errors never appear in toasts.

## Stripe Lifecycle

Remove the credit-block price configuration, creation script, invoice wording, and
`BillCreditBlock`. Existing subscriptions, customers, cards, taxes, and plan prices remain.

At period close, one database transaction snapshots exact overage, closes the period, and creates
a pending settlement. It opens the next period independently. No Stripe request occurs while a
database row is locked.

The settlement worker rounds exact overage to cents using half-up rounding and records the
difference:

```text
rounded invoice cents = half_up(exact overage micro-USD / 10,000)
rounding adjustment = rounded cents * 10,000 - exact overage micro-USD
```

For monthly subscriptions, the worker attaches one `Extra AI usage — <start> to <end>` item to
the draft renewal invoice before finalization. For annual subscriptions, it creates and finalizes
one out-of-cycle invoice after each monthly AI period. Stable idempotency includes workspace,
period, pricing version, and settlement version.

Webhook reconciliation matches both invoice paths. Repeated failure uses existing past-due and
payment-failed policy and may eventually lock hosted AI without preventing a new period from
opening during a temporary Stripe outage.

## Atomic Activation and Migration

Deployment activates the new pricing version immediately. The cutover migration is idempotent and
performs these operations in a transaction where PostgreSQL permits:

1. Create periods, ledger, reservations, settlements, estimates, constraints, and indexes.
2. Rename `on_demand_enabled` to `extra_ai_usage_enabled`, preserving customer choices.
3. Open one new period per workspace at zero usage with the full allowance for its current plan
   and billing interval; Founder periods are unlimited.
4. Set the server-side active pricing version to the new catalog version.
5. Rename `billing_credit_ledger` to `billing_credit_ledger_legacy` and install immutability
   protection.
6. Remove application access and all credit services, DTOs, tests, scripts, analytics properties,
   and Stripe credit-product configuration.
7. Drop `included_credits`, `credits_used`, `on_demand_blocks_invoiced`, and other obsolete credit
   columns after the new period rows exist.

Legacy usage is neither converted nor deducted. Legacy rows remain queryable for audit support
but never appear in the new balance.

Because this is an immediate atomic replacement, rollback is a forward-fix operation. Deployment
must stop if catalog validation, migration validation, or representative settlement tests fail.

## Operational Safety

The catalog validation command and CI fail for missing model/tool rates, alias collisions, tier
ceiling violations, incomplete cache/modifier pricing, unsupported built-in routing, incorrect
10% markup, or frontend/website snapshot drift.

Internal reporting groups provider cost, published value, subscription-covered usage, invoiced
extra usage, customer-funded orchestration, promotion, embeddings, paid tools, gross margin,
cache ratios, estimates, telemetry loss, and reservation accuracy by route, task, workspace plan,
and tier.

Alerts cover negative-margin routes, provider cost above catalog, missing telemetry, unknown
models/tools, settlement failures, unreconciled terminal runs, reservation leaks, negative
availability, duplicate idempotency, and cache-hit regressions. Alerts contain identifiers and
configuration metadata, never prompts or customer content.

## Testing and Acceptance

### Calculation tests

Cover all token classes and tiers; reasoning separation; hosted/customer-funded charging;
promotional and embedding behavior; tools and modifiers; checked micro-USD arithmetic; invoice
rounding; missing telemetry; alias resolution; unsupported models; and catalog ceilings.

### Repository and concurrency tests

Cover concurrent reservations/consumption; idempotent runtime events; reservation resize,
reconcile, release, and recovery; period rollover; extra usage; unlimited Founder; monthly,
annual, and trial schedules; upgrades/deferred downgrades; settlement retries; and transaction
rollback.

### Provider/runtime tests

Cover OpenAI, Anthropic, OpenRouter, Codex, and native-runtime normalization; cache writes;
cumulative variants; long context; provider tools; pre-call budget enforcement; safe stopping;
and successful output with estimates.

### API and presentation tests

Cover removal of credit fields and terminology; percentage calculations and rounding; Founder;
reserved segments; exact extra-usage disclosures; token drill-downs; estimate/promotion labels;
upgrade dialogs; curated models; unsupported legacy models; organization aggregation; and pricing
snapshot consistency.

### Migration and release acceptance

Validate migration ordering/idempotency; snapshot and protect legacy rows; verify every workspace
gets a fresh full period; prove no credit-ledger writes; verify no credit-block Stripe items;
reprice representative telemetry; validate estimates; exercise every plan/funding path; and run
Go tests/build/vet plus frontend and website tests/builds.

## Explicit Product Changes From the Original Proposal

- The original dollar-first billing meter is replaced by a percentage-first meter.
- Included allowance dollars remain internal and in machine-readable contracts; the Helpin UI
  does not present them as a wallet or balance.
- Actual dollar amounts are shown for accrued extra usage and invoices.
- Credits and normalized units still disappear completely.
- Actual token counts remain available in drill-downs and are never presented as a fixed plan
  allowance.
- The replacement activates immediately instead of collecting several days of shadow data.
- The supplied launch rates are authoritative and are not changed during implementation.

## Out of Scope

- Changing subscription prices or plan names.
- Reintroducing credits, normalized tokens, prepaid packs, or fixed action promises.
- Billing embeddings in this release.
- Allowing unpriced models, arbitrary OpenRouter fallbacks, or GPT-5.5 Pro.
- Automatically changing customer rates from external provider-price lookups.
