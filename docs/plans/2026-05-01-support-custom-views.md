# Support custom views implementation plan

> Historical implementation plan, reviewed against the checkout on 2026-09-17.
> Custom views are implemented. The unchecked steps below preserve the original
> work plan for contributors; they are not an outstanding implementation queue
> or instructions to execute the old agent workflow.

## Current behavior and differences

The [model](../../server/internal/model/support_inbox_view.go) supports custom,
default, and team view types. The [repository](../../server/internal/repository/support_inbox_view.go)
lists custom views visible to the caller (shared or created by them), ordered by
case-insensitive name and creation time. Built-in overrides are stored separately
by user/type/key.

The [service](../../server/internal/service/support_inbox_view_service.go) allows
only owners/admins to create or enable shared views. Private views can be edited
or deleted by their creator; shared views by their creator or an owner/admin.
These checks sit behind [route permissions](../../server/internal/router/router.go):
reads require `support.read`, while create/update/delete require `support.edit`,
in addition to Support module/workspace access. “Anyone can create private” in
the original plan therefore does not include every workspace role. Sharing a
filter preset does not grant access to otherwise restricted conversations;
count queries still receive the caller's membership and role.

The [store](../../frontend/src/stores/supportInboxStore.ts) now keeps the active
custom-view ID when search, status, or list filters change and sets
`customViewDirty`. This supersedes the original clear-on-edit requirement.
Switching built-in navigation or resetting filters can clear the active view.
[ConversationList](../../frontend/src/components/support/ConversationList.tsx)
provides save/update behavior, while the
[Support sidebar](../../frontend/src/components/layout/sidebar/SupportRailNav.tsx)
shows the custom-view section and shared indicator.

The original npm/npx commands and unchecked verification tasks below are
historical. Use the repository's current pnpm workspace and targeted test setup
when changing this feature; this review inspected sources rather than rerunning
the original broad suites or applying migrations.

## Original implementation record

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add private/shared saved support inbox views that appear in one alphabetized Custom views section.

**Architecture:** Mirror the PM saved-view ownership pattern with a support-specific model, repository, service, handler, and API routes. Frontend stores the active custom view separately from built-in nav filters, saves current list filter state from the filter popover, and renders custom views in the support sidebar with a subtle shared indicator.

**Tech Stack:** Go + GORM + Chi, React + TypeScript + TanStack Query + Zustand + shadcn/ui.

---

### Task 1: Backend Saved View API

**Files:**
- Create: `server/internal/model/support_inbox_view.go`
- Create: `server/internal/repository/support_inbox_view.go`
- Create: `server/internal/service/support_inbox_view_service.go`
- Create: `server/internal/handler/support_inbox_view.go`
- Modify: `server/cmd/api/main.go`
- Modify: `server/internal/router/router.go`
- Test: `server/internal/service/support_inbox_custom_view_test.go`

- [ ] Write failing service/repository tests for listing visible views alphabetically, private/shared visibility, shared creation permissions, and edit/delete permissions.
- [ ] Run focused Go test and verify it fails because the new types do not exist.
- [ ] Add support inbox view model with `workspace_id`, `name`, `filters`, `is_shared`, `created_by`, timestamps.
- [ ] Add repository list/create/get/update/delete methods.
- [ ] Add service validation and permissions: anyone can create private, admin/owner can create shared, creator/admin/owner can edit/delete shared, creator can edit/delete private.
- [ ] Add handler and routes under `/support/inbox/views`.
- [ ] Wire AutoMigrate, DI, and router handler.
- [ ] Run focused Go tests and verify pass.

### Task 2: Frontend Data Layer And Store

**Files:**
- Modify: `frontend/src/lib/pm-types/support.ts`
- Modify: `frontend/src/lib/services/supportService.ts`
- Modify: `frontend/src/hooks/queries/useSupport.ts`
- Modify: `frontend/src/lib/queryKeys.ts`
- Modify: `frontend/src/stores/supportInboxStore.ts`
- Test: `frontend/src/stores/__tests__/supportInboxStore.test.ts`

- [ ] Write failing store tests for applying a custom view and clearing it when selecting a built-in view or editing filters.
- [ ] Add SupportInboxView types and service methods.
- [ ] Add query/mutation hooks for list/create/update/delete views.
- [ ] Add store state/actions for `activeCustomViewId`, `applyCustomView`, and clearing active view on built-in changes.
- [ ] Run focused frontend tests and verify pass.

### Task 3: Sidebar And Save UI

**Files:**
- Modify: `frontend/src/components/support/ConversationList.tsx`
- Modify: `frontend/src/components/layout/sidebar/SupportRailNav.tsx`
- Modify: `frontend/src/components/layout/Sidebar.tsx`
- Test: `frontend/src/components/support/__tests__/ConversationList.test.tsx`

- [ ] Write failing component tests for Save as view request payload and custom view sidebar rendering/application.
- [ ] Add Save as view action in filter popover with name input and shared toggle gated by `canManageSettings`.
- [ ] Render one alphabetized Custom views sidebar section.
- [ ] Show a subtle shared indicator for shared views.
- [ ] Add item menu for rename/delete and visibility update where permitted.
- [ ] Apply custom views by restoring stored nav/mailbox/search/list filters.
- [ ] Run focused frontend tests and verify pass.

### Task 4: Verification

- [ ] Run `npm test -- supportInboxStore ConversationList supportInboxFilters`
- [ ] Run `npx tsc -b --noEmit`
- [ ] Run `GOCACHE=/tmp/go-build-cache go test ./internal/repository ./internal/service -count=1`
- [ ] Run `git diff --check`
