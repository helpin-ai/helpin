# AI usage metering

Use this guide when adding an AI call, investigating recorded usage, or checking
which edition owns charging. Both editions record execution usage. **Community
has no financial pricing policy; EE adds pricing, reservations, and settlement.**
The build selects the lifecycle; an environment variable cannot turn a Community
binary into EE. See [AI connections](ai-connections.md) for model selection.

## Product model

Keep model execution, usage telemetry, and financial policy separate:

- Product services identify the workspace, action, feature, and idempotency key.
- The selected connection/profile determines the provider, model, and accepted
  policy for the execution.
- Provider token usage is normalized and recorded through the edition's lifecycle.
- EE computes charges using the accepted rate snapshot and funding mode.

The old weighted-token formula, per-feature credit floors, and 5,000-unit pack
examples previously on this page describe the legacy credit system. They are
not the current token-priced AI usage contract. Some legacy billing schema and
code remain; their presence does not make them the active AI metering path.

## Community

[CommunityAIUsage](../server/internal/service/ai_usage_community.go) implements
the shared lifecycle without a price catalog or financial reservation. It
validates the accepted Community policy, normalizes tokens, and persists
execution usage. Charge calculation returns zero. This does not mean the
configured external model provider is free; its account remains separately owned.

## EE pricing and funding

The [EE usage service](../server/ee/service/ai_usage.go) resolves an eligible route,
freezes the pricing context, reserves usage before execution, and reconciles the
actual result. Rates and route eligibility come from the versioned
[pricing catalog](../server/ee/pricing/catalog.json), rather than a universal
multiplier or per-feature floor table.

The [calculator](../server/ee/pricing/calculator.go) uses integer micro-USD and
separate token classes: uncached input, cache reads, cache writes, output, and
reasoning. Paid tool usage is included where applicable. Funding mode determines
which portion becomes the customer charge. Do not calculate a second charge in
a product handler or copy current catalog prices into a feature guide.

For customer-provided credentials using flat-token pricing, the
[flat tariff path](../server/ee/service/ai_usage_flat.go) requires a versioned USD
tariff with an explicit nonnegative rate and supported accounting version. It
copies the accepted tariff and tool rates so later configuration changes do not
mutate an already accepted execution. This path does not replace governed media
operation pricing.

## Adding a direct model call

Use the existing [metered provider](../server/internal/service/ai_usage_meter.go)
and action-policy registry rather than calling the provider around the lifecycle.

1. Supply workspace, action, feature, and idempotency context through
   `WithAIUsageMetering`.
2. Let the provider wrapper resolve policy and preflight the execution.
3. Record the provider's actual usage through the same accepted context.
4. Use `WithAIUsageMeteringExempt` only for an intentionally exempt call; it is
   not a general fallback for missing context. The wrapper can reject calls
   without metering context or an explicit exemption.

Whether an action is exempt belongs to its current policy and call site. Do not
assume all setup, preview, embedding, or import work is exempt based on its UI
label. In particular, [support previews](support-ai-preview.md) use the normal
execution and edition policy even though they do not send customer messages.

## Agent executions

Helpin preflights the accepted run and passes the selected model and policy to
Agent Runtime. Runtime usage callbacks and checkpoints feed the shared lifecycle;
settlement uses persisted execution telemetry. A Runtime-delivered support reply
belongs to its run rather than creating an independent legacy reply-floor charge.

Use stable idempotency keys across retries. The EE repository owns transactional
reservation, checkpoint, and reconciliation behavior; service-level hints cannot
replace that boundary. Releasing or cancelling execution must preserve already
recorded usage and the accepted policy.

## Code and verification

| Responsibility | Source |
| --- | --- |
| Shared lifecycle types | [aiusage](../server/internal/aiusage/) |
| Product call context and metered provider | [AI usage meter](../server/internal/service/ai_usage_meter.go) |
| Community recording | [Community lifecycle](../server/internal/service/ai_usage_community.go) |
| EE reservation and reconciliation | [EE usage service](../server/ee/service/ai_usage.go) |
| EE transactional persistence | [Usage repository](../server/ee/repository/ai_usage.go) |
| Pricing arithmetic and funding modes | [Calculator](../server/ee/pricing/calculator.go) |
| Accepted BYOK tariff | [Flat tariff resolution](../server/ee/service/ai_usage_flat.go) |

When changing metering, verify retry idempotency, token normalization, accepted
rate immutability, rejected/missing policy, and edition behavior. The colocated
Community, EE service/repository, and pricing tests cover these boundaries.
Source inspection does not establish a deployment's active prices, allowances,
or successful settlement; inspect its configured policy and ledger separately.
