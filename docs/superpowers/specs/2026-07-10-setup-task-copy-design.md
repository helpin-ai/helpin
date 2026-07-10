# Setup Task Copy Simplification

**Date:** 2026-07-10
**Status:** Approved direction

## Goal

Make every Setup & Success checklist item immediately understandable to a new customer.

## Copy rule

Each task is presented as one short, plain-language sentence that says both:

1. what the customer should do; and
2. why the action matters.

Use an imperative verb followed by a concise purpose clause. Prefer familiar product language and avoid adoption terminology such as “assisted value,” “delivery loop,” “coverage improvement,” “closeout reviewable,” or “unattended value.”

Examples:

- “Add company details so Helpin understands your business.”
- “Create your first task so your team has real work to track.”
- “Resolve a customer conversation to confirm your support process works.”
- “Run an agent on real work to see how it saves your team time.”
- “Turn on an automation so repeat work can happen automatically.”

## UI treatment

- Render one task sentence; do not render a separate task description.
- Keep the stage, personal-scope, completion, attention, and blocked states as compact labels.
- Keep action buttons short and direct.
- Surface permission or entitlement blockers as compact status information, not as a second explanatory paragraph.
- Preserve the existing hierarchy, colors, progress calculation, actions, and evidence behavior.

## Scope

Rewrite the first-release Foundation, Product delivery, Customer support, and Automation task labels. No journey logic, evidence rules, permissions, routes, or completion semantics change.

## Verification

- Add a catalog-level regression test for plain one-sentence task labels.
- Add a frontend regression check that task descriptions are not rendered.
- Run focused backend/frontend tests, TypeScript, and production builds.
