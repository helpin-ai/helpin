# Deal creation implementation plan

> Historical implementation record, compared with the checkout on
> 2026-09-17. Creation, stage/filter, and shared display-control sources were
> inspected; visual/browser results below were not rerun. Original branch
> instructions and test counts below are historical, not current verification.

## Verified creation differences

The [create form](../../frontend/src/components/crm/CreateDealDialog.tsx) mounts
only while open, keeps an editable generated name, remembers currency per
workspace after successful creation, and defaults a new deal to annual revenue
and a close date 30 days ahead. It does **not** currently render or submit a
probability override or custom-property editor. The original completion and
compatibility claims for those fields are therefore not the current UI contract.

The [service](../../server/internal/service/crm_deal.go) defaults an omitted
revenue type to `one_time`, unlike the form's explicit `annual` default. The
[creation repository](../../server/internal/repository/crm_deal.go) creates the
deal and canonical customer/participant associations in one transaction and
checks additional contacts against the workspace. The
[revenue helpers](../../frontend/src/components/crm/dealCreationDefaults.ts)
calculate potential MRR/ARR from monthly/annual amounts; these are arithmetic
projections, not recorded subscription revenue. Totals are omitted for mixed
currencies or revenue periods.

[Deal updates](../../frontend/src/lib/services/crmService.ts) use a client-side
queue keyed by workspace and record. This serializes calls through that client
helper; it is not a distributed concurrency guarantee across users or browsers.

## Verified follow-up implementation

The [stage-color migration](../../server/internal/dbmigrate/sql/202609120003_crm_stage_colors.sql)
backfills only null colors and enforces six-digit hex values. Pipeline updates
preserve omitted existing colors; duplication copies them. The
[stage picker](../../frontend/src/components/crm/DealStageSelect.tsx) uses shared
state content and a popover selector, with fallback colors for missing values.

[Deal filter definitions](../../server/internal/repository/crm_deal_query_builder.go)
feed the reusable server query builder, including numeric amount/probability and
date fields. [Deals](../../frontend/src/pages/crm/Deals.tsx) stores serialized
filters in route search and forwards them to list queries.

[Board dragging](../../frontend/src/components/crm/DealBoard.tsx) reuses task-board
drop targeting/commit helpers. [Shared deal edits](../../frontend/src/components/crm/useDealEdits.ts)
keep optimistic patches per record and refuse another edit to the same record
while pending in that hook scope. Success updates cached rows; failure removes
that record's optimistic patch, preserving other records' changes. Old-scope
completions do not invoke current UI callbacks. This is client-side coordination,
not server-side conflict detection.

[Deal display controls](../../frontend/src/components/crm/DealDisplayMenu.tsx),
CRM column controls, and PM display adapters use the shared display-settings
component. That source reuse does not by itself prove visual parity on every
screen, theme, or viewport. The original browser counts, screenshots, full build,
and disposable PostgreSQL checks remain historical results.

## Original implementation record

Implement directly in waqar-fixes with no sub-agents. This is the single plan for the approved form.

Goal: all fields visible in this order: company, contacts, editable generated name, amount/currency/revenue type, pipeline/stage, owner/close date, probability; no heading subtext.

Architecture: reuse CRM associations for the customer, primary person and participants. Extend atomic deal creation for additional contact IDs and persist a revenue period. Use centralized Quiet dropdowns and underline inputs. Mount form state only while open to reset safely. Keep caller context explicit.

- [x] Add tests for atomic participant creation, workspace validation and revenue periods; implement backend DTO/model/service/repository and additive migration.
- [x] Build tested form defaults/revenue helpers and company/contact picker with inline creation, association lookup, stale-search protection and clear errors.
- [x] Replace CreateDealDialog with the approved visible form; inherit pipeline/stage from Deals and board columns; default owner to current member. Preserve edited names and explicit probability overrides.
- [x] Show/edit revenue period in deal details, calculate potential MRR/ARR, and support changing primary contact while retaining people.
- [x] Verify targeted Go and frontend tests, TypeScript/lint and rendered form interactions. Review diff and commit only task files.

Compatibility: preserve custom properties, company-only and independent-person deals, existing creation callbacks, toasts and routes. Existing amounts retain one-time semantics. The app currently has no workspace currency preference: preserve USD fallback and remember the user's last explicit currency per workspace.


Follow-up scope approved in the same task: match task state controls and reliable moves, add editable stage colors, add Deals filters like Contacts, and unify display settings and Board/List controls across the platform.

- [x] Add persisted stage colors with the task color picker, safe defaults and color validation. Preserve colors through stage edits, reorders and pipeline duplication.
- [x] Reuse the task state content and picker for deal stage selection; show stage colors in board/list/detail/create/settings.
- [x] Reuse task board drop-target and commit helpers; isolate optimistic rollback per deal and guard in-flight/context changes. Verify successful, failed and concurrent moves.
- [x] Add server-backed Deals filters using the shared query builder, including numeric comparisons and removable applied filters.
- [x] Unify display menus across Deals, Contacts, Companies, Tasks, Epics and embedded task views; match view icons and lighten the inactive icon.
- [x] Verify stage UI and transitions; deliver the follow-up changes on waqar-fixes.

Verified creation: 18 frontend unit checks, 7 browser cases (including task-style toast/Open and inline contact retry), CRM service tests, repository/dbmigrate tests, TypeScript, new-form lint, and a disposable PostgreSQL migration check. Existing board/card/table lint findings are being assessed in the follow-up stage work. Toolbar from waqar-images/pipeline.png is hidden for workspaces with zero deals.

Follow-up verification (2026-09-13): 69 frontend unit checks; creation, pipeline-settings and deal-stage Chromium cases, including color editing/copying, numeric filtering, independent rollback, failed owner edits, keyboard cancellation and Space/Enter moves, and narrow dark display settings. Targeted Go model/repository/service/handler/dbmigrate checks pass. The stage-color SQL passes backfill, idempotency, custom-color preservation and constraint checks in disposable PostgreSQL. Full frontend TypeScript check passes. No new lint errors; pre-existing findings remain in the task board and pipeline browser fixture.

Deal updates serialize by workspace/record. Board and list apply optimistic patches per record, preserving unrelated successful saves. Keyboard Left/Right targets adjacent stage centers, including empty stages; pointer targeting and drop commit timing use the task board helpers. The visual drag overlay has a separate registration from the source card.
