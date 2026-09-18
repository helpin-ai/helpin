# Command bar UX polish plan

This historical proposal records UX work for the former command-bar interface. Contributors should use it to understand earlier decisions, not as a current backlog: the search and agent surfaces have since diverged.

## Source review — 2026-09-18

- [SearchCommandPalette](../../frontend/src/components/search/SearchCommandPalette.tsx) is now workspace search and navigation, with a 400 ms search debounce and “Search tasks, epics, docs, people…” placeholder. It does not render agent plans, parse requests, or expose the proposed context badge.
- The old `StepToolPicker.tsx`, `PromotionDialog.tsx`, `CommandBarRunRail.tsx`, `commandBarStore.ts`, and `CommandIntentsSettingsPage.tsx` are absent from their proposed paths. Their tasks cannot be applied mechanically to the current UI.
- [AskAgentsDock](../../frontend/src/components/agents/AskAgentsDock.tsx) and [dockStore](../../frontend/src/stores/dockStore.ts) own the present agent surface. The dock still consumes [page context](../../frontend/src/components/command-bar/pageContext.tsx), but its root is a fixed overlay, not evidence that the proposed three-state layout column shipped.
- The original staging claims, screenshots, external dogfooding host, and completion checklist are dated records. Current behavior needs review in the replacement surfaces; this source review does not certify deployment or visual acceptance.

## Original plan

**Date:** 2026-04-28
**Status:** Draft, not started
**Altitude:** UX/UI work over the v1 command-bar surface that already shipped. Frontend-mostly; small backend tweaks called out per item.

---

## Summary

The command bar v1 (Cmd+K palette, plan rendering, tool picker, run rail, promotion dialog, Command Intents settings) is functionally complete and live on staging. A dogfooding pass surfaced UX gaps that erode trust or make the surface harder to use than necessary. This plan organizes those findings into a concrete polish queue, prioritizes the five with the highest impact, and sequences the rest.

The headline issue: **Cmd+K has no visible binding to the current page**. A user opens it on a task and on the workspace dashboard and sees an identical input. The bar's "magic" — auto-resolving page context — is invisible until the parser finishes. That's the credibility hole this plan starts with.

## Goals

- Make the command-bar surface feel **deliberate and trustworthy** at every step (open → type → confirm → run).
- Eliminate the layout/overlap and grammar/copy issues that make the rail and plan card feel rough.
- Lift the tool picker and promotion dialog from "works" to "scannable at 100+ tools, not noisy."
- Establish a consistent status/action vocabulary across all command-bar surfaces.

## Non-goals (this pass)

- New features beyond what v1 promises. No diff review, no @-mention parity, no spawned agents.
- Backend contract changes that aren't strictly required to support a UX fix.
- Replacing per-page agent buttons (still parallel surface).
- Themeing or motion library introductions; small CSS-level easing only.

## Grounding (what already exists)

| Surface | State | Path |
|---|---|---|
| Cmd+K palette | Functional, no context badge, no cost preview | `frontend/src/components/search/SearchCommandPalette.tsx` |
| Plan-step card | Outside cmdk listbox; flat hierarchy | `SearchCommandPalette.tsx` (inline) |
| StepToolPicker | Backend catalog wired; flat list of 100+ rows; descriptions full-length | `frontend/src/components/command-bar/StepToolPicker.tsx` |
| Run rail | Floating fixed-position aside; overlaps page content | `frontend/src/components/command-bar/CommandBarRunRail.tsx` |
| PromotionDialog | Editable tools/targets shipped; auto-name truncates; tool list duplicates picker | `frontend/src/components/command-bar/PromotionDialog.tsx` |
| Command Intents page | Filter pills + redaction toggle + expand/review work; raw JSON in expanded view | `frontend/src/pages/settings/CommandIntentsSettingsPage.tsx` |
| `PageContext` resolver | Already passes a structured context to dispatch | `frontend/src/components/command-bar/pageContext.tsx` |
| Status meta map | Exists but inconsistently used across surfaces | `frontend/src/components/pm/agentRunConstants` |

## Top 5 — ship first

