# Email forwarding round-trip verification implementation plan

> Historical design and implementation record, reviewed against the checkout on
> 2026-09-17. This page explains the original forwarding verification work for
> contributors. Its task instructions, branch name, and validation checklist are
> historical; they do not establish current deployment or test results.

## Current behavior and design differences

The [send-test service](../../server/internal/service/support_email_route.go)
validates the source address, requires an active workspace-scoped route, saves a
new pending token before sending, and records delivery errors. The
[route registration](../../server/internal/router/router.go) requires
`support.admin` for `POST /api/support/inbox/email-routes/{routeId}/send-test`.

[Inbound processing](../../server/internal/service/email_fallback.go) recognizes
Google/Zoho confirmation patterns before test matching. Detected confirmations
remain available as conversations and do not verify forwarding. A subject containing
`Helpin forwarding test` is consumed even if its bracketed token is absent or
wrong; only a match with the route's pending token records test success. This is
broader consumption than the original “that exact message” wording suggests.
Non-confirmation ordinary mail mentioning the configured source address can also
verify an unverified route and continues through normal conversation processing.
These are inbound-message checks, not proof of ongoing provider access.

Verification uses the [repository's ordinary row save](../../server/internal/repository/support_email_route.go).
The original “atomically” wording should not be read as a token-conditional update
or concurrency guarantee: the read/match/save sequence has no compare-and-swap
predicate. The matching code also does not expire a token based on its sent time.

The current [setup component](../../frontend/src/components/settings/EmailForwardingSetup.tsx)
is a four-step flow. The [route query](../../frontend/src/hooks/queries/useSupport.ts)
polls every three seconds while setup is open with an unverified route; outside
that condition, pending tests poll for up to ten minutes. That UI polling window
is not a server-side verification-token lifetime. Existing-route behavior also
includes the later [verification backfill](2026-08-11-forwarding-verification-backfill.md).

## Original record

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

- [x] Write a failing route-state repository/service test.
- [x] Add route verification fields and scoped repository updates.
- [x] Run the focused Go tests.

### Task 2: Send and complete a round-trip test

**Files:**
- Modify: `server/internal/service/support_email_route.go`
- Modify: `server/internal/service/email_fallback.go`
- Modify: `server/internal/handler/support_inbox.go`
- Modify: `server/internal/router/router.go`
- Test: `server/internal/service/support_email_route_test.go`
- Test: `server/internal/service/email_fallback_test.go`

- [x] Write failing tests for sending a tagged message and consuming its returned payload.
- [x] Add the admin endpoint and service orchestration.
- [x] Detect confirmation messages without treating them as verified.
- [x] Verify qualifying real mail while preserving normal processing.
- [x] Run the focused Go tests.

### Task 3: Show truthful status and next steps

**Files:**
- Modify: `frontend/src/lib/pm-types/support.ts`
- Modify: `frontend/src/lib/services/supportService.ts`
- Modify: `frontend/src/hooks/queries/useSupport.ts`
- Modify: `frontend/src/components/settings/SupportEmailForwardingTab.tsx`
- Modify: `frontend/src/components/settings/ConversationRoutingTab.tsx`
- Test: `frontend/src/components/settings/__tests__/conversationRoutingStatus.test.ts`
- Test: `frontend/src/components/settings/__tests__/supportEmailForwardingVerification.test.ts`

- [x] Write failing status and checklist tests.
- [x] Add the send-test client mutation and temporary polling.
- [x] Render orange incomplete state, checklist, and verified timestamp.
- [x] Run focused Vitest and ESLint checks.

### Task 4: Verify and commit

- [x] Run focused backend and frontend verification.
- [x] Run `git diff --check` and inspect the final diff.
- [ ] Commit the complete scoped change on `waqar-fixes`.
