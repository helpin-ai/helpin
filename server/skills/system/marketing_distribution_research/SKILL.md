---
name: distribution_research
description: Finds and evaluates public distribution channels such as directories, communities, newsletters, podcasts, launch surfaces, and partner opportunities.
metadata:
  title: Distribution Research
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent needs to find places to distribute a launch, asset, product, opinion, lead magnet, or campaign.

Adapted from the MIT-licensed `directory-submissions`, `community-marketing`, `co-marketing`, `launch`, and `social` skills in `coreyhaines31/marketingskills`.

## Research Sources

Use search and fetch tools to find:

- Product directories and marketplaces.
- Niche communities and forums.
- Newsletters and podcasts.
- Partner ecosystems.
- Creator or influencer surfaces.
- Launch calendars and announcement sites.
- Relevant GitHub or developer communities when appropriate.

Fetch exact pages before recommending a channel.

## Evaluation

Score each channel by:

- ICP fit.
- Submission or participation requirements.
- Credibility and audience quality.
- Effort and asset needs.
- Risk of spam or low-quality attention.
- Next action.

## Output

Create a distribution plan with:

- Prioritized channels.
- Source URLs.
- Why each channel fits.
- Required copy/assets.
- Outreach or submission draft.
- PM tasks when configured.

## Guardrails

- Do not submit, post, message, or publish externally.
- Do not recommend low-trust directories just because they are easy.
- Prefer channels where the offer matches audience intent.
