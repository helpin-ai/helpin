# Widget Composer Brand Alignment Design

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

**Date:** 2026-08-04

## Goal

Vertically align “We run on” with the Helpin mark and name in the chat widget composer footer.

## Design

Reuse the structure already proven by the correctly aligned “Powered by” footer. The composer footer becomes one centered flex row: an explicit label span and the existing Helpin link are sibling flex items, aligned with `align-items: center` and separated by a container gap. The whole Helpin lockup remains clickable, while the hover underline moves from the link to the `Helpin` name span so it never runs beneath the mark. The branding URL, visibility behavior, and mobile hiding behavior remain unchanged.

This avoids baseline alignment between a plain text node and an inline-flex link, and avoids brittle `translateY` or margin offsets. Regression coverage asserts the explicit label and name structure, the flex alignment contract, and the name-only underline.

## Verification

- Focused composer markup and widget style tests.
- Complete widget-core test suite.
- Widget-core typecheck and production build.
