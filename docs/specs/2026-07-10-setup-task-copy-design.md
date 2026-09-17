# Setup task copy and journey behavior

**Date:** 2026-07-10
**Status:** Approved direction


This design records the approved setup-checklist wording and journey behavior.
Use the implementation notes to understand what a completed step actually proves.

## Current implementation and limits

Source-compared on 2026-09-18; no browser or application tests were run.

- The [setup catalog](../../server/internal/service/setup_catalog.go) contains
  the plain-language task titles and the nine support milestones. It still
  stores some descriptions internally; the requirement is about presentation,
  not deleting every backend description field.
- [SetupSuccessPage](../../frontend/src/pages/SetupSuccessPage.tsx) renders the
  task title with wrapping, independent keyed journey toggles, semantic header
  buttons, `aria-expanded`/`aria-controls`, and mounted task lists using `hidden`.
  Its state is component-local, initialized on a nonempty journey payload and
  reset on workspace change. Existing keys survive nonempty refreshes and an
  empty same-workspace refresh does not itself clear the toggle map.
- [Support evidence queries](../../server/internal/repository/setup.go) check
  active email routes, widget installations joined to workspace sessions,
  published public articles, ready/indexed brand sources, active mailboxes, and
  configured routing rules/prompts. A session proves a recorded session exists;
  it does not independently prove a production website installation outside a
  preview/setup environment.
- The AI-support completion predicate checks `ai_enabled` and a nonempty
  `ai_agent_id` in active installation settings. It does not itself prove that
  the agent still exists, is operational, has usable credentials, or produces
  accurate answers. Likewise, indexed-source and configured-routing predicates
  are setup evidence, not quality or delivery guarantees.
- Current navigation is centralized in
  [setupActions](../../frontend/src/lib/setupActions.ts). Use that mapping when
  changing destinations rather than introducing another route table in UI code.

The original regression matrix remains useful acceptance guidance. Exact labels,
all maturity/prerequisite combinations, assistive-technology behavior, and narrow
viewport rendering require their respective tests; this source review does not
report them as freshly passed.

## Original design

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
| `support.brand_knowledge_ready` | Add and sync a brand knowledge source so the AI support agent gives accurate, on-brand answers. |
| `support.ai_agent_activated` | Activate the AI support agent so common customer questions can be answered automatically. |
| `support.team_inbox_created` | Create a team inbox so conversations have clear ownership. |
| `support.routing_enabled` | Turn on automatic routing so every conversation reaches the right team. |
| `support.pm_task_linked` | Create or link a task from a customer issue so feedback becomes product work. |
| `support.coverage_fix_applied` | Review a coverage gap and apply a fix so the AI can answer more customer questions. |
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

### Collapsible journey sections

- Render every journey as an independently collapsible section.
- After the first successful non-empty journey payload for a workspace, expand only the first journey containing a visible task with status `available` or `needs_attention`.
- Scan all rendered journeys in API order, including the final featured Automation journey. An actionable featured journey opens ahead of blocked work in an earlier selected journey because actionable work has higher priority than a blocker explanation.
- Skip journeys whose visible tasks are all `completed` when selecting the initial section.
- If no actionable task exists, expand the first journey containing an incomplete `blocked` task so the customer can see what prevents progress.
- If every visible task in every journey is complete, initialize every journey as collapsed; the page-level completion message remains the primary state.
- `unavailable` and `unable_to_verify` do not participate in actionable or blocked fallback selection. If they are the only incomplete statuses, initialize every journey as collapsed because opening the section cannot offer a next action.
- Loading and empty payloads do not initialize expansion state. After initialization, a loading or empty same-workspace refresh also does not clear or mutate the keyed toggle map. When a later non-empty payload returns, existing keys recover their prior toggles and genuinely new keys start collapsed.
- Do not persist expansion state between visits.
- Allow multiple journeys to remain expanded at the same time; this is not a single-open accordion.
- Keep the complete journey header visible while collapsed: accent indicator, maturity or `Power up` label, journey title, journey description, verified completion count, and expand/collapse chevron.
- Make the full header a semantic button with `aria-expanded` and `aria-controls`; keep a stable task-list container with the matching `id` mounted and set its native `hidden` attribute while collapsed. This removes task rows from the layout and accessibility tree without leaving `aria-controls` pointed at a missing element.
- Rotate the chevron when expanded and respect reduced-motion preferences.
- Do not render collapsed task rows in the page layout or accessibility tree.
- On later successful refreshes in the same workspace, preserve keyed toggles for existing journey keys regardless of order and initialize only newly seen keys as collapsed.
- When the workspace ID changes, clear the prior workspace’s expansion state and reinitialize from the first successful non-empty payload for the new workspace. A full component remount/page visit follows the same next-work-only initialization.

## Customer support journey behavior

The support journey measures durable support-system configuration rather than routine conversation volume. Remove test-resolution, first-resolution, repeat-resolution, and one-off AI-reply milestones.

Use this order and completion evidence:

