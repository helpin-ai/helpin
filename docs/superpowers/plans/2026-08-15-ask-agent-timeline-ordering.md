# Ask Agent Timeline Ordering Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Preserve the Ask agent's runtime execution order after pause/completion and render a compact, privacy-safe work timeline that visually separates internal work from the final answer.

**Architecture:** Reconcile durable chat rows with the retained runtime snapshot by stable assistant-message and tool-call IDs, then use that ordered runtime timeline independently from the run's visual live state. Derive conversational intervals in the dock renderer so progress, final-response styling, and separators are structural rather than content-based. Keep tool disclosures deliberately lossy: identity/status for success, error-only expansion for failure.

**Tech Stack:** React 19, TypeScript, Vitest/jsdom, Tailwind CSS, pnpm/Vite

---

### Task 1: Preserve runtime chronology after pause and completion

**Files:**
- Modify: `frontend/src/components/agents/dock/dockChatTimeline.ts`
- Modify: `frontend/src/components/agents/dock/DockTranscript.tsx`
- Modify: `frontend/src/components/agents/dock/ChatView.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/dockChatTimeline.test.ts`
- Test: `frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx`

- [ ] **Step 1: Add the production-shaped reconciliation regression**

Create durable messages where progress assistant rows have no `turn_segments`, the final durable row owns an aggregate tool list, and `live_turn_segments` contains the correct assistant/tool interleaving. Assert reconciliation retains snapshot assistant segments by runtime message ID.

- [ ] **Step 2: Run the focused tests and verify RED**

Run:
`pnpm --dir frontend exec vitest run src/components/agents/dock/__tests__/dockChatTimeline.test.ts src/components/agents/dock/__tests__/DockTranscript.test.tsx`

Expected: failure because earlier snapshot assistant segments are filtered out and paused rendering ignores the snapshot timeline.

- [ ] **Step 3: Retain matched snapshot assistant segments**

In `mergePersistedChatMessages`, allow `segmentIdentityKeys(segment)` to match `persistedMessageKeys` as well as `persistedSegmentKeys`. Continue discarding unrelated historical snapshot leftovers.

- [ ] **Step 4: Define trustworthy retained chronology**

Add a focused helper that returns true for paused/completed chronology only when `live_turn_segments` contains both assistant and tool segments and fully covers stable assistant-message IDs and tool-call IDs from the durable current conversational interval. Add incomplete and wholly unmatched snapshot cases that return false and preserve durable order. Active streams remain eligible without durable coverage because projection can legitimately lag.

- [ ] **Step 5: Separate timeline reconciliation from visual activity**

Extend `collectSegments` with an independent `runtimeActive` option while preserving existing-call defaults. `includeLive` controls whether runtime chronology is reconciled; `runtimeActive` alone controls streaming flags and live reasoning treatment. In `DockTranscript`, pass retained chronology and active styling separately. In root `ChatView`, enable retained chronology only for an active stream or when the trust helper passes. Keep `active={isDockTranscriptStreaming(run)}` as the sole source for spinner, accent, and automatic expansion.

- [ ] **Step 6: Assert execution phases survive completion**

Render the production-shaped stream with `active={false}` and runtime-timeline reconciliation enabled. Assert the sequence is tool group → progress → tool group → progress → final, groups are inactive/collapsed, and the final response remains flat.

Rerender the same component from running to paused/completed while durable messages intentionally lag. Assert no progress or final segment disappears during the handoff and no additional message polling loop is introduced.

Add paused stale-`streaming` coverage proving no assistant caret, reasoning Live marker, or active group survives. Add cumulative multi-turn coverage with a review-decision boundary and assert older runtime segments stay settled in their original interval.

- [ ] **Step 7: Run focused tests and verify GREEN**

Run the Task 1 command and expect all tests to pass.

### Task 2: Make reasoning and assistant hierarchy structural

**Files:**
- Modify: `frontend/src/components/agents/dock/dockWorkingGroups.ts`
- Modify: `frontend/src/components/agents/dock/DockTranscript.tsx`
- Modify: `frontend/src/components/agents/transcript/segmentRenderers.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/dockWorkingGroups.test.ts`
- Test: `frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx`

- [ ] **Step 1: Add failing reasoning-boundary tests**

Assert reasoning is emitted as a flat muted disclosure and is never included inside a tool working group, including when it precedes a live tool tail.

- [ ] **Step 2: Add failing conversational-interval tests**

Cover two user intervals and an active current interval. Assert completed interval finals are high contrast, earlier/current progress is muted, and an active interval has no structurally final response yet.

- [ ] **Step 3: Add failing separator tests**

Assert a divider appears immediately before a final response only when the same interval contains preceding reasoning, progress, or tools. Assert no divider for a direct answer and no divider caused by work in a previous user interval.

- [ ] **Step 4: Implement reasoning as a hard group boundary**

Simplify `buildDockWorkingTimeline` so only contiguous tool segments enter tool groups. Assistant, reasoning, user, status, context, and review-decision segments flush the current group and render flat.

