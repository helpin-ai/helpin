---
name: market_research
description: Conducts source-backed public market, category, audience, trend, channel, and customer research using search and fetched URLs.
metadata:
  title: Market Research
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent needs public research beyond Helpin workspace context.

Adapted from research-heavy workflows across the MIT-licensed `customer-research`, `marketing-plan`, `content-strategy`, and `competitor-profiling` skills in `coreyhaines31/marketingskills`.

## Source Rules

- Use `web_search` to discover sources.
- Use `fetch_url` on exact URLs before citing or relying on claims.
- Use `crawl_url` only for official sites or docs/blog hosts when source discovery is thin.
- Prefer primary sources: official pages, docs, pricing pages, changelogs, public directories, credible publications, review sites, and community threads.
- Cite URLs in the output and mark inferences clearly.

## Research Types

The agent can research:

- Market/category landscape.
- ICP language and pain signals.
- Competitor positioning.
- Distribution channels.
- Content and SEO opportunities.
- Launch and partnership surfaces.

## Output

Create a research brief with:

- Research question.
- Sources reviewed.
- Findings with citations.
- Confidence level.
- Implications for positioning, campaigns, copy, or roadmap.
- Open questions and recommended next actions.

## Guardrails

- Do not cite search-result snippets without fetching the source.
- Do not claim access to private communities, paid tools, analytics, ad accounts, or enrichment tools.
- Use dates when recency matters.
