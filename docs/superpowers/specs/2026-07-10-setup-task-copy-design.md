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
| `support.email_inbox_connected` | Connect your support email inbox so customer emails arrive in Helpin. |
| `support.live_chat_installed` | Add Helpin live chat to your website so customers can contact you instantly. |
| `support.help_docs_ready` | Import or create public help docs so customers can find answers themselves. |
| `support.brand_knowledge_ready` | Add brand knowledge sources so the AI support agent gives accurate, on-brand answers. |
| `support.ai_agent_activated` | Activate the AI support agent so common customer questions can be answered automatically. |
| `support.team_inbox_created` | Create team inboxes so conversations have clear ownership. |
| `support.routing_enabled` | Turn on automatic routing so every conversation reaches the right team. |
| `support.pm_task_linked` | Create a task from a customer issue so feedback becomes product work. |
| `support.coverage_gap_resolved` | Resolve coverage gaps so the AI answers more questions and your resolution rate improves. |
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

## Customer support journey behavior

The support journey measures durable support-system configuration rather than routine conversation volume. Remove test-resolution, first-resolution, repeat-resolution, and one-off AI-reply milestones.

Use this order and completion evidence:

| Order | Task | Durable completion evidence |
|---|---|---|
| 1 | Email inbox | At least one active support email route. |
| 2 | Live chat | An active widget installation has received at least one widget session, proving the widget was used outside its setup screen. |
| 3 | Public help docs | At least one public help-center article is published in the workspace. |
| 4 | Brand knowledge | At least one support or agent knowledge source exists in the workspace. |
| 5 | AI support agent | Widget settings have AI enabled with a selected support agent. |
| 6 | Team inbox | At least one active support mailbox exists. |
| 7 | Automatic routing | Automated routing is enabled and at least one active routing rule or inbox AI-routing prompt targets an active team inbox. |
| 8 | Customer issue task | A support conversation is linked to a product task. |
| 9 | Coverage gap | At least one coverage recommendation has been applied. |

The first seven tasks are the core setup denominator. The task-linking and coverage-gap steps are advanced value milestones. Support activation is reached when the AI support agent is active; established maturity requires team inboxes and automatic routing; advanced maturity additionally requires the task-linking and coverage-gap outcomes.

Actions deep-link to the relevant product surfaces: email forwarding, chat widget, Docs, knowledge settings, AI Assistant, team inboxes, routing, support inbox, and coverage.

## Scope

Rewrite the first-release Foundation, Product delivery, Customer support, and Automation task labels. For Customer support, replace the old conversation-resolution journey with the nine approved setup and value milestones above, including their evidence, dependencies, actions, maturity, and progress semantics. Permissions and module entitlement behavior remain unchanged.

## Verification

- Add a catalog-level regression test that asserts the exact approved label for every task key above.
- Add a frontend regression check that exposes and renders only the task label, never the legacy description.
- Verify task labels wrap without truncation and remain accessible at narrow widths.
- Run focused backend/frontend tests, TypeScript, and production builds.
