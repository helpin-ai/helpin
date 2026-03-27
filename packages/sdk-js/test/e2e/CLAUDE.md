# SDK Widget E2E Notes

This folder contains Playwright browser coverage for the SDK and widget runtime.

## What is here

- root `test/e2e/` specs for SDK/browser compatibility
- `widget/mock/` for deterministic widget runtime coverage
- `widget/live/` as the future location for backend-backed smoke coverage

## Canonical Layout

Keep the widget browser suites split by environment layer:

- `widget/mock/` is the main regression harness
- `widget/live/` is for a narrow smoke layer only

Do not move widget-chat specs back into a flat folder once this split exists.

## Design Intent

- `widget/mock/` should hold the broad behavior matrix: boot, session lifecycle, unread, typing, links, previews, transcripts, attachments, and failure paths.
- `widget/live/` should eventually prove only a few high-value live integration paths.
- Generic SDK/browser compatibility tests can remain at the `test/e2e/` root when they are not support-widget specific.

## Minimum bar for widget changes

- Keep the widget mock suite green.
- Prefer adding deterministic widget coverage before adding a live smoke case.
- Keep `pnpm --dir packages/sdk-js run build` green.
