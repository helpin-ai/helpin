# Ask working groups implementation plan

> Historical implementation record, reviewed against the checkout on 2026-09-17.
> This page explains the original transcript work for contributors. Later UI and
> compact-history changes supersede several checked tasks below; the original
> test/build/commit checklist is not a fresh verification record.

## Current presentation and differences

The [grouping function](../../frontend/src/components/agents/dock/dockWorkingGroups.ts)
preserves the normalized segments it receives, but live grouping now combines
adjacent tool segments only when there are at least two. Non-tool segments remain
separate. Completed-turn grouping can collect preceding work into a disclosure
while keeping the selected final response outside it.

[DockWorkingGroup](../../frontend/src/components/agents/dock/DockWorkingGroup.tsx)
starts collapsed even when active. Its label can show the latest tool while
working; users open details manually, and manual disclosure state survives active
state transitions for the mounted group. Bodies are mounted only while open.
This supersedes the original automatic expansion of the current activity.

The [tool renderer](../../frontend/src/components/agents/transcript/segmentRenderers.tsx)
uses concise tool rows and an expandable error presentation. The original promise
to display all arguments, successful results, and timing without summarization
is not the current rendering contract. Keeping segment data in the grouping pass
does not establish that every field is visible or was transferred to the browser.
Completed chat history also uses the later
[lazy work-detail design](../specs/2026-08-18-dock-chat-lazy-work-history-design.md).

[Live progress](../../frontend/src/components/agents/dock/agentProgress.ts) gives
optimistic sends `Starting…` priority and suppresses status after an answered
turn. Pause reasons are handled separately: user-message waiting can be silent,
while interaction and authentication pauses can show waiting text. The old
“suppress Waiting for your reply” task is not a blanket ban on all waiting labels.

## Original implementation record

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
