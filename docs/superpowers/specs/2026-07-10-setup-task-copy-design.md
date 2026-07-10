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

## Final task copy

| Task key | One-sentence label |
|---|---|
| `foundation.company_context_ready` | Add company details so Helpin understands your business. |
| `foundation.team_ready` | Create a team so work has a clear owner. |
| `foundation.invite_sent` | Invite a teammate so you can work together in Helpin. |
| `foundation.member_joined` | Have a teammate join so progress can be shared. |
| `product.initial_work` | Create your first task so your team has real work to track. |
| `product.first_task_completed` | Complete your first task to confirm your workflow works. |
| `product.repeat_completion` | Complete another task on a different day to build a consistent habit. |
| `product.repository_ready` | Connect a code repository so Helpin can link work to what you ship. |
| `product.agent_result_used` | Run an agent on delivery work to save time on planning or review. |
| `product.sprint_closeout_reviewable` | Close a sprint so your team can review what was finished. |
| `product.release_notes_flow_succeeded` | Run the release notes automation so updates are created from shipped work. |
| `support.channel_ready` | Connect a support channel so customers can reach your team. |
| `support.delivery_validated` | Resolve a test conversation to confirm your support setup works. |
| `support.first_real_conversation_resolved` | Resolve your first customer conversation to complete the full support process. |
| `support.repeat_resolution` | Resolve another conversation on a different day to build a consistent support habit. |
| `support.knowledge_ready` | Add a knowledge source so your team and AI can give reliable answers. |
| `support.ai_reply_used` | Use AI to help answer a customer so you can respond faster. |
| `support.pm_task_linked` | Link a customer issue to a task so your product team can act on it. |
| `support.coverage_improvement_applied` | Apply a suggested knowledge improvement so future answers get better. |
| `automation.target_ready` | Choose real work to automate so the result will be useful. |
| `automation.first_assisted_value` | Complete your first agent run to see how Helpin can save time. |
| `automation.repeat_assisted_value` | Complete another useful agent run to make AI part of your workflow. |
| `automation.personal_contribution` | Complete an agent run yourself so you learn how the workflow works. |
| `automation.personal_repeat_contribution` | Complete another agent run yourself to build confidence using AI. |
| `automation.flow_enabled` | Turn on an automation flow so repeat work can run automatically. |
| `automation.triggered_value` | Run a triggered or scheduled automation to confirm it works without a manual start. |
| `automation.approval_resolved` | Review an automation approval so important actions stay under human control. |
| `automation.custom_agent_succeeded` | Run a custom agent successfully so it can help with your team’s specific work. |
| `automation.reliable_unattended_value` | Run the same automation successfully over time to make sure it is reliable. |

## UI treatment

- Render one task sentence; do not render a separate task description.
- “One line” means one textual sentence, not a forced single visual line.
- Allow the sentence to wrap naturally at narrow widths and browser zoom; never truncate or ellipsize it.
- Keep the complete sentence as the accessible task text associated with its action and state.
- Keep the stage, personal-scope, completion, attention, and blocked states as compact labels.
- Keep action buttons short and direct.
- Surface permission or entitlement blockers as compact status information, not as a second explanatory paragraph.
- Preserve the existing hierarchy, colors, progress calculation, actions, and evidence behavior.

## Scope

Rewrite the first-release Foundation, Product delivery, Customer support, and Automation task labels. No journey logic, evidence rules, permissions, routes, or completion semantics change.

## Verification

- Add a catalog-level regression test that asserts the exact approved label for every task key above.
- Add a frontend regression check that exposes and renders only the task label, never the legacy description.
- Verify task labels wrap without truncation and remain accessible at narrow widths.
- Run focused backend/frontend tests, TypeScript, and production builds.