These are sequenced by impact-per-effort. The first two are meaningful refactors; the next three are smaller tweaks that punch above their weight.

### T1. Context badge in Cmd+K

**Problem:** No visible page binding. User trust depends on the bar correctly resolving "this," but they can't see what was resolved until the parser returns.

**Recommendation:**
- Render a pill above the input showing `{icon} {entity_type} · {display_title}` (or `📁 Workspace · {name}` on fallback).
- Pill is clickable: clicking opens the entity's detail route; long-press / × strips back to workspace fallback.
- Use the existing icon vocabulary (RecordIcon for tasks, BookOpen01 for epics, File01 for documents, Briefcase for CRM).
- When `PageContext.entityType === 'workspace'`, style the pill with `outline` variant so the absence of a specific entity is intentional, not invisible.
- Update placeholder copy to `Search or ask agents...` (today says only `Search Usermaven...`).

**Files:** `SearchCommandPalette.tsx`, optionally a small `<PageContextBadge>` extracted under `components/command-bar/`.

### T2. Run rail as a real docked column

**Problem:** Today the rail is `fixed right-4 top-20` and overlaps any main-content area on the right (proven on the Command Intents page screenshot). Resize/scroll behavior is brittle.

**Recommendation:**
- Replace fixed positioning with a layout-aware right column controlled by a state in `commandBarStore`. Three states: `closed` (hidden), `peek` (40px strip with active count), `open` (320–400px column that resizes the main grid).
- Auto-peek when there's any active run; auto-close after 30s of idle when no runs are tracked.
- Empty state inside the open column: `Recent runs appear here. Try ⌘K to ask an agent.` with a Cmd+K hint chip.
- Keyboard shortcut: `⌘.` toggles the rail.

**Files:** `CommandBarRunRail.tsx`, `commandBarStore.ts`, parent layout `routes/_authenticated/w/$slug.tsx`.

### T3. Tool picker grouping + allowed/not-available split

**Problem:** Picker shows ~100 rows with most disabled. Scanning for the right tool is painful.

**Recommendation:**
- Use the catalog's `Category` field to render collapsible groups. Default expanded; remember collapsed state in `localStorage` keyed by `agentId`.
- Two top-level sections: `Available (N)` — expanded — and `Not available (M)` — collapsed by default. Disabled rows still searchable, just not in the way of the happy path.
- Description: line-clamp to 1 by default; click row body (not checkbox) to expand description.
- Filter input: add count footer `Showing X of Y`. `Esc` while focused clears filter; if filter is non-empty, the trigger button shows a `× clear` affordance inline.
- Promote `Reset to all` from the popover footer to inline next to the trigger when narrowed: trigger reads `Tools: 14 of 16 ↻`.

**Files:** `StepToolPicker.tsx`. No backend change.

### T4. Plan-card hierarchy flip

**Problem:** Today the step row leads with `Step 1 · Forge · workspace`, instruction in muted text. Users approve *actions*, not agents — the headline should be the action.

**Recommendation:**
- Swap typographic weight: instruction in `text-sm font-medium`, agent name + target on a single muted meta line below. Step number stays as a small left chip.
- For multi-step plans, add a faint vertical connector down the left gutter and step numbers as filled circles. Makes sequence legible at a glance.
- Add a `Tools narrowed (-2)` chip on the row header when the user has narrowed tools — visible without opening the popover.

**Files:** `SearchCommandPalette.tsx` (plan-card subtree).

### T5. Promotion auto-name fix

**Problem:** Auto-suggested name `Forge — Create An Agent And` is the truncated 4-word slice with trailing junk. Visibly broken.

**Recommendation (pick one):**
- **Option A (cheap):** drop auto-suggest entirely. Use a smart placeholder `e.g., Doc Refresher` and let the user type. Validation already requires ≥2 chars.
- **Option B (richer):** call the existing LLM provider for a 3–5 word title from `prompt + agent_name`. Cache the suggestion per `runId` so the user can edit without losing it. Backend addition: small `POST /command-bar/runs/{runID}/suggest-name` endpoint.

