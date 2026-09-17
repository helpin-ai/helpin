# Agent Run Turn Chronology Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:executing-plans to implement this plan. Subagents are explicitly disabled for this task.

**Goal:** Keep approval decisions in their true agent-run position and render completed standalone work behind the same `Worked for` disclosure used by Ask chats.

**Architecture:** Treat persisted event-list sequence numbers as the canonical cross-type chronology, with payload sequence retained only for legacy/runtime message events. Add a transcript-level completed-turn projection that groups already-loaded standalone-run progress while leaving the final response visible; keep Dock chat lazy disclosures untouched.

**Tech Stack:** React 19, TypeScript 5.9, Vitest, Go event projection API.

---

### Task 1: Canonical cross-type event chronology

**Files:**
- Modify: `frontend/src/components/pm/CodingSession/codingSessionUtils.ts`
- Test: `frontend/src/components/pm/CodingSession/__tests__/codingSessionUtils.test.ts`
- Test: `frontend/src/components/pm/CodingSession/__tests__/codingSessionStream.test.ts`

- [ ] Add a failing HEL-102 regression proving an approval resolution stays before resumed assistant messages despite their smaller payload message sequences.
- [ ] Add a failing utility test proving persisted projections use top-level sequence while legacy message events retain payload ordering.
- [ ] Run the focused tests and confirm the chronology assertions fail.
- [ ] Change only the event sort-key selection needed to make persisted cross-type ordering canonical.
- [ ] Rerun the focused tests and confirm they pass.

### Task 2: Standalone completed-work disclosure

**Files:**
- Modify: `frontend/src/components/agents/dock/dockWorkingGroups.ts`
- Modify: `frontend/src/components/agents/dock/DockWorkingGroup.tsx`
- Modify: `frontend/src/components/agents/dock/DockTranscript.tsx`
- Modify: `frontend/src/components/agents/dock/DockRunView.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/dockWorkingGroups.test.ts`
- Test: `frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/DockRunView.test.tsx`

- [ ] Add failing tests for one completed multi-step turn, a direct answer, and multiple user/approval boundaries.
- [ ] Run focused tests and confirm missing completed-work grouping causes failure.
- [ ] Extend the working timeline projection with an explicit completed-run mode and duration metadata.
- [ ] Render completed groups with `Worked for …`, collapsed by default, while preserving current active group labels and behavior.
- [ ] Pass terminal timing context only from standalone `DockRunView`; do not change chat-backed history.
- [ ] Rerun focused transcript and run-view tests.

### Task 3: Verification

**Files:**
- Verify only.

- [ ] Run all affected frontend test suites.
- [ ] Run the full frontend test suite.
- [ ] Run `pnpm --dir frontend exec tsc -b`.
- [ ] Run `pnpm --dir frontend build`.
- [ ] Run `git diff --check` and inspect the final diff for unrelated changes.

### Task 4: Automation Activity transcript parity

**Files:**
- Modify: `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`
- Modify: shared completed-work timeline/presentation modules only if needed
- Test: `frontend/src/components/pm/CodingSession/__tests__/CodingTranscriptPane.test.tsx`

- [x] Add a failing completed-session test proving progress and tool activity render behind the chat-style collapsed `Worked for …` disclosure while the final assistant response stays visible.
- [x] Run the focused test and confirm it fails because `CodingTranscriptPane` still renders a flat list.
- [x] Reuse the existing completed-work timeline and `DockWorkingGroup` presentation in `CodingTranscriptPane`; do not create a second visual variant.
- [x] Keep active sessions flat/live and preserve the transcript's chronological segment order.
- [x] Rerun the focused test, existing Coding Session tests, and Dock transcript tests.
