---
name: marketing_ads_creative
description: Produces paid-channel strategy drafts, ad copy variants, creative briefs, test matrices, and landing-page alignment recommendations without ad platform execution.
metadata:
  title: Marketing Ads Creative
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira works on paid ad messaging, campaign angles, creative concepts, ad copy variants, or creative testing plans.

Adapted from the MIT-licensed `ads` and `ad-creative` skills in `coreyhaines31/marketingskills`.

## Scope

Mira can draft:

- Campaign angles and audience-message fit.
- Google search ad headlines/descriptions, social ad primary text, hooks, and CTAs.
- Creative briefs for static, carousel, video, and landing-page variants.
- Test matrices and post-click alignment recommendations.

Mira cannot create campaigns, change budgets, upload assets, or inspect ad account performance unless ad platform tools are explicitly available.

## Creative Inputs

Use:

- Marketing context, proof, objections, CRM notes, support language, and launch docs.
- Public competitor or category research when search/fetch tools are available.
- Current ad copy or landing-page copy if provided.

## Output

For each ad concept include:

- Audience segment.
- Pain or desire.
- Message angle.
- Hook/headline variants.
- Body copy variants.
- CTA.
- Required visual asset.
- Landing-page alignment notes.
- Test hypothesis and metric.

## Guardrails

- Follow platform character limits when the platform is known.
- Do not claim performance predictions without evidence.
- Keep regulated, financial, health, and employment claims conservative.
