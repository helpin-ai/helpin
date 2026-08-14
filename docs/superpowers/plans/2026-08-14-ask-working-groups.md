# Ask Working Groups Implementation Plan

**Goal:** Preserve and disclose the Ask agent's full recorded work while keeping only the current activity expanded during live execution.

**Architecture:** Replace destructive root-chat progress compaction with a pure frontend grouping pass over normalized transcript segments. Render inferred working groups with independent disclosure state, and render their original progress, reasoning, tool inputs, outputs, errors, and timing without summarization.

**Tech Stack:** React 19, TypeScript, Vitest, existing transcript segment model and Tailwind UI primitives.

---

### Task 1: Derive stable sequential working groups

**Files:**
- Create: `frontend/src/components/agents/dock/dockWorkingGroups.ts`
- Test: `frontend/src/components/agents/dock/__tests__/dockWorkingGroups.test.ts`

- [x] Write failing tests for progress/tool grouping, final-answer separation, active-group selection, and stable live/persisted IDs.
- [x] Run `pnpm test -- src/components/agents/dock/__tests__/dockWorkingGroups.test.ts` and verify the missing implementation fails.
- [x] Implement a pure grouping function that preserves every original segment.
- [x] Run the focused tests and verify they pass.

### Task 2: Render full tool details

**Files:**
- Modify: `frontend/src/components/agents/transcript/segmentRenderers.tsx`
- Test: `frontend/src/components/agents/transcript/__tests__/segmentRenderers.test.tsx`

- [x] Write failing tests for visible arguments, full results, error text, malformed payload fallback, and timing.
- [x] Run the focused renderer tests and verify the new assertions fail.
- [x] Add an opt-in full-detail tool rendering mode while preserving concise tool rows elsewhere.
- [x] Run the focused tests and verify they pass.

### Task 3: Add live auto-collapse with independent manual expansion

**Files:**
- Create: `frontend/src/components/agents/dock/DockWorkingGroup.tsx`
- Modify: `frontend/src/components/agents/dock/DockTranscript.tsx`
- Modify: `frontend/src/components/agents/dock/ChatView.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx`

- [x] Write failing component tests showing the active group open, its automatic collapse after a successor starts, multiple manual expansions, preserved details, and an uncollapsed final answer.
- [x] Run the focused Dock transcript tests and verify the behavior fails.
- [x] Render working groups in root Ask chat instead of deleting earlier assistant progress.
- [x] Preserve group disclosure state through streaming updates using stable keys; lazy-mount collapsed bodies.
- [x] Run the focused Dock transcript tests and verify they pass.

### Task 4: Verify the integrated transcript

**Files:**
- Modify only if verification exposes a scoped regression.

- [x] Run the transcript and Dock test suites.
- [x] Run `pnpm test` in `frontend`.
- [x] Run `pnpm build` in `frontend`.
- [x] Run `git diff --check` and inspect the final diff.
- [x] Commit the implementation to `develop`.

### Task 5: Keep live status conversationally accurate

**Files:**
- Modify: `frontend/src/components/agents/dock/AgentLiveStatus.tsx`
- Modify: `frontend/src/components/agents/dock/ChatView.tsx`
- Modify: `frontend/src/components/agents/dock/agentProgress.ts`
- Test: `frontend/src/components/agents/dock/__tests__/agentProgress.test.ts`
- Test: `frontend/src/components/agents/__tests__/AskAgentsDock.test.tsx`

- [x] Render live status inline after the latest chat content using the compact thinking-row treatment.
- [x] Suppress the misleading idle `Waiting for your reply` label.
- [x] Make optimistic sends show `Starting…` before stale prior-run progress.
- [x] Add a clear turn boundary before the first assistant response after each user message.
- [x] Cover the status semantics, placement, and spacing with regression tests.
