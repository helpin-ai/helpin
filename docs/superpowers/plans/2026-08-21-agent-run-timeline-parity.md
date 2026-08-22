# Agent Run Timeline Parity Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give every agent execution surface one stable, causally ordered timeline and consistent live interaction presentation.

**Architecture:** Canonicalize interaction lifecycle events at the server boundary and defensively reconcile them by semantic identity in the frontend. Reuse the established Ask Agent runtime presentation contract in Agent Runs while keeping surface-specific controls separate.

**Tech Stack:** Go 1.24, React 19, TypeScript 5.9, Vitest, Go testing

---

### Task 1: Canonical interaction events

**Files:**
- Modify: `server/internal/service/coding_session.go`
- Test: `server/internal/service/coding_session_test.go`
- Modify: `frontend/src/components/pm/CodingSession/codingSessionUtils.ts`
- Test: `frontend/src/components/pm/CodingSession/__tests__/codingSessionUtils.test.ts`

- [x] Add failing tests for deterministic realtime IDs, `resolved_at` chronology, and realtime/REST semantic deduplication.
- [x] Run the focused tests and confirm the expected failures.
- [x] Publish deterministic interaction event IDs and use immutable lifecycle timestamps.
- [x] Add semantic frontend reconciliation with persisted-event precedence.
- [x] Run focused tests until green.

### Task 2: Agent Run presentation parity

**Files:**
- Modify: `frontend/src/components/agents/dock/DockRunView.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/DockRunView.test.tsx`

- [x] Add failing tests for compact progress, current-plan rendering, actor attribution, retained authoritative runtime history, and streamed-session status precedence.
- [x] Run the focused test and confirm the expected failures.
- [x] Feed Agent Runs through the established runtime presentation options and derive state from the freshest session.
- [x] Run the focused test until green.

### Task 3: Verification

**Files:** all files above

- [x] Run coding-session, dock timeline, and Agent Runs test suites.
- [x] Run focused backend interaction/coding-session tests; record unrelated package fixture failures.
- [x] Run changed-file lint, TypeScript build, `git diff --check`, and review the final diff.
- [x] Commit the verified correction on `waqar-fixes`.
