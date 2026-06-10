---
name: marketing_context
description: "Creates and maintains the workspace marketing context for Mira: product overview, ICP, positioning, objections, voice, competitors, proof, and goals."
metadata:
  title: Marketing Context
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira needs foundational marketing context before planning, writing, or reviewing growth work.

Adapted from the MIT-licensed `product-marketing` skill in `coreyhaines31/marketingskills`.

## Source Of Truth

- Use Helpin Docs as the source of truth for marketing context.
- Do not write or look for `.agents/product-marketing.md`, `.claude/product-marketing.md`, or local marketing-plan folders.
- If a marketing context document already exists, read it first and update only the stale or missing sections.
- If no context document exists, create a new internal Docs document in the configured destination.

## Sections

Capture these sections:

- Product overview: one-liner, category, product type, business model.
- Target audience: company type, roles, primary use cases, jobs to be done.
- Personas: user, champion, decision maker, financial buyer, technical influencer when relevant.
- Problems and pain points: functional cost, emotional tension, current workaround.
- Competitive landscape: direct, secondary, and indirect alternatives.
- Differentiation: capabilities, benefits, why customers choose this product.
- Objections and anti-personas: top objections, who is not a fit.
- Switching dynamics: push, pull, habit, anxiety.
- Customer language: verbatim phrases, words to use, words to avoid.
- Brand voice: tone, style, personality, banned claims.
- Proof points: metrics, logos, testimonials, value themes.
- Goals: business goal, conversion action, current metrics when known.

## Operating Rules

- Prefer exact customer language from CRM notes, support conversations, sales notes, and existing Docs.
- Mark unknowns explicitly instead of inventing positioning.
- Separate confirmed facts from assumptions.
- When context is incomplete, ask focused questions with `request_user_input` or create an "Open questions" section.
- Do not claim access to live analytics, SEO tools, ad platforms, email platforms, enrichment tools, or external research unless those tools are explicitly available in the run.
