# Automation Flows Redesign Implementation Plan

**Date:** 2026-08-10
**Status:** Implemented
**Design source:** `/root/automation_redesign/design_handoff_automation/Automation Flows.dc.html` and its `README.md`

## Goal

Rebuild the Automation > Flows page as the high-fidelity, operational list in the redesign: status-first rows, composable filters, a right-side detail drawer, and a compact template gallery. Preserve every production capability already present in Helpin, including template installation, custom-flow creation, editing, enable/disable, run history, scheduled run-now, template-aware deletion, permissions, billing upgrade handling, and existing query-string entry points.

The redesign is a presentation and read-model change. The existing flow composer, template install form, automation rule execution, and template manifests remain the source of truth.

Implementation note: the presentation helpers, flat list, and detail drawer were kept in the existing `AutomationFlows.tsx` module to preserve its established orchestration and avoid a high-risk file split while another cleanup was in progress. The planned behavior and data contract are implemented; extraction can be done as a separate mechanical refactor.

## Architecture

Keep `AutomationFlowsPage` as the orchestration layer for queries, mutations, composer state, template installation, and permission checks. Move the list-specific presentation logic into small flow-page modules:

- a pure presentation model for status, chips, counts, search, scope, dates, and incomplete reasons;
- a flat responsive list/row component;
- an accessible Radix `Sheet` detail drawer that loads recent runs only when opened;
- page-scoped visual tokens for the handoff palette, spacing, and severity states.

The list continues to combine the workspace-scoped rule collection with Automation Overview health data. The backend read model gains only the missing aggregates needed by the final design: lifetime failed-run count and exact next scheduled time. No new database table or migration is required.

## Decisions and fidelity boundaries

1. **Use Helpin's existing workspace header for `Automation / Flows`.** It already owns the sticky breadcrumb and sidebar trigger. Do not render a second breadcrumb inside the page. Its 56px height is the only intentional difference from the handoff's standalone 52px bar.
2. **Keep the existing Inter UI face and `font-mono` stack.** The handoff explicitly permits the product's existing UI and mono fonts. Match the supplied size, weight, tracking, and numeric treatment without adding page-only webfonts.
3. **Match the handoff exactly in light mode; preserve dark mode semantically.** Scope the light palette to the flows page. Add dark token overrides using Helpin's existing semantic colors rather than forcing a light island inside a dark workspace.
4. **`Needs attention` includes `error`, `incomplete`, and Helpin's existing `needs_review` state.** The prototype only has error and incomplete, but dropping `needs_review` would hide a production state that can require action.
5. **Scope meanings are explicit:**
   - `Workspace-wide` is the default and shows all flows the current actor can see.
   - `My team` shows flows whose `team_id` is in the actor's team memberships.
   - `Created by me` shows flows whose `created_by` equals the signed-in user.
   - Legacy flows with a null `created_by` remain visible in Workspace-wide/My team but cannot truthfully appear under Created by me. Do not invent a backfill owner.
6. **Preserve template discovery.** Restyle the gallery to the handoff's 720px, two-column layout and dashed custom-flow action, but retain the production search and category filters because the real manifest catalogue is larger than the six-item prototype.
7. **Preserve the educational first-run state.** A genuinely empty workspace keeps a restrained New flow/Browse templates teaching state. A non-empty collection with filters producing zero rows uses the handoff's single-line `No flows match this view.` state.
8. **Keep all existing deep links.** `template`, `trigger_type`, `workflow`, `team`, `show_trigger`, `show_rule`, `agent_id`, and repository/target prefill parameters must continue to work. A `show_rule` link should select and open that flow's drawer; contextual filtering can remain until the user clears it.
9. **No new detail route.** Row clicks open the drawer in place. `View run history` and individual recent runs continue to link to Automation Activity.

## Capability parity

| Existing capability | Redesign behavior |
|---|---|
| New flow | Opens the redesigned template gallery; custom flow opens the existing composer. |
| Template deep link | If `template` matches a manifest key, open that template's install form. Legacy trigger-prefill behavior remains under `trigger_type`. |
| Template install | Keep the generated input/review form, agent overrides, additional instructions, and install mutation unchanged. |
| Edit | Opens the existing composer populated from the selected rule. |
| Enable/disable | Available in row menu and drawer, permission-gated, with optimistic list state and final query reconciliation. |
| Run now | Shown for scheduled flows; enabled only for supported agent flows that are complete and not already queued/running. Billing errors open `UpgradeRequiredDialog`. |
| View runs | Available from row menu and recent-run entries; keeps the existing Activity filters and hashes. |
| Delete | Uses the existing confirmation, including template-created-agent keep/delete logic and shared-agent protection. |
| Permissions | New/edit/toggle/delete require `canAdminAutomations`; run-now requires `canEdit`; read-only users can open drawers and run history. |
| Billing | Create/install/toggle/run paths retain `getUpgradeRequiredReason`; no raw entitlement or usage errors are shown. |
| External filters | Existing `show_rule`, `show_trigger`, `agent_id`, workflow, and team links compose with the new status/search/scope controls. |
| Empty/loading/error | Purpose-built list skeleton, retryable rule-load error, degraded health metrics if Overview fails, educational first-run state, and single-line filtered empty state. |

