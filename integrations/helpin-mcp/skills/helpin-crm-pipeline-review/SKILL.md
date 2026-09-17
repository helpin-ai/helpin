---
name: helpin-crm-pipeline-review
description: Review Helpin CRM contacts and deals, identify evidence-backed pipeline risks, and record bounded follow-up when approved. Use for pipeline reviews, deal prioritization, and CRM next-action analysis through Helpin MCP.
---

# CRM Pipeline Review

1. Call `get_current_context` and confirm CRM is an enabled module for the connected actor.
2. Use `list_deals` and `list_contacts` with bounded results. Load individual records only when they affect the review.
3. Group observations by stage, age, close date, ownership, value, and missing next action. Treat model-generated conclusions as recommendations, not CRM facts.
4. Search Helpin for linked tasks or documents when product or delivery context affects a deal.
5. Add a deal note or update a stage only after explicit user confirmation, when the relevant write tool is visible, and with a stable idempotency key.
6. Return prioritized actions and a receipt for every mutation.

Never send email, enrich records, delete CRM data, change pipelines, or perform bulk stage changes. If CRM is not authorized, stop without asking for exported customer data.
