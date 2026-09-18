# Support inbox global search implementation plan

> Historical plan, source-compared on 2026-09-17. Global Support search is
> implemented. This page is for contributors tracing the original feature;
> unchecked tasks, proposed filenames, old toolchain versions, and test commands
> below are historical rather than a current setup or verification checklist.

## Current implementation

The UI lives in [SupportSearch](../../frontend/src/pages/pm/SupportSearch.tsx),
not the proposed `SupportGlobalSearchPage.tsx`. Its
[route](../../frontend/src/routes/_authenticated/w/$slug/support/search.tsx)
uses URL-backed query/filter/sort/page state. Parsing helpers are implemented
with the page/route rather than the proposed standalone `supportSearchRouting.ts`.
The actual migration is
[202606300001_support_inbox_global_search.sql](../../server/internal/dbmigrate/sql/202606300001_support_inbox_global_search.sql),
not the proposed `support_inbox_search_vectors.sql` name.

The [service](../../server/internal/service/support_inbox.go) requires a query
or at least one filter, limits queries to 256 Unicode characters and six quoted
phrases, and checks explicitly selected mailboxes. The
[repository](../../server/internal/repository/support_inbox.go) applies the
caller's mailbox scope to search results. “Global” therefore means searching
across permitted conversations, not bypassing access restrictions.

PostgreSQL uses generated search vectors with text-field substring alternatives;
the SQLite path uses substring matching and does not prove identical full-text
ranking or syntax. Message-body search includes nondeleted `reply` and
`email_notice` messages without a system event. Totals are capped at 1,000 and
reported with `total_capped`; that total is not an exact count above the cap.

The UI clamps/sorts highlight ranges and renders text slices inside React
`mark` elements, rather than injecting server-supplied highlight HTML. Existing
scoped inbox-list search remains a separate path. The original test/build steps
below were not rerun, and migration presence does not establish deployment.

## Original implementation record

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build first-class global Support Inbox search for the current `support_conversations` model.

**Architecture:** Add a dedicated backend search endpoint that applies mailbox access, structured filters, message-body search, capped totals, and safe highlight ranges. Add a full-width `/support/search` route with URL-backed state, filter chips, dense results table, and navigation to conversation detail. Keep existing scoped inbox-list search unchanged.

**Tech Stack:** Go 1.24, Chi, GORM, PostgreSQL generated/search-vector columns with SQLite test fallback, React 19, TanStack Router/Query, TypeScript, shadcn/ui.

---

## File Structure

- Modify `server/internal/model/support_inbox.go`: add search DTOs.
- Modify `server/internal/handler/support_inbox.go`: parse and serve `GET /support/inbox/search`.
- Modify `server/internal/service/support_inbox.go`: normalize params and call repository under existing mailbox access policy.
- Modify `server/internal/repository/support_inbox.go`: implement search query, filters, capped total, snippets, highlights, and SQLite fallback.
- Modify `server/internal/router/router.go`: register search route before conversation ID routes.
- Add `server/internal/dbmigrate/sql/202606300001_support_inbox_search_vectors.sql`: generated tsvector columns and indexes for Postgres.
- Add/modify `server/internal/repository/support_inbox_search_test.go`: backend behavior tests.
- Modify `frontend/src/lib/pm-types/support.ts`: add search response/filter types.
- Modify `frontend/src/lib/queryKeys.ts`: add support global search key.
- Modify `frontend/src/lib/services/supportService.ts`: add `searchConversations`.
- Modify `frontend/src/hooks/queries/useSupport.ts`: add `useSupportConversationSearch`.
- Add `frontend/src/lib/supportSearchRouting.ts`: URL search parse/build helpers.
- Add `frontend/src/components/support/SupportGlobalSearchPage.tsx`: global search page and components.
- Add `frontend/src/routes/_authenticated/w/$slug/support/search.tsx`: route.
- Modify `frontend/src/components/support/SupportInboxLayout.tsx`: route/sidebar entry behavior if needed.
- Add `frontend/src/components/support/__tests__/SupportGlobalSearchPage.test.tsx` and/or `frontend/src/lib/__tests__/supportSearchRouting.test.ts`.

## Task 1: Backend Search Contract

- [ ] Write failing repository tests for customer email, display number, title, message body, filters-only, capped totals, and mailbox access.
- [ ] Run `go test ./internal/repository -run SupportConversationSearch` from `server`; expect failure because search API does not exist.
- [ ] Add search DTOs in `server/internal/model/support_inbox.go`.
- [ ] Add `Search` repository method and SQLite-compatible implementation in `server/internal/repository/support_inbox.go`.
- [ ] Run focused repository tests; expect pass.
- [ ] Commit backend repository search contract.

## Task 2: Backend Handler, Service, Route, Migration

- [ ] Write failing handler/service tests or extend repository tests for validation: overlong query, empty query without filters, multiple quoted phrases, and route behavior.
- [ ] Run focused backend tests; expect failure.
- [ ] Add service method to normalize search params, enforce workspace ID, and reuse actor mailbox scope.
- [ ] Add handler parser and route registration under `/api/support/inbox/search`.
- [ ] Add dbmigrate SQL migration for generated vectors/indexes, guarded for idempotency.
- [ ] Run focused backend tests; expect pass.
- [ ] Run `go test ./internal/repository ./internal/handler ./internal/service`.
- [ ] Commit backend API and migration.

## Task 3: Frontend Types, Service, Query, Routing

- [ ] Write failing TypeScript tests for support search URL parsing/building and service param serialization.
- [ ] Run focused frontend tests; expect failure.
- [ ] Add search result/filter types.
- [ ] Add query key, service method, and query hook.
- [ ] Add URL routing helper.
- [ ] Run focused frontend tests; expect pass.
- [ ] Commit frontend search data layer.

## Task 4: Global Search UI

- [ ] Write failing component tests for rendering result count, highlights, filter changes, empty state, and result navigation.
- [ ] Run focused frontend tests; expect failure.
- [ ] Add `/w/:slug/support/search` route.
- [ ] Implement `SupportGlobalSearchPage`, toolbar, filter bar, results table, and safe highlight rendering.
- [ ] Wire sidebar Search navigation to the global route while preserving scoped list search icon.
- [ ] Run focused component tests; expect pass.
- [ ] Commit frontend global search UI.

## Task 5: Verification

- [ ] Run `go test ./internal/repository ./internal/handler ./internal/service` from `server`.
- [ ] Run `npm test -- --run` or the focused Vitest suite from `frontend`.
- [ ] Run `npm run build` from `frontend`.
- [ ] Run `git diff --check`.
- [ ] Summarize verification and remaining risk.
