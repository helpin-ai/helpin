---
name: monetization_strategy
description: Creates pricing, packaging, upgrade, paywall, retention, and win-back recommendations from product, CRM, and customer context.
metadata:
  title: Monetization Strategy
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira works on pricing, packaging, upgrade paths, paywalls, trial-to-paid conversion, retention offers, or win-back strategy.

Adapted from the MIT-licensed `pricing`, `paywalls`, and `churn-prevention` skills in `coreyhaines31/marketingskills`.

## Inputs

Use:

- Product context, ICP, value proposition, plans, feature gates, and pricing notes.
- CRM deal objections, lost reasons, support issues, buyer signals, and customer language.
- Public competitor pricing pages when search/fetch tools are available.

## Analysis

Consider:

- Buyer segments and willingness-to-pay signals.
- Packaging clarity and plan differentiation.
- Upgrade triggers and expansion paths.
- Trial or freemium activation moments.
- Churn reasons and save-offer fit.
- Sales-assisted exceptions and discount policy.

## Output

Create a monetization brief with:

- Current-state diagnosis.
- Pricing or packaging recommendations.
- Upgrade/paywall copy drafts.
- Retention or win-back sequence ideas.
- Risks and open decisions.
- Experiment plan and PM tasks when configured.

## Guardrails

- Do not change billing, prices, subscriptions, discounts, or entitlements.
- Do not invent churn, conversion, or revenue numbers.
- Label competitor pricing facts with fetched source URLs when public research is used.
