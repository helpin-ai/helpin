---
name: seo_research
description: Performs source-backed SERP, topic, content-gap, comparison-page, and AI-visibility research without keyword-volume or backlink tools.
metadata:
  title: SEO Research
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent researches search demand patterns, content gaps, comparison pages, AI visibility opportunities, or SERP/source landscape with available web tools.

Adapted from the MIT-licensed `seo-audit`, `content-strategy`, `ai-seo`, `schema`, and `programmatic-seo` skills in `coreyhaines31/marketingskills`.

## What This Skill Supports

- Find visible pages and recurring content patterns.
- Identify questions, comparison topics, alternatives, templates, and guides.
- Review public source structure for AI answerability.
- Recommend schema and content structure.
- Build briefs and roadmaps.

## What This Skill Cannot Claim

Do not claim keyword volume, ranking position, backlink count, traffic, Core Web Vitals, Google Search Console, GA4, Ahrefs, Semrush, or DataForSEO data unless those tools are explicitly available.

## Research Process

- Use `web_search_exa` for topic and source discovery.
- Fetch exact pages before citing.
- Prefer official docs, competitor pages, high-quality guides, community questions, and review content.
- Label search opportunities as hypotheses unless supported by dedicated SEO data.

## Output

Create an SEO research brief with:

- Topic clusters and intent.
- Existing source examples.
- Content gaps.
- Briefs with audience, angle, outline, CTA, and proof needed.
- AI visibility recommendations.
- PM tasks when configured.
