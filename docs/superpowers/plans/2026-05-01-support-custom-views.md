# Support Custom Views Implementation Plan

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
