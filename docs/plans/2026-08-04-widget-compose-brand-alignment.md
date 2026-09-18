# Widget Composer Brand Alignment Implementation Plan

> Historical design/implementation plan, source-compared on 2026-09-17. Branding
> alignment is implemented and has since moved into the shared
> [BrandAttribution](../../packages/widget-core/src/components/BrandAttribution.tsx)
> component. The whole attribution (label and brand) is one link, not a label
> beside a separate link. Shared CSS uses a 5px gap, rather than the proposed 4px;
> the hover underline remains on the name. ComposeBar retains its branding flag
> and mobile CSS hides the footer. The link is built by the
> [attribution URL helper](../../packages/widget-core/src/utils/helpinAttribution.ts).
>
> Current [markup tests](../../packages/widget-core/src/__tests__/ComposeBar.test.tsx)
> and [style tests](../../packages/widget-core/src/__tests__/widgetStyles.test.ts)
> describe the shared component. Checkboxes and the old `waqar-fixes` merge step
> below are historical records, not current branch instructions or a fresh
> test/build/visual validation report.

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Align the composer’s “We run on Helpin” attribution and keep its hover underline strictly beneath the Helpin name.

**Architecture:** Keep the existing composer and branding components. Add explicit label and brand-name elements, make the existing footer the single alignment container, and attach the underline decoration only to the name; preserve link, icon, responsive, and feature-flag behavior.

**Tech Stack:** Preact, TypeScript, CSS, Vitest, Testing Library.

---

### Task 1: Lock the alignment contract

**Files:**
- Test: `packages/widget-core/src/__tests__/ComposeBar.test.tsx`
- Test: `packages/widget-core/src/__tests__/widgetStyles.test.ts`

- [x] Add markup assertions for an explicit “We run on” label span and a dedicated Helpin name span.
- [x] Add style assertions for flex layout, centered cross-axis alignment, centered justification, and container gap.
- [x] Assert that the underline lives on the Helpin name and expands through the footer hover target.
- [x] Run the focused tests and confirm they fail because the alignment contract is missing.

### Task 2: Apply the minimal structural fix

**Files:**
- Modify: `packages/widget-core/src/components/ComposeBar.tsx`
- Modify: `packages/widget-core/src/styles/widget.css`

- [x] Wrap the label in a dedicated span.
- [x] Make `.helpin-compose-footer` a centered flex row with `align-items: center` and a four-pixel gap.
- [x] Move underline styles from the link to the Helpin name span.
- [x] Re-run the focused tests until green.

### Task 3: Verify and integrate

- [x] Run the complete widget-core test suite.
- [x] Run widget-core typecheck and production build.
- [x] Run `git diff --check` and review the scoped diff.
- [ ] Commit and merge the fix back to `waqar-fixes`.
