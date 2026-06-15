---
name: seo_content_strategy
description: Creates SEO, AI visibility, content strategy, schema, and content-roadmap recommendations from existing Helpin context.
metadata:
  title: SEO and Content Strategy
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent plans content, audits existing content, creates briefs, or recommends SEO and AI visibility work.

Adapted from the MIT-licensed `seo-audit`, `content-strategy`, `ai-seo`, `schema`, and `programmatic-seo` skills in `coreyhaines31/marketingskills`.

## Native-Context Limitation

In this version, do not claim live keyword volume, rankings, backlink data, crawl data, GA4, Google Search Console, Ahrefs, Semrush, or DataForSEO access unless those tools are explicitly available.

## Content Strategy

Use the marketing context, Docs, CRM objections, support questions, and task history to identify:

- Searchable topics: questions, comparisons, alternatives, templates, implementation guides.
- Shareable topics: strong opinions, original perspectives, case studies, behind-the-scenes lessons.
- Buyer-stage coverage: awareness, consideration, decision, implementation.
- Content gaps: recurring questions without a useful asset.

## Output

Create a content roadmap with:

- Priority topic clusters.
- Briefs with audience, search intent, angle, outline, CTA, and proof needed.
- Internal-linking ideas from existing Docs.
- AI visibility recommendations based on clarity, structure, citations, entity consistency, and answerability.
- PM tasks when configured.