## Filter and status model

Build a single pure projection so tab counts and visible rows cannot disagree:

1. Start with workspace rules.
2. Apply external context filters from the route (`show_rule`, `show_trigger`, `agent_id`, `workflow`, and `team`).
3. Apply the scope select.
4. Apply case-insensitive search across flow name, description, trigger kind/value, action kind/value, agent, and resolved target.
5. Compute tab counts from that shared base set.
6. Apply the selected status tab to produce rows.

Tabs are `All`, `Active`, `Paused`, and `Needs attention`. Status precedence stays deterministic:

1. incomplete configuration;
2. latest execution failed;
3. pending review/approval signal;
4. paused/disabled;
5. active.

Use a 7px dot plus quiet status text. Error and attention rows receive only the handoff's 2px inset severity edge. A small tinted badge is reserved for a concrete setup problem such as `Missing agent` or `Missing target`; status itself is never a pill.

## Data contract

### Existing data to reuse

- `GET /api/automation/flows` supplies rules, scope, creator, trigger/action configuration, and template metadata.
- `GET /api/automation/overview` supplies last execution, last run, current status, total runs, and last error.
- `GET /api/automation/activity?source=automation_rule&reference_id=<id>&per_page=4` supplies the selected drawer's recent runs.
- Existing agents, workflows, teams, tasks, epics, repositories, and workspace settings resolve human labels and composer inputs.

### Read-model additions

- Add `error_runs` to each automation-rule inventory item's health metrics. Count trigger executions whose status is `failed`.
- Add `next_run_at` in RFC3339 for enabled, valid cron rules by parsing the stored UTC five-field expression with the existing `robfig/cron` dependency. Event flows omit it; paused or incomplete presentation overrides it with `Paused` or `Setup incomplete`.
- Populate `AutomationRule.CreatedBy` for all new manual and legacy automation-rule create paths using the authenticated actor. Manifest template installs already receive an actor and must keep doing so.
- Keep historical null creators unchanged.

The frontend should read these metrics through one typed local adapter rather than spreading string-key lookups throughout row and drawer components. Missing metrics render as an em dash and do not block the rule list.

## Implementation tasks

### Task 1: Complete the backend flow read model

**Files:**

- Modify: `server/internal/handler/automation.go`
- Modify: `server/internal/handler/automation_rule.go`
- Modify: `server/internal/service/automation_rule_engine.go`
- Modify as needed for legacy template callers: `server/internal/service/agent_templates.go`
- Modify: `server/internal/repository/agent.go`
- Modify: `server/internal/service/automation_inventory.go`
- Modify: `server/internal/automationcron/cron.go`
- Test: `server/internal/service/automation_rule_engine_test.go`
- Test: `server/internal/service/automation_inventory_test.go`
- Test: repository/cron tests beside the modified packages

- [ ] Thread the authenticated actor through rule creation and set `CreatedBy` without accepting a client-supplied creator.
- [ ] Replace or extend the rule execution aggregate query so it returns total and failed counts per rule in one grouped query.
- [ ] Add a tested cron `Next` helper and expose `next_run_at` only for valid scheduled rules.
- [ ] Add `error_runs` and `next_run_at` to inventory health metrics while preserving all existing metric keys.
- [ ] Cover completed/failed/cancelled mixes, no-run flows, paused flows, custom valid schedules, invalid schedules, and actor attribution.

### Task 2: Isolate and test the frontend presentation model

**Files:**

- Add: `frontend/src/pages/automation/flows/flowPresentation.ts`
- Add: `frontend/src/pages/automation/flows/__tests__/flowPresentation.test.ts`
- Modify: `frontend/src/pages/automation/AutomationFlows.tsx`

- [ ] Move/normalize status, incomplete-reason, trigger, action, target, last-run, next-run, run-count, and Activity-link derivation into pure functions.
- [ ] Add the shared filter pipeline and tab count derivation described above.
- [ ] Resolve scope using team memberships and the authenticated user ID.
- [ ] Ensure search covers the rendered trigger/action/target wording, not raw JSON alone.
- [ ] Preserve `needs_review` and all route-context filters in tests.
- [ ] Keep compatibility exports temporarily if existing tests import helpers from `AutomationFlows.tsx`, then update tests to the new module in the same change.

### Task 3: Rebuild the page shell, toolbar, and flat list

**Files:**

- Add: `frontend/src/pages/automation/flows/FlowList.tsx`
- Add: `frontend/src/pages/automation/flows/automationFlows.css`
- Modify: `frontend/src/pages/automation/AutomationFlows.tsx`
- Modify: `frontend/src/routes/_authenticated/w/$slug/automation/flows.tsx`
- Test: `frontend/src/pages/automation/__tests__/AutomationFlows.test.tsx`

