# Automatic Icons Implementation Plan

**Goal:** Suggest icons while naming new collections and support inboxes without interrupting creation or overriding manual choices.

**Architecture:** Extend the existing operator-funded Jev decision service with `icon_selection`, its bounded admission/usage audit and independent policy. Supply a curated canonical shortlist plus name matches. Expose two permission-protected endpoints and share a debounced frontend hook. Existing items and agent avatars remain unchanged; featured cards inherit collection icons as today.

**Tech stack:** Go/Chi, existing Jev provider and icon catalog, React/TypeScript/Vitest.

- [x] Add failing service tests for canonical selection, uncertainty, invalid input, caching and disabled modes; hook tests for debounce, stale responses, manual overrides/removal and reopening.
- [x] Extend Jev policy and database constraint; implement candidates and service using existing admission/metering. Add handlers and routes with docs.edit/support.admin permissions.
- [x] Implement shared suggestion hook and connect create dialogs; preserve manual choices and default fallback, keep saving independent of suggestions.
- [x] Run focused Go/Vitest checks, frontend type check and migration validation; review final diff and commit only task changes.

Validate broad baseline candidates for non-English names, cancellation on close/save, no requests for edit forms, and no persistence after submit. No live provider calls required for deterministic tests.

## Validation and delivery

- Frontend: 8 automatic-icon lifecycle tests and 3 picker tests passed; TypeScript and scoped ESLint passed.
- Backend: icon service, handler, authorization, Jev policy/cache tests passed; API package compiled.
- Migration: uses 202610250003 after resolving an existing version collision; preserves all prior decision feature values. Embedded migration checks cover the new constraint.
- Reuses the existing operator-funded Jev admission and usage recording; JEV_ICON_SELECTION_MODE, _THRESHOLD, and _DAILY_LIMIT provide independent controls.
- Live Jev quality/latency and database application are not exercised locally. Deployment requires the included migration.
