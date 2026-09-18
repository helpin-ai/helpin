# Playbook publishing implementation plan

> Historical implementation record (2026-09-10), source-compared on 2026-09-17.
> The four-step setup, separate Automation tab and save-before-confirmation path
> are implemented. Old test totals, screenshots, review and commit/push directions
> below belong to that session; they are not fresh validation or current workflow
> instructions.

## Current publication boundary

[Setup readiness](../../frontend/src/lib/crmPlaybookSetup.ts) covers Purpose & scope,
Milestones, Team & permissions, and Monitoring. The
[editor](../../frontend/src/components/crm/playbooks/PlaybookEditor.tsx) serializes
saves, preserves edits on failure, and passes the returned saved revision to the
[detail confirmation](../../frontend/src/pages/crm/PlaybookDetail.tsx). The
[publish action](../../frontend/src/components/crm/playbooks/PlaybookPublishAction.tsx)
uses `aria-disabled` with tooltip feedback; handlers also prevent blocked actions.

Publishing a Playbook version, publishing an automation connection, allowing
customer enrollment, and enabling automation are distinct operations. A ready
form or successful publication does not start work. The
[Playbook service](../../server/internal/service/crm_playbook.go) and repository
validate version publication separately from enrollment. The
[execution service](../../server/internal/service/crm_playbook_execution.go)
requires confirmed configuration of a published connection, revision/limit checks,
permissions and applicable entitlements before enabling automation. Consult the
[automation product model](../automation-product-model.md) for that runtime boundary.

Source inspection verifies these paths exist; it does not verify a deployed
Playbook, enabled connection, or the historical browser/test results below.

User-approved scope: four setup steps ending with Monitoring, a separate Automation tab, live required-field feedback on a disabled Publish action, and automatic draft saving before publish confirmation. Keep all changes local.

- [x] Cover four-step navigation, live missing-field feedback, touch/keyboard access, save-before-confirmation, saved revision, failure preservation, and automation settings persistence with browser regressions; observe failures first.
- [x] Keep form state and save ownership in PlaybookEditor. Render Save draft and Publish together into the detail header. Publish validates the current draft, serializes saving, and passes the saved revision to the parent confirmation. Preserve explicit final publication and optimistic concurrency.
- [x] Remove review content and navigation. Move Preview matches to Matching signals and enrollment/automation controls into a separately mounted Automation tab. Preserve previews, enrollment permissions/confirmation, readonly inspection, unsaved form and automation state.
- [x] Generate concise readiness messages per field and step, including numbered milestone problems. Show them through a hover/focus/tap tooltip on an aria-disabled Publish action; show incomplete-step indicators. Keep loading/error/stale verification blockers distinct from missing data.
- [x] Run targeted unit/browser tests and frontend TypeScript checks, inspect desktop/mobile screenshots, request bounded review, fix findings, and commit only scoped files. Do not push.

Capability audit: remove only the duplicated review summary as explicitly approved. Preserve all setup fields and validation, Save draft, Preview matches, explicit publish confirmation, version/revision safety, enrollment controls, automation connection/activation/limits, permissions, and unsaved-edits protection.

Verification: readiness unit tests passed (4); complete three-spec browser run passed (27), including live readiness, save-before-confirmation, failure/retry, saved revision, stale version, member verification, touch/keyboard tooltips, read-only preview, enrollment, and automation persistence. TypeScript build and diff checks passed. Desktop/mobile screenshots inspected. Review found and resolved read-only preview inheritance from the disabled fieldset; all editable controls now disable individually. No pushes.
