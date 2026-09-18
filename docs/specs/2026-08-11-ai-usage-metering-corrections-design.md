# AI Usage Metering Corrections Design

> Superseded billing design (2026-08-11), source-compared on 2026-09-17. The
> fixed-unit prices, universal cache discount, and “no reservations” scope below
> describe an earlier system. Use [current AI usage metering](../ai-usage-metering.md).
>
> The [edition-neutral lifecycle](../../server/internal/aiusage/lifecycle.go)
> separates telemetry from financial policy. Enterprise
> [usage service](../../server/ee/service/ai_usage.go) reserves and settles micro-USD
> using the admitted pricing context; Community records telemetry without financial
> reservation/settlement. Legacy `AIUsageMeter.Consume` wrappers no longer implement
> the old fixed-unit charge. Current cache-read/write pricing follows admitted
> rates or flat tariff, not the blanket 10% rule in this design.
>
> Provider normalization remains relevant: [OpenAI decoding](../../server/internal/llm/openai.go)
> separates reasoning from completion totals, and
> [Anthropic decoding](../../server/internal/llm/claude.go) retains cache read/write
> counts. These implemented pieces do not make the old financial model current.
> This review did not execute charges or verify deployed billing configuration.

## Goal

Keep customer AI usage simple while correcting surprising or incomplete billing behavior.

## Decisions

- Command-bar work remains agent work. Remove the unused 4-unit command-bar feature rather than creating a second command-bar execution path.
- A runtime-generated support reply is covered by its agent-run usage. Keep the standalone 8-unit support reply charge only for direct LLM reply generation.
- Idempotency identifies one AI execution or one immutable source version, not an entity forever.
- Customer usage keeps one provider-independent formula. Normalize provider usage into total input, cached input, cache writes, output, and reasoning before applying it.
- Cached input remains charged at 10% of ordinary input. Cache-write premiums are tracked in metadata but do not change customer units.
- Running agents may finish after the workspace crosses its allowance; later AI actions remain responsible for enforcing the balance.

## Implementation

Extend the shared LLM token-usage value and provider decoders. OpenAI-compatible responses populate cached and reasoning details when returned, separating reasoning from completion totals that already include it. Anthropic responses populate cache-read and cache-creation fields. The meter continues to accept total input tokens plus cached input tokens, so its existing `input - 0.90 * cached_input` calculation remains equivalent to charging cached reads at 10%.

Replace entity-lifetime idempotency keys with keys containing the relevant source update time, triggering message, prompt hash, or execution identifier already available at the call site. Add tests at the smallest stable boundary for each behavior.

## Out of Scope

No reservations, live price estimates, strict per-run budgets, per-provider customer formulas, or billing UI expansion.