| Order | Task | Durable completion evidence |
|---|---|---|
| 1 | Email inbox | At least one active support email route. |
| 2 | Live chat | An active widget installation has received at least one widget session, proving the widget was used outside its setup screen. |
| 3 | Public help docs | At least one public help-center article is published in the workspace. |
| 4 | Brand knowledge | At least one `support_content_sources` row in the workspace has `sync_status = 'ready'` and indexed content (`indexed_pages > 0` or `indexed_chunks > 0`). Docs-backed agent knowledge does not satisfy this task, keeping it distinct from public help docs. |
| 5 | AI support agent | An active widget installation has JSON settings with `ai_enabled = true` and a non-empty `ai_agent_id`. |
| 6 | Team inbox | At least one active `support_mailboxes` row exists. The shared inbox is a synthetic UI view, not a row, and therefore cannot satisfy this task. |
| 7 | Automatic routing | An active widget installation has `triage_enabled = true`, an active team inbox exists, and either (a) an active `support_triage_rules` row targets an active inbox or (b) an active triage-eligible inbox has a non-empty `routing_prompt` or `description` for AI routing. Assignment mode alone does not satisfy routing. |
| 8 | Customer issue task | A support conversation is linked to a product task. |
| 9 | Coverage gap | At least one coverage recommendation has been applied. |

The first seven tasks are the core setup denominator. The task-linking and coverage-gap steps are advanced value milestones.

Maturity is conjunctive and never skips an earlier threshold:

- `preparing`: email inbox or live chat is incomplete;
- `ready`: email inbox and live chat are complete, but at least one of public help docs, brand knowledge, or AI support agent is incomplete;
- `activated`: tasks 1–5 are complete, but team inbox or automatic routing is incomplete;
- `established`: tasks 1–7 are complete, but task linking or coverage improvement is incomplete;
- `advanced`: all nine tasks are complete.

Display order does not force unrelated setup work into a linear lock. Exact prerequisites are:

- `support.ai_agent_activated` requires both `support.help_docs_ready` and `support.brand_knowledge_ready`;
- `support.routing_enabled` requires `support.team_inbox_created`;
- `support.coverage_fix_applied` requires `support.ai_agent_activated`;
- all other support tasks have no prerequisite. In particular, task linking has no single channel prerequisite because either email or live chat can supply the customer issue.

Exact action mappings are:

| Task | Action key | Workspace-relative route |
|---|---|---|
| `support.email_inbox_connected` | `support_email_inbox` | `/settings/inboxes-routing?tab=email` |
| `support.live_chat_installed` | `support_live_chat` | `/settings/chat-general` |
| `support.help_docs_ready` | `support_help_docs` | `/docs` |
| `support.brand_knowledge_ready` | `support_brand_knowledge` | `/settings/knowledge` |
| `support.ai_agent_activated` | `support_ai` | `/settings/support-ai-assistant` |
| `support.team_inbox_created` | `support_team_inboxes` | `/settings/inboxes-routing?tab=inboxes` |
| `support.routing_enabled` | `support_routing` | `/settings/inboxes-routing?tab=routing` |
| `support.pm_task_linked` | `support_inbox` | `/support` |
| `support.coverage_fix_applied` | `support_coverage` | `/support/coverage` |

## Scope

Rewrite the first-release Foundation, Product delivery, Customer support, and Automation task labels. For Customer support, replace the old conversation-resolution journey with the nine approved setup and value milestones above, including their evidence, dependencies, actions, maturity, and progress semantics. Permissions and module entitlement behavior remain unchanged.

## Verification

- Add a catalog-level regression test that asserts the exact approved label for every task key above.
- Add a frontend regression check that exposes and renders only the task label, never the legacy description.
- Verify task labels wrap without truncation and remain accessible at narrow widths.
- Assert the exact nine-key support order and the absence of the removed resolution and one-off AI-reply keys.
- Test every support evidence predicate with workspace scoping, active-state requirements, public publication, successful knowledge indexing, non-empty agent selection, synthetic shared-inbox exclusion, and routing-target validity.
- Test the support prerequisite graph, seven-task core denominator, each conjunctive maturity threshold, and every action-key route.
- Add frontend interaction tests proving that the first actionable journey opens; completed journeys are skipped; `needs_attention` counts as actionable; actionable featured Automation outranks earlier blocked selected work; a blocked journey is the fallback; unavailable/unable-to-verify-only and all-complete payloads start fully collapsed; omitted `support.help_docs_ready` and no-task journeys do not participate in selection; sections toggle independently; multiple sections can remain open; reordered existing keys retain their state; newly added keys start collapsed; an empty-refresh/non-empty round trip preserves keyed state; changing workspace or remounting recalculates the next-work default; collapsed task rows are absent from layout and the accessibility tree while header progress remains visible; trigger `aria-expanded`/`aria-controls` values match a stable controlled container; and the chevron uses reduced-motion-safe transition classes.
- Run focused backend/frontend tests, TypeScript, and production builds.
