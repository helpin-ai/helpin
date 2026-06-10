---
name: marketing_plan
description: Builds practical marketing plans, growth roadmaps, campaign plans, and prioritized marketing task backlogs from Helpin context.
metadata:
  title: Marketing Plan
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira needs to turn product context, customer signals, existing Docs, CRM context, and user goals into a marketing plan.

Adapted from the MIT-licensed `marketing-plan` and `marketing-ideas` skills in `coreyhaines31/marketingskills`.

## Planning Frame

Use AARRR as the default structure:

- Acquisition: how strangers become aware.
- Activation: how a new lead or user reaches first value.
- Retention: how users or customers stay engaged.
- Referral: how retained customers bring more customers.
- Revenue: how the business monetizes and expands.

## Required Output

Create a Helpin Docs report with:

- Executive summary: 3 big bets, 90-day priorities, expected outcome.
- Strategic frame: ICP, positioning, current constraints, brand voice.
- Current state: what's working, stuck, missing, in-flight, or unknown.
- AARRR plan: recommendations by stage with rationale.
- 90-day roadmap: owner/team suggestions, sequence, dependencies.
- Task backlog: concrete tasks suitable for PM creation.
- Measurement plan: leading indicators and open metric questions.
- Open decisions: missing budget, CAC, conversion, team, or tool data.

## Task Creation

- Create PM tasks only when the flow explicitly enables follow-up tasks.
- Group related recommendations into actionable tasks instead of creating one task per idea.
- Include acceptance criteria in task descriptions.
- Use configured destination team and stage directly when provided.

## Guardrails

- Do not assume paid budget, analytics access, or external tool data.
- Be operationally honest about what the team can execute.
- Mark ideas as Now, Next, Later, or Skip.
- Prefer useful sequencing over exhaustive tactic lists.