- [ ] Remove route-level double padding and let the page own the 1080px container, 44px top spacing, and responsive gutters.
- [ ] Build the 27px `Flows` header, final description, and permission-gated New flow action.
- [ ] Build status tabs with mono counts, a labelled search field, the scope select, and a removable contextual-filter indicator when entered from another surface.
- [ ] Replace card rows with the handoff grid: identity/logic, total runs, last run, next run, and overflow menu.
- [ ] Use real Helpin icons for plus, arrow, menu, close, and search; keep semantic text available to assistive technology.
- [ ] Make the row keyboard-openable while stopping menu/link events from opening the drawer.
- [ ] Keep only one menu open through Radix menu behavior and close it when filter state changes.
- [ ] Add list-shaped skeletons, retryable error UI, health-metric degradation, and the two distinct empty states.
- [ ] Add the template footer below the list, hidden or disabled consistently for actors who cannot create flows.

### Task 4: Add the flow detail drawer

**Files:**

- Add: `frontend/src/pages/automation/flows/FlowDetailDrawer.tsx`
- Modify: `frontend/src/pages/automation/AutomationFlows.tsx`
- Test: `frontend/src/pages/automation/__tests__/AutomationFlows.test.tsx`

- [ ] Use the existing Radix `Sheet` for focus trapping, Escape, scrim click, and focus restoration; override it to 460px desktop width, full-width mobile, and no more than 150ms motion.
- [ ] Render status/name header, description, Logic card (`When`, `Then`, `Using`, `On`), 2x2 health grid, and recent runs.
- [ ] Fetch only four recent executions while a flow is selected; show compact loading, empty, and error states without blocking the rest of the drawer.
- [ ] Link recent executions to the exact Activity run/execution when IDs exist.
- [ ] Mirror all allowed actions in the drawer: Enable/Pause, Edit, scheduled Run now, and Delete.
- [ ] Reuse the existing billing dialog, run blockers, and template-aware delete confirmation.
- [ ] Open the matching drawer for a valid `show_rule` deep link and close it safely if that flow is deleted or filtered out.

### Task 5: Restyle template discovery without regressing installation

**Files:**

- Modify: `frontend/src/pages/automation/AutomationFlows.tsx` (existing `FlowTemplateGallery` and dialog orchestration)
- Test: `frontend/src/pages/automation/__tests__/AutomationFlows.test.tsx`
- Test: `frontend/src/lib/setupActions.test.ts`

- [ ] Restyle the gallery to the 720px handoff modal, two-column template cards, concise trigger/source kind label, and full-width dashed `Build a custom flow` action.
- [ ] Retain compact search/category controls above the grid for the production catalogue.
- [ ] Derive each template card's kind from its trigger source and use the manifest's short description as the sentence.
- [ ] Keep the existing install input/review dialog and custom composer unchanged after selection.
- [ ] Fix/lock direct manifest-key links such as `?template=stale_task_escalation` so they open that template rather than treating the key as a trigger type.
- [ ] Preserve upgrade handling, install result highlighting, created-agent overrides, and back navigation to the gallery.

### Task 6: Responsive, accessibility, and integration pass

**Files:**

- Modify the flow page files above.
- Add focused tests beside the affected components as needed.

- [ ] At widths below roughly 900px, move runs/last/next metadata under the logic sentence and remove the desktop column labels.
- [ ] Make the toolbar wrap cleanly, keep all critical actions available, and make the drawer full-width on small screens.
- [ ] Verify visible focus, accessible names, status text independent of color, screen-reader dialog title/description, and keyboard operation for tabs, row, menu, drawer, and gallery.
- [ ] Respect reduced motion and keep hover/drawer transitions at or below 150ms; add no entrance animation.
- [ ] Verify light-mode fidelity against the handoff and dark-mode contrast/functionality.
- [ ] Confirm real long names, descriptions, repositories, targets, four-digit counts, missing metrics, and long errors do not overflow.

## Verification

Focused checks during implementation:

```bash
cd /root/helpin/server
go test ./internal/automationcron ./internal/repository ./internal/service

cd /root/helpin/frontend
pnpm vitest run src/pages/automation/__tests__/AutomationFlows.test.tsx src/pages/automation/flows
```

Final checks:

```bash
cd /root/helpin/server
go test ./...

cd /root/helpin/frontend
pnpm test
pnpm build

cd /root/helpin
git diff --check
```

Manual/visual acceptance matrix:

- Desktop: 1440px and 1024px, light and dark.
- Responsive: 899px, 768px, and 390px.
- Data states: loading, rule-load error, health-load error, first-run empty, filtered empty, active, paused, incomplete, needs-review, errored, running, scheduled, and never run.
- Permissions: read-only, editor/run-now, and automation admin.
- Interactions: every status/search/scope combination, row click, menu event isolation, drawer close paths, Activity links, toggle, run-now success/blocker/billing failure, edit, ordinary delete, template uninstall with kept/deleted/shared agent, template deep link, and custom-flow path.

## Definition of done

- The light-mode page is pixel-close to the supplied Flows reference inside Helpin's existing workspace shell.
- Every production flow capability in the parity table still works and remains permission/billing safe.
- Counts, rows, drawer stats, and recent-run links all derive from consistent real data.
- The page is usable by keyboard, responsive without hiding critical actions, and functional in dark mode.
- Focused backend/frontend tests, full frontend build, and `git diff --check` pass.