- [ ] **Step 5: Derive final assistant entries per interval**

In `DockTranscript`, scan ordered entries between user/review boundaries. Mark the last assistant in completed intervals as final; in the current interval mark it final only when `active` is false. Pass an explicit assistant presentation tone to `TranscriptSegmentView`.

- [ ] **Step 6: Render hierarchy and divider**

Render progress/reasoning using muted foreground classes. Render structurally final assistant content with normal foreground. Add a subtle top border and spacing before final entries only when prior work exists in the same interval. Add stable data attributes for tests.

- [ ] **Step 7: Run focused tests and verify GREEN**

Run:
`pnpm --dir frontend exec vitest run src/components/agents/dock/__tests__/dockWorkingGroups.test.ts src/components/agents/dock/__tests__/DockTranscript.test.tsx src/components/agents/transcript/__tests__/segmentRenderers.test.tsx`

Expected: all tests pass.

### Task 3: Make tool rendering private and failure-only expandable

**Files:**
- Modify: `frontend/src/components/agents/transcript/segmentRenderers.tsx`
- Modify: `frontend/src/components/agents/transcript/TranscriptRow.tsx`
- Modify: `frontend/src/components/agents/dock/DockWorkingGroup.tsx`
- Test: `frontend/src/components/agents/transcript/__tests__/segmentRenderers.test.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx`

- [ ] **Step 1: Add failing privacy tests**

Supply distinctive secrets only in tool arguments, successful output, timestamps, and duration. Assert none appear before or after clicking any successful row and that successful rows have no disclosure button.

- [ ] **Step 2: Add failing error-disclosure tests**

Assert a failed row starts collapsed, is the only tool row with `aria-expanded`, and reveals only the error after click. Cover missing/malformed error with a short fallback.

- [ ] **Step 3: Remove successful tool details and all tool timing**

Delete input/output/time rendering and duration metadata from `ToolSegment`. Replace `ToolCallDetails` with an error-only body. Permit expansion only when effective status is failed, default it closed, and keep lazy mounting.

- [ ] **Step 4: Make labels payload-independent**

Ensure working-group headers and tool-row labels derive only from canonical tool identity, static presentation metadata, status, and counts. Add assertions that argument/output-only secret strings never enter the header or row label.

- [ ] **Step 5: Put chevrons immediately after labels**

Change `TranscriptRow` and `DockWorkingGroup` flex ordering so the chevron follows the label directly, with flexible space and optional metadata after it. Add stable DOM assertions for reasoning, failed tools, and work-group headers.

- [ ] **Step 6: Align the working disclosure styling**

Use a muted, lightweight header and ordered expanded body without exposing payloads. Preserve independent manual expansion and automatic open-while-running/collapse-on-completion behavior.

- [ ] **Step 7: Run focused tests and verify GREEN**

Run the Task 2 command and expect all tests to pass.

### Task 4: Integration verification

**Files:**
- Test: `frontend/src/components/agents/__tests__/AskAgentsDock.test.tsx`
- Test: all modified test files above

- [ ] **Step 1: Add optimistic-echo reconciliation regressions**

In `AskAgentsDock.test.tsx`, cover an empty chat and an ongoing conversation. Resolve `sendMessage` with an `accepted_message` carrying the submitted `client_message_id`; assert the message text is never present in both `DockUserMessage` and the persisted transcript in the same render.

- [ ] **Step 2: Hide matched optimistic echoes synchronously**

In `ChatView`, derive `visiblePendingEcho` from `pendingEcho` and `persistedMessages.some(message.client_message_id === pendingEcho.id)`. Use the derived value for rendering and presentation state so the durable row replaces the echo in one render, while keeping the existing state cleanup for lifecycle hygiene.

- [ ] **Step 3: Update the root dock integration fixture**

Assert completed production-shaped history preserves multiple ordered disclosures and a flat, separated final response without live indicators or payload details.

- [ ] **Step 4: Run all relevant dock/transcript tests**

Run:
`pnpm --dir frontend exec vitest run src/components/agents/dock/__tests__ src/components/agents/transcript/__tests__ src/components/agents/__tests__/AskAgentsDock.test.tsx`

Expected: all tests pass with zero failures.

- [ ] **Step 5: Run changed-file lint**

Run `pnpm --dir frontend exec eslint` against every modified `.ts`/`.tsx` file. Expected: exit 0.

- [ ] **Step 6: Run production build**

Run: `pnpm --dir frontend build`

Expected: TypeScript and Vite production build exit 0.

- [ ] **Step 7: Inspect the final diff**

Run: `git diff --check && git status --short && git diff --stat`

Expected: no whitespace errors and only scoped transcript/spec/plan changes.

- [ ] **Step 8: Commit and push**

Commit with `fix: preserve ask agent execution timeline`, pull `origin/waqar-fixes` with fast-forward only, and push `waqar-fixes`.
