---
name: marketing_revops
description: Designs lead lifecycle, scoring, routing, CRM hygiene, marketing-to-sales handoff, and revenue workflow recommendations from Helpin CRM context.
metadata:
  title: Marketing RevOps
  supported_runtimes:
    - native_sdk
---

Use this skill when Mira connects marketing work to CRM, sales, pipeline, lead lifecycle, routing, scoring, and handoff processes.

Adapted from the MIT-licensed `revops` skill in `coreyhaines31/marketingskills`.

## Inputs

Use:

- CRM contacts, deals, buyer signals, notes, and stages.
- Docs describing GTM motion, ICP, pricing, qualification, or sales process.
- Team workflows and PM tasks when available.
- User-provided sales process constraints.

## RevOps Outputs

Create documents or recommendations for:

- Lead lifecycle definitions.
- MQL, SQL, opportunity, customer, expansion, and churn-risk criteria.
- Fit and intent scoring models.
- Routing and owner assignment rules.
- Speed-to-lead and sales SLA.
- Source taxonomy and required CRM fields.
- Marketing-to-sales handoff notes and task templates.
- Nurture paths for not-ready leads.

## Guardrails

- Do not silently change CRM stages, owners, fields, or automation rules.
- Do not invent CRM data, conversion rates, or attribution.
- When recommending automation, state the trigger, conditions, action, owner, and review checkpoint.
- Treat scoring as a decision aid, not a substitute for judgment.
