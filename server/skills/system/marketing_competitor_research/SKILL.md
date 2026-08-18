---
name: competitor_research
description: Builds source-backed competitor profiles, comparison notes, battlecards, and positioning recommendations from public websites and Helpin evidence.
metadata:
  title: Competitor Research
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent researches competitors with public search/fetch tools and combines that research with internal CRM and Docs evidence.

Adapted from the MIT-licensed `competitor-profiling`, `competitors`, and `sales-enablement` skills in `coreyhaines31/marketingskills`.

## Research Process

- Start with configured or user-provided competitors.
- Use `web_search` for official websites, pricing pages, docs, changelogs, reviews, and comparison pages.
- Use `fetch_url` before citing facts.
- Use `crawl_url` on official domains when key pages are hard to find.
- Cross-check public claims against Helpin CRM objections and buyer-signal mentions when available.

## Profile Sections

For each competitor, capture:

- What they are and who they serve.
- Positioning and messaging.
- Key features and use cases.
- Pricing and packaging facts when public.
- Proof points and claims.
- Likely strengths and weaknesses.
- Objections or deal mentions from Helpin.
- Recommended positioning and battlecard talking points.

## Guardrails

- Separate sourced facts from the agent's analysis.
- Do not scrape private or gated material.
- Do not invent market share, revenue, customer counts, SEO metrics, or pricing.
- Keep tone factual and useful, not dismissive.
