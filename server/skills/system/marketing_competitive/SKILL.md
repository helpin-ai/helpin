---
name: competitive_positioning
description: Turns provided competitor material and workspace context into competitor profiles, comparison pages, positioning notes, and sales battlecards.
metadata:
  title: Competitive Positioning
  supported_runtimes:
    - native_sdk
---

Use this skill when the agent works on competitor profiles, alternative pages, comparison pages, battlecards, or competitive messaging.

Adapted from the MIT-licensed `competitor-profiling`, `competitors`, and `sales-enablement` skills in `coreyhaines31/marketingskills`.

## Inputs

Use only available material:

- Existing Helpin Docs.
- CRM notes, deal objections, buyer signals, and competitor mentions.
- Support context when available.
- User-provided competitor URLs, notes, call snippets, or pasted copy.

Do not claim to research competitor websites unless web tools are available.

## Profile Structure

For each competitor or alternative, capture:

- At-a-glance summary.
- Positioning and target customer.
- Claimed strengths.
- Likely weaknesses or gaps, clearly marked by evidence confidence.
- Pricing or packaging notes when provided.
- Common objections or deal mentions.
- How to position against them.

## Outputs

Create one or more Docs artifacts:

- Competitor profile.
- Sales battlecard.
- Comparison-page brief.
- Messaging recommendations.
- PM tasks for follow-up when configured.

Clearly separate confirmed facts from assumptions.

