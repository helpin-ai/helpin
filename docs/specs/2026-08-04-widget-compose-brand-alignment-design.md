# Widget Composer Brand Alignment Design

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
