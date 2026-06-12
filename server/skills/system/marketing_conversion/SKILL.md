---
name: conversion_optimization
description: Audits conversion paths and creates CRO, signup, pricing, paywall, popup, and experiment recommendations using available Helpin context.
metadata:
  title: Conversion Optimization
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira evaluates landing pages, signup flows, pricing pages, onboarding starts, lead-capture paths, or experiment ideas.

Adapted from the MIT-licensed `cro`, `ab-testing`, `signup`, `pricing`, `paywalls`, and `popups` skills in `coreyhaines31/marketingskills`.

## Audit Order

Analyze in this order:

- Audience-message fit: is the page or flow for the right person?
- Value proposition: is the promised outcome clear?
- Friction: forms, steps, copy ambiguity, missing proof, confusing CTAs.
- Trust: proof, objections, risk reversal, credibility.
- Motivation: urgency, relevance, timing, perceived value.
- Measurement: what would prove whether the change helped?

## Experiment Backlog

For each recommendation, include:

- Hypothesis: if we change X for audience Y, metric Z should improve because...
- Variant idea: what changes.
- Primary metric and guardrail metric.
- Effort and risk.
- Priority: high, medium, low.

## Guardrails

- Do not invent analytics numbers.
- If conversion rates, traffic, sample size, or current performance are unknown, say so.
- Avoid recommending tests that cannot plausibly reach enough sample size.
- Use PM tasks for implementation follow-up only when configured.

