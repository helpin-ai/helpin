---
name: lead_generation_strategy
description: Creates lead generation offers, lead magnets, free-tool briefs, referral ideas, and capture-path recommendations from Helpin context and optional public research.
metadata:
  title: Lead Generation Strategy
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent plans lead magnets, free tools, referral loops, directory/listing submissions, or other ways to create qualified demand.

Adapted from the MIT-licensed `lead-magnets`, `free-tools`, `directory-submissions`, and `referrals` skills in `coreyhaines31/marketingskills`.

## Inputs

Use:

- Marketing context, Docs, CRM signals, support questions, and task history.
- Public research from `web_search_exa`, `fetch_url`, or `crawl_url` when available and relevant.
- User-provided offers, audiences, constraints, examples, or existing funnel notes.

Do not submit listings, publish forms, change referral incentives, or connect external tools.

## Offer Types

Consider:

- Templates, checklists, calculators, diagnostic reports, benchmarks, teardown offers, email courses, webinars, and free tools.
- Referral or affiliate concepts when the product has existing happy customers or strong network effects.
- Directory and marketplace listings when search confirms relevant, reputable submission surfaces.

## Output

Create a lead generation brief with:

- Target segment and pain.
- Offer promise and why it is worth an email or meeting.
- Format and delivery path.
- Required proof, inputs, and assets.
- Capture CTA and follow-up sequence idea.
- Qualification criteria for sales or nurture.
- PM tasks when configured.

## Guardrails

- Do not invent conversion rates, list size, search volume, or referral economics.
- If using public sources, cite exact fetched URLs and label inferences.
- Prefer fewer, higher-intent offers over generic downloadable content.
