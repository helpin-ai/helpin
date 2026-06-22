# AI Usage Metering

## Product Model

Helpin shows customers **AI usage**, not AI task credits. The customer should understand how much monthly AI capacity has been used, how much remains, and when it resets. They should not need to memorize per-task prices before using AI.

Customer-facing surfaces should use:

- Monthly AI usage
- AI usage used
- AI usage remaining
- Extra AI usage
- AI usage packs

Internal backend code may continue to use credit-oriented names such as `included_credits`, `credits_used`, and `billing_credit_ledger` until a later API/schema cleanup.

## Plan Allowances

| Plan | Monthly AI usage |
| --- | ---: |
| Starter | 5,000 units |
| Growth | 25,000 units |
| Growth trial | 25,000 units |

Usage resets every billing period. Unused included usage does not roll over.

## Internal Formula

AI usage is calculated internally from model token usage plus a feature floor:

```text
weighted_tokens =
  input_tokens
  + output_tokens * 6
  + reasoning_tokens * 6
  - cached_input_tokens * 0.90

usage_units = max(feature_floor, ceil(weighted_tokens / 1000))
```

The `6x` output and reasoning multiplier reflects the higher cost of generated and reasoning tokens. Cached input receives a `90%` discount. Weighted usage can exceed the feature floor on large runs.

## Feature Floors

The highest feature floor is `100` usage units. No action should have a minimum above `100`, though actual token-based usage may exceed `100`.

| AI action | Floor |
| --- | ---: |
| AI triage/routing | 2 |
| Coverage gap analysis, per conversation | 2 |
| CRM signal detection | 3 |
| Rewrite/improve support reply | 4 |
| Command bar read-only answer | 4 |
| Deal automation inference | 5 |
| CRM summary | 6 |
| Support reply draft | 8 |
| Support task draft | 8 |
| Docs AI section generation | 15 |
| Help article translation | 15 |
| Help article generation/update | 20 |
| Built-in lightweight agent run | 40 |
| Scribe task planning run | 50 |
| Mira marketing run | 50 |
| Quill documentation run | 60 |
| Custom agent run | 60 |
| Atlas epic planning run | 80 |
| Forge coding run | 100 |
| Lens review run | 100 |
| Custom coding/review agent run | 100 |

## Free Setup Actions

Do not charge AI usage for rare setup and activation actions:

- Custom agent draft/config generation
- Creating or editing automations
- Creating or editing flows
- Agent prompt improvement
- Inbox, help center, docs, CRM, and import setup

Charge when the configured AI work runs, not when users configure it.

## Implementation Wiring

AI usage is currently metered in two paths:

- Direct LLM calls wrap the request context with `WithAIUsageMetering(...)`; `MeteredLLMProvider` first runs a simple preflight check using the feature floor, then records final usage after the provider returns token usage.
- Setup/config calls that are intentionally free must use `WithAIUsageMeteringExempt(...)`. A metered provider call without usage context or an explicit exemption fails closed.
- Agent runtimes call `PreflightAgentRunAIUsage(...)` before native SDK, Codex, or OpenCode execution starts, then call `RecordAgentRunAIUsage(...)` after token totals are persisted on the `agent_run`.

Direct LLM metering is wired for support routing/replies/rewrite/task draft fallback, inbox triage, coverage gap analysis, coverage docs suggestions/enrichment, docs AI section generation, help center translation and auto-translation, CRM signal detection, CRM summaries, deal automation inference, and command-bar routing/answers.

Agent run metering is wired for built-in agents, custom agents, and coding/review agents through the shared runtime callback. The feature key is selected from the agent preset where possible; custom coding/review agents receive the coding/review floor.

The metering layer uses idempotency keys so retries or repeated persistence steps do not double-charge the same billable execution. Workspace attribution is required; unattributed internal previews do not consume AI usage.

Usage limit enforcement must happen inside the database transaction that locks the workspace billing row. Service-level prechecks are only early rejection hints; the locked row is the source of truth for included usage, extra usage availability, and workspace lock status.

The direct LLM preflight is intentionally simple: it blocks calls when the workspace is locked, past included AI usage without extra usage enabled, or unable to buy extra usage. It uses the feature floor because exact token usage is unknown before the model call. Final charging still uses the persisted token usage and the transaction-level billing checks.

Extra usage billing is part of the locked billing-row transaction. If a usage event requires a new 5,000-unit paid pack and the Stripe charge fails or times out, the usage ledger, usage counter, and invoiced block count do not commit. The Stripe request uses a short timeout to keep row locks and database connections bounded; a durable charge/reconciliation queue is the next step if this path needs higher throughput.

Embeddings, setup/config generation, and feature configuration are intentionally not metered. SLA limits are not part of this system yet and should be added when the SLA feature ships.

## Customer Explanation

The main app should show a simple usage meter, for example:

```text
AI usage
3,420 / 5,000 used
1,580 remaining
Resets July 1
```

Usage history may show categories such as Support AI, Docs AI, CRM AI, Agents, and Automation. Avoid presenting those categories as a pricing menu. Help copy should say that short routing/classification uses little AI usage, while drafting, translation, article generation, and agent runs use more because they read more context and produce longer outputs.
