# Custom flow builder implementation plan

This plan covers the approved custom-flow drawer for contributors implementing and reviewing the change. The same drawer now covers custom creation, template setup, and editing, per the approved scope updates. Status: option-parity implementation complete; final regression checks recorded below. Live model verification and application deployment have not been performed. The builder reuses the existing Ask Agent runtime connection.

**Goal:** Create custom flows, set up templates, and edit existing flows through one concise agent drawer, with approval before saving.

**Architecture:** Reuse Ask Agent's durable dock conversations, streaming, and question controls. A private flow-builder conversation receives a restricted tool set and a server-validated draft; an agent save tool requires an owner-resolved approval matching the exact draft revision. Shared Community code uses existing edition and AI usage contracts.

**Tech stack:** Go, PostgreSQL/GORM, Agent Runtime, React, TanStack Query, shared Helpin Sheet and conversation components.

## Scope and parity

- Keep template selection and installer side effects; route template setup and flow editing through the agent drawer. Editing opens with the current flow and asks what to change.
- Preserve supported workflow, GitHub/GitLab, and schedule triggers; agent, task-state, and branch-merge actions; explicit targets; branch options; and optional semantic conditions.
- Do not infer a team. Resolve named entities against workspace data. Unsupported requests require a clear response rather than an approximate flow.
- Use descriptive prose with key details bolded for the current flow and the proposed summary. Describe the current flow once, then ask what to change. Conditions are changed conversationally. No structured field rows, Advanced section, or default team.
- Keep AI usage/error handling, cancellation, private ownership, and retries. Require a fresh validated draft before creation; repeated create requests return the same flow.

## Tasks

- [x] Add backend tests for restricted tools, invalid/cross-workspace draft references, unsupported conditions, and stale/repeated creation.
- [x] Add nullable builder state to `model/dock_chat.go` with an idempotent migration. Implement draft contracts, catalog, validation, and explicit creation in focused `flow_builder*.go` files.
- [x] Connect three scoped tools to InternalCommandService and constrain builder chats in DockChatService. Reuse existing run preflight and metering. Verify backend and frontend integration.
- [x] Add a custom-flow Sheet and concise prose review under `frontend/src/components/automation/`. Extend the shared chat view with optional builder composition rather than copying streaming and interaction handling.
- [x] Route custom creation, template setup, and editing to the drawer. Preserve template installer behavior, flow identity when editing, and deep-link context.
- [x] Verify targeted Go and frontend tests, types/build, edition behavior, and browser states including narrow width, clarification, conditions, failure/retry, and creation. Review the final diff without sub-agents.

## Verification

Run focused service, repository, handler, migration, and UI tests first. Run frontend type checking/build and backend build/vet for changed packages. Browser-check the actual drawer with controlled fixtures; report separately whether a configured runtime was available for a real model turn. Do not publish operational details or private data in this plan.

## Verification results

- Backend regression checks cover all 16 former custom-editor triggers, the three editor actions, template-dependent references, agent override schema/validation, timezone review, and removing conditions while preserving unrelated fields.
- Integration tests execute the actual context, preview and save tools for custom creation, template installation and editing. They verify approval is required, paused state persists, edits retain identity and retrying a save does not duplicate a flow.
- Automation UI suite: 148 tests passed; the final focused prose suite has 6 passing tests, including a subsequently added check that generated template configuration stays hidden. Three Chromium tests pass for custom/template entry, edit/review/save, and narrow layout. Browser fixtures run only in test mode; they do not establish live model behavior.
- Community production build, Go build/vet, lint, and documentation checks pass. EE backend regression checks pass; EE TypeScript validation also passes.
- The public mock preview and its assets have been removed. Live application rollout, the deployment migration and model-driven end-to-end verification remain unperformed. There is no separate flow-builder runtime configuration.

## Completion follow-up

- [x] Reproduce and fix team/workflow state dependencies, conditional inputs, space types and collection ownership in `flow_builder.go` and repository helpers. Test valid and rejected combinations.
- [x] Expose workspace timezone, template defaults, eligible agent presets, repository branch discovery, and tools/skills through scoped context. Document and validate every former template agent setting.
- [x] Make the prose confirmation cover template inputs, edited descriptions, schedule timezone, and consequential agent overrides without redundant UI. Verify all supported trigger/action combinations.
- [x] Remove the mock preview assets and public mock servers; keep automated fixtures isolated from the application.
- [x] Run targeted regression tests, frontend build, Go build/vet, and review create/edit approval persistence.
- [ ] Deploy the application and migration and verify a live model conversation using the existing Ask Agent runtime connection. No sample-response fallback is used.
