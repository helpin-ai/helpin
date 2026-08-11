# Email Forwarding Round-Trip Verification Implementation Plan

> **For agentic workers:** Execute locally with TDD. The user explicitly requested no subagents.

**Goal:** Replace first-email forwarding status with provider-independent end-to-end verification.

**Architecture:** Store route verification timestamps and a pending test marker on `support_email_routes`. Send a tagged test through the existing email client, recognize its return before conversation creation, and render status/checklist UI from the persisted evidence.

**Tech Stack:** Go, GORM, Postmark inbound/outbound email, React, TanStack Query, Vitest.

---

### Task 1: Persist verification state

**Files:**
- Modify: `server/internal/model/support_inbox.go`
- Modify: `server/internal/repository/support_email_route.go`
- Modify: `server/internal/service/testdb_test.go`
- Test: `server/internal/service/support_email_route_test.go`

- [ ] Write a failing route-state repository/service test.
- [ ] Add route verification fields and scoped repository updates.
- [ ] Run the focused Go tests.

### Task 2: Send and complete a round-trip test

**Files:**
- Modify: `server/internal/service/support_email_route.go`
- Modify: `server/internal/service/email_fallback.go`
- Modify: `server/internal/handler/support_inbox.go`
- Modify: `server/internal/router/router.go`
- Test: `server/internal/service/support_email_route_test.go`
- Test: `server/internal/service/email_fallback_test.go`

- [ ] Write failing tests for sending a tagged message and consuming its returned payload.
- [ ] Add the admin endpoint and service orchestration.
- [ ] Detect confirmation messages without treating them as verified.
- [ ] Verify qualifying real mail while preserving normal processing.
- [ ] Run the focused Go tests.

### Task 3: Show truthful status and next steps

**Files:**
- Modify: `frontend/src/lib/pm-types/support.ts`
- Modify: `frontend/src/lib/services/supportService.ts`
- Modify: `frontend/src/hooks/queries/useSupport.ts`
- Modify: `frontend/src/components/settings/SupportEmailForwardingTab.tsx`
- Modify: `frontend/src/components/settings/ConversationRoutingTab.tsx`
- Test: `frontend/src/components/settings/__tests__/conversationRoutingStatus.test.ts`
- Test: `frontend/src/components/settings/__tests__/supportEmailForwardingVerification.test.ts`

- [ ] Write failing status and checklist tests.
- [ ] Add the send-test client mutation and temporary polling.
- [ ] Render orange incomplete state, checklist, and verified timestamp.
- [ ] Run focused Vitest and ESLint checks.

### Task 4: Verify and commit

- [ ] Run focused backend and frontend verification.
- [ ] Run `git diff --check` and inspect the final diff.
- [ ] Commit the complete scoped change on `waqar-fixes`.
