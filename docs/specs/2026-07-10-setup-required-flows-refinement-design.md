# Setup Required Flows Refinement

**Date:** 2026-07-10
**Status:** Approved for direct implementation

## Goal

Make Setup guide customers through a focused, achievable configuration path and require the small set of automation flows that turn each selected workflow into repeatable value. A required flow task completes when an eligible template installation is enabled; setup does not wait for an external event or scheduled execution.

## Non-negotiable constraint

The Customer Support journey is frozen. Do not change its task keys, order, copy, core classification, prerequisites, completion evidence, or action destinations. Cross-cutting rendering and access fixes may continue to apply to the page, but Support's product contract remains unchanged.

## Visual and interaction direction

- **Visual thesis:** a restrained operational guide with one dominant next action and calm, compact progress.
- **Content plan:** core progress, recommended next step, selected journeys, featured Automation, and a compact goal editor.
- **Interaction thesis:** preserve independent journey expansion, open typed destinations, and let goal edits update the guide without a page reload.
- Do not add a dashboard-card grid, a separate automation promo section, or duplicate install/run rows.

## Journey visibility and progress

- Render Workspace essentials first.
- Render only selected journeys in saved order.
- Render Automation last as a featured journey when it is not selected.
- Do not render every unselected catalog journey.
- Keep core completion as the finite setup denominator, but label it explicitly as core progress so incomplete value-extension tasks cannot make a `100%` display misleading.
- Once core work is complete, recommendations may continue into value-extension tasks under a "Next value step" presentation.

## Required flow tasks

Each row below is a core requirement. Completion requires an enabled `automation_rules` row installed from one of the named template keys. A successful trigger execution is not required for setup completion.

| Journey | Task key | Eligible template keys | Prerequisite |
|---|---|---|---|
| Plan and ship team projects | `product.required_flow_enabled` | `release_notes_writer`, `stale_task_escalation`, `advance_on_approval` | Planned and assigned project work |
| Publish help center docs | `help_center.required_flow_enabled` | `public_help_freshness_sweep` | Published help-center site |
| Build internal knowledge | `internal_docs.required_flow_enabled` | `docs_freshness_sweep` | Published internal content with ownership/review date |
| Build a sales pipeline | `crm.required_flow_enabled` | `buying_signal_to_task` | Actionable deal and connected CRM email |

The Setup action opens Automation Flows with the relevant recommended template preselected. Project setup offers the three eligible templates as one alternative requirement; users do not receive three checklist rows.

The Automation journey treats an enabled flow as the required automation-setup milestone. Triggered success and multi-day reliability remain visible value milestones but no longer block core setup. Custom-agent success is ordered after prebuilt flow adoption.

## Other corrections

### Actions and access

- Route company context to Knowledge settings and anchor the company-context editor.
- Match Help Center publish/config/widget permissions to destination APIs.
- Match CRM pipeline settings to `crm.admin`.
- Require the Automation module for agent actions.
- Allow Docs and CRM editors to qualify for applicable agent actions alongside PM and Support editors.
- Do not require `agent_scheduling` for event-triggered flow setup.
- Add template-aware Setup URLs for required flow actions.

### Onboarding and goal management

- Replace the duplicate Product engineering and Team/project management onboarding choices with one `Plan and ship team projects` choice.
- Keep the maximum of three canonical goals.
- Add a compact goal editor to Setup using the existing update-goals API.

### Evidence and recommendation quality

- Do not count template-created agents as custom-agent success.
- Do not use the automatically seeded CRM pipeline as a user-completed setup task; remove that task from the visible CRM journey and prerequisite denominator.
- Require CRM deal readiness to include a real contact association; company remains a separate useful context requirement.
- Keep recommendations deterministic and in displayed task order, but name the missing prerequisite in blocked copy when possible.
- Invalidate/refetch Setup on window focus and after goal updates so returning from a destination reflects completed work promptly.

## Flow entitlement behavior

Required flow tasks remain required even when the workspace lacks the Automation Flows entitlement. The task is blocked with the normal plan explanation and the destination uses the existing upgrade dialog. Setup does not silently downgrade the task to optional.

## Verification

- Catalog tests assert exact journey visibility, task order, core membership, and frozen Support rows.
- Repository tests cover each template-key alternative and exclude disabled installations.
- Access tests cover Docs, CRM, Automation, and event-flow entitlement rules.
- Frontend tests cover the context route, template-aware flow routes, core-progress language, goal editing, and selected-plus-Automation rendering.
- Focused Go and frontend tests, Go build, and frontend production build must pass before completion.