**Recommendation:** Ship A now, file B as a follow-up if users keep entering generic names.

**Files:** `PromotionDialog.tsx` (Option A); plus handler/service stub if Option B.

## Polish queue (after top 5)

Grouped by surface. Each item is small enough for a single PR.

### Cmd+K palette

- **P1.** Drop the redundant `Ask agents: <query>` row when a plan is already rendered for the same query. Keep it only when the parser hasn't run yet.
- **P2.** Auto-trigger parse after 600ms of typing (debounced), matching search's cadence. User can still hit ⌘↵ to parse immediately.
- **P3.** Add `~ N runs · sequential` micro-meta to the Confirm action so the user sees fan-out before commit.
- **P4.** Footer keyboard hints become contextual: when a plan is rendered, show `⌘↵ Confirm  ⌥T Edit tools  Esc Discard`.

### Plan card

- **P5.** Rationale text moves from inline (`PLAN · Matched request keywords...`) to a single muted line under the Plan header, or as a tooltip on the header.
- **P6.** Multi-step rationale per step (today the rationale is global). Per-step rationale is a backend addition; defer until parser supports it.

### StepToolPicker

- **P7.** Search filter expands to include `category` and `disabled_reason` (already done in linter pass — verify).
- **P8.** Tool description supports markdown links so descriptions can point at tool docs.
- **P9.** Bulk actions inside a category: "Enable all in Filesystem" / "Disable all in Filesystem".

### Run rail

- **P10.** Plan grammar fix: `1 sequential steps · completed` → `1 step · completed`. `2 sequential steps · failed` → `2 steps · step 1 failed`.
- **P11.** Single-step plans collapse to a one-line entry (no nested step card).
- **P12.** "View all runs" link at the bottom that routes to the existing run history page so the rail is not the only entry point.
- **P13.** Plan/run timestamps as relative (`2m ago`) with full timestamp on hover.
- **P14.** Run actions row: change `Open` to `Open run` for clarity; add a tooltip on the icon-only ghost buttons.

### PromotionDialog

- **P15.** Filter target pills to the supported set (`task`, `epic`, `document`, `crm_contact`, `crm_deal`, `workspace`). Today `repository` and other source-agent targets that aren't command-bar-supported leak through.
- **P16.** Tool list shows only the source agent's allowlist (no disabled rows). Disabled rows belong in the step picker for transparency; they're noise in the dialog.
- **P17.** Promote provenance card to the top of the dialog body as a single dense line (`Forge → Workspace · Usermaven · "create an agent"`). Form fields below.
- **P18.** Button label `Save agent` → `Save reusable agent` to disambiguate from the generic "save my changes" pattern elsewhere.

### Command Intents page

- **P19.** Tab counts: `Open (5) · Accepted (2) · Deferred · Rejected · All`. Backend already returns enough data; counts can be derived client-side from the All response or via a lightweight separate count endpoint if scale demands.
- **P20.** Page-context chip becomes a link to the entity (task/epic/doc/contact/deal). Today it's static.
- **P21.** Expanded view: render `page_context` as structured rows (`Type / ID / Title / Related`) by default. Raw JSON behind a `Show raw` disclosure.
- **P22.** Bulk-select rows for triage actions (Defer/Reject in batch). With 50+ open intents, single-click doesn't scale.
- **P23.** "Show full prompts" toggle gets a `(audit-logged)` caption + a subtle warning chip on first activation per session.
- **P24.** Add a secondary sort by reason similarity so admins can spot clusters of unmet intents pointing at the same missing capability.

## Cross-cutting design system

These need to land alongside the polish queue, not after, otherwise inconsistencies multiply.

### X1. Status vocabulary unification

A single `STATUS_META` map drives badges, copy, color tokens for every command-bar surface. Today the same status renders as `Active`, `Completed`, `queued next`, `Waiting`, etc. across rail/plan/dialog.

**Action:** audit existing `STATUS_META` in `pm/agentRunConstants`, extend with command-bar-specific variants, replace ad-hoc strings.

### X2. Action-first language

