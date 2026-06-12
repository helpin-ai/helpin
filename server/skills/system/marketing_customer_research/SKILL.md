---
name: customer_research_synthesis
description: Synthesizes customer, support, CRM, and sales evidence into voice-of-customer themes, positioning insights, objections, and growth opportunities.
metadata:
  title: Customer Research Synthesis
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira analyzes existing workspace material for customer language, ICP insight, objections, churn reasons, conversion blockers, or campaign inputs.

Adapted from the MIT-licensed `customer-research` skill in `coreyhaines31/marketingskills`.

## Evidence Sources

Use only sources available in the run:

- Helpin Docs and linked documents.
- CRM contacts, deals, buyer signals, and notes.
- Support conversations or support-derived context when exposed to Mira.
- Task descriptions, task comments, launch notes, and explicit user input.

## Extraction Framework

For each source, extract:

- Jobs to be done: functional, emotional, and social jobs.
- Pain points: recurring frustrations and broken workflows.
- Trigger events: why customers start looking now.
- Desired outcomes: what success looks like in customer words.
- Objections: concerns that block conversion or expansion.
- Alternatives: competitors, workarounds, doing nothing, building internally.
- Verbatim language: phrases worth using or avoiding in copy.

## Confidence Rules

- High confidence: appears in 3+ independent sources or multiple segments.
- Medium confidence: appears in 2 sources or one strong source.
- Low confidence: single-source, old, ambiguous, or possibly biased.

Always label insights with confidence and source references when available.

## Output

Produce a concise research report with:

- Top themes ranked by frequency and intensity.
- Representative customer quotes or paraphrased source snippets.
- Implications for positioning, copy, product, lifecycle, or sales.
- Contradictions and sample-bias warnings.
- Recommended follow-up tasks or questions.

