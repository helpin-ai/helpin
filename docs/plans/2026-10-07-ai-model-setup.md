# Simplify AI model setup

Implementation plan for contributors, approved through the setup discussion on 2026-10-07. Shared Community functionality; managed connection policy remains behind existing edition contracts.

## Outcome and compatibility

Connect a provider, then enable models for Ask Agent. Keep profile IDs as internal route records. Preserve scopes, existing variant names, controls, fallbacks, agent/version references, workspace defaults, policy checks, and frozen run snapshots. Hiding only affects the Ask Agent picker. Existing entries start visible; discoveries start hidden. Members manage personal setup and see shared configuration read-only without a scope selector.

## Implementation

- [x] Add Ask Agent visibility, a revision-checked endpoint, and idempotent migration. Test authorization and continued default/unattended resolution after hiding.
- [x] Discover models through fixed official OpenAI, Anthropic, and OpenRouter endpoints. Persist successful results for six hours and support refresh. Preserve cached data on failure. Managed, subscription, and custom endpoint connections retain curated/manual choices. Discovery never changes routes, pricing, or defaults. Missing models are marked not listed, never silently deleted or substituted.
- [x] Replace Profiles with per-connection model management. Toggle discovered models, add exact IDs, and retain editing, variants, fallbacks, deletion protections, and defaults. Put supplementary explanations in tooltips. Models inherit connection scope.
- [x] Filter hidden entries only in Ask Agent, retaining saved/default context. Other agent configuration pickers remain unaffected. Update touched user-facing terminology.
- [x] Run focused backend/frontend checks in both editions, type checks, migration validation, and diff review. Commit in waqar-fixes.

## Files and validation

Backend: model, repository, service, and handler AI connection/profile files; router; model discovery adapter and tests; additive dbmigrate SQL.
Frontend: AISettingsPage, per-connection model component, AIProfileEditor, AIRouteFields, AIModelCombobox, AIProfilePicker, services/hooks and related tests.

Run focused Go service/handler tests and Vitest tests in both editions; TypeScript compilation; migrate validate; git diff --check. No paid live completions or deployment required.

## Verification notes

Focused UI tests cover one-click enable, scope permissions, saved variants and orphaned configurations, and hidden selections. Browser checks cover desktop/mobile model visibility and read-only workspace settings. Backend tests cover caching, provider filtering and pagination, authorization, idempotent enable, revision conflicts, and preserved defaults. The additive migration was applied twice to disposable PostgreSQL and retained existing credentials and configurations.

Discovery refreshes lazily when Settings requests a cache older than six hours, periodically while the model dialog is open, or on explicit refresh. ChatGPT and managed connections use supported catalog choices; compatible endpoints retain exact-ID entry. A missing provider listing is labelled “Not listed,” because disappearance alone cannot establish retirement. No provider credentials or paid live calls were used for verification. Full deployment migration status requires its DATABASE_URL; the migration itself was tested independently.

Final checks: 33 targeted Vitest tests passed in each edition; three Chromium checks passed; targeted Go service/handler regressions passed with and without the ee build tag; Community and hosted TypeScript checks passed; Go vet and the Community API build passed. dbmigrate unit tests and the disposable PostgreSQL migration test passed. Documentation naming/link checks, 11 documentation utility tests, secret scan, and diff whitespace checks passed. Changes are local; no deployment or provider-account verification was performed.
