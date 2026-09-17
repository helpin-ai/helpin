# Playbook publishing implementation plan

User-approved scope: four setup steps ending with Monitoring, a separate Automation tab, live required-field feedback on a disabled Publish action, and automatic draft saving before publish confirmation. Keep all changes local.

- [x] Cover four-step navigation, live missing-field feedback, touch/keyboard access, save-before-confirmation, saved revision, failure preservation, and automation settings persistence with browser regressions; observe failures first.
- [x] Keep form state and save ownership in PlaybookEditor. Render Save draft and Publish together into the detail header. Publish validates the current draft, serializes saving, and passes the saved revision to the parent confirmation. Preserve explicit final publication and optimistic concurrency.
- [x] Remove review content and navigation. Move Preview matches to Matching signals and enrollment/automation controls into a separately mounted Automation tab. Preserve previews, enrollment permissions/confirmation, readonly inspection, unsaved form and automation state.
- [x] Generate concise readiness messages per field and step, including numbered milestone problems. Show them through a hover/focus/tap tooltip on an aria-disabled Publish action; show incomplete-step indicators. Keep loading/error/stale verification blockers distinct from missing data.
- [x] Run targeted unit/browser tests and frontend TypeScript checks, inspect desktop/mobile screenshots, request bounded review, fix findings, and commit only scoped files. Do not push.

Capability audit: remove only the duplicated review summary as explicitly approved. Preserve all setup fields and validation, Save draft, Preview matches, explicit publish confirmation, version/revision safety, enrollment controls, automation connection/activation/limits, permissions, and unsaved-edits protection.

Verification: readiness unit tests passed (4); complete three-spec browser run passed (27), including live readiness, save-before-confirmation, failure/retry, saved revision, stale version, member verification, touch/keyboard tooltips, read-only preview, enrollment, and automation persistence. TypeScript build and diff checks passed. Desktop/mobile screenshots inspected. Review found and resolved read-only preview inheritance from the disabled fieldset; all editable controls now disable individually. No pushes.