Throughout the surfaces, lead with the action (verb), demote agent name (proper noun) to metadata. Today the headlines say `Forge` first.

**Action:** copy pass + `font-medium` swap in plan card, rail step row, promotion dialog.

### X3. Empty states as onboarding

Every empty state is an opportunity. Examples:
- Empty rail → "Recent runs appear here. Try ⌘K to ask an agent."
- Empty Command Intents → "When the command bar can't match a prompt, it lands here for review."
- Empty plan parse → "No agent matched. Tell us what you wanted to do."

**Action:** sweep all empty states in command-bar surfaces, replace `null` returns / "No results" with one-line onboarding copy.

### X4. Motion language (light)

Plan steps appearing, tool picker opening, dialog opening — all currently snap. Even 100ms ease-out + slight slide-in makes the surface feel purposeful.

**Action:** apply existing Radix `data-state` animations consistently. Two transitions: `slide-in-from-top-2 duration-100` on plan cards, `slide-in-from-right-2 duration-150` on the docked rail.

## Sequenced bets

- [ ] **T1.** Context badge in Cmd+K
- [ ] **T2.** Run rail as a docked column
- [ ] **T3.** Tool picker grouping + allowed/not-available split
- [ ] **T4.** Plan-card hierarchy flip
- [ ] **T5.** Promotion auto-name fix (Option A: drop auto-suggest)
- [ ] **X1.** Status vocabulary unification (lands alongside top 5)
- [ ] **X2.** Action-first language sweep (lands alongside top 5)
- [ ] **P1–P4.** Cmd+K polish queue
- [ ] **P5–P6.** Plan card polish
- [ ] **P7–P9.** Tool picker polish
- [ ] **P10–P14.** Run rail polish
- [ ] **P15–P18.** Promotion dialog polish
- [ ] **P19–P24.** Command Intents polish
- [ ] **X3.** Empty-state onboarding pass
- [ ] **X4.** Motion language pass

## Open questions

- **Context badge dismissal.** Should clicking × on the pill strip context to workspace fallback, or close the bar? (Recommendation: strip to workspace fallback; closing should be Esc only.)
- **Rail docking on small viewports.** Below 1024px, does the docked rail collapse to a sheet/drawer, or stay floating? (Recommendation: drawer at <1024px, docked column otherwise.)
- **Auto-name LLM call (T5 Option B).** Worth the round-trip and cost, or is the smart-placeholder UX enough? Decision deferred until v1 data shows users renaming most promotions anyway.
- **Tab counts on Command Intents (P19).** Compute client-side from a single fetch, or add `GET /unmet-intents/counts`? (Recommendation: client-side is fine until ≥1k intents.)

## What this plan does not address

- Per-step rationale rendering (P6 — gated on backend parser changes).
- Workspace-scoped vs actor-scoped rail visibility — out of scope here, addressed in the v1 plan's "Plan-listing privacy scope."
- Per-target agent buttons sunsetting plan — separate product decision tracked in the v1 plan's open follow-ups.
- Mobile/tablet variants. Today the bar is desktop-first; a future plan will cover responsive.

## Risks

- **Refactoring the rail (T2)** touches the workspace layout. If not contained, regressions show up everywhere. Mitigation: ship behind a feature flag or split into the layout-grid PR + rail-component PR.
- **Status vocabulary unification (X1)** is a breadth-first refactor. Easy to leave one surface stale. Mitigation: test the rail, plan card, dialog, and intents page in the same PR.
- **Tool picker grouping (T3)** changes default state of localStorage-keyed UI. Existing users may see groups collapse on first load. Mitigation: default all groups expanded on first visit per agent; only collapse on explicit user action.

## Test plan

- Each top-5 item: dogfood on `helpin-dev-fe.tryunhide.com` with the same flow used to validate v1 (Cmd+K → parse → narrow tools → confirm → rail → save agent).
- Status vocabulary unification (X1): visual diff of `STATUS_META` rendering on a single fixture page covering all states.
- Cross-cutting (X3, X4): manual sweep of every command-bar surface.
- No new automated tests required; existing typecheck + vite build remain the build gate.
