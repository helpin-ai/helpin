# Ask Agent Efficient Execution Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Keep Ask Agent as the broad flash-model executor while giving it a single evidence-driven operating contract and compacting routine progress prose only in the root Ask chat.

**Architecture:** The backend always appends one current, product-owned Ask execution contract after normalizing any exact current copy out of the saved prompt. The frontend keeps runtime storage and streaming unchanged, then optionally compacts normalized assistant segments at presentation time; only root `ChatView` opts in, while tools, user turns, approval decisions, child views, and full run views remain unchanged.

**Tech Stack:** Go 1.24 service/agent contract code and tests; React 19, TypeScript, Vitest, and the existing normalized transcript pipeline.

---

### Task 1: Make the Ask execution contract authoritative and efficient

**Files:**
- Modify: `server/internal/agentcontract/skill_catalog.go:267-299`
- Modify: `server/internal/service/agent_presets.go:834-885`
- Test: `server/internal/service/agent_system_prompts_test.go`
- Test: `server/internal/service/agent_runtime_coding_delegation_test.go`
- Test: `server/internal/service/agent_policy_test.go`

- [ ] **Step 1: Write failing tests for the new effective contract**

Add table-driven assertions that `runtimeAgentFromHelpinAgent` produces an effective Ask prompt containing:

```go
required := []string{
    "Begin with the smallest targeted action",
    "After every tool result",
    "resolve a material uncertainty",
    "Do not explore merely to build a complete picture",
    "Distinguish confirmed findings",
    "Do not announce routine tool calls",
}
```

Cover the managed prompt, a custom prompt, and a stale prompt containing `## Required Ask Agent execution policy`. Assert the current version marker occurs exactly once after repeated effective-prompt assembly. Update preset tests to require the role/domain rules but reject duplicated generic execution language in `askAgentSystemPrompt()`.

- [ ] **Step 2: Run the focused Go tests and verify RED**

Run:

```bash
(cd server && go test ./internal/service -run 'Test(ManagedAskAgent|RuntimeAgentFromHelpinAgent.*Ask|AskAgent)' -count=1)
```

Expected: FAIL because the new contract/version behavior is absent and generic execution rules still live in the preset prompt.

- [ ] **Step 3: Replace the product-owned contract**

In `skill_catalog.go`, replace the existing policy with a versioned constant whose content implements the approved contract. Use a stable marker such as:

```go
const askAgentExecutionPolicyMarker = "## Required Ask Agent execution policy v2"
```

Change `EnsureAskAgentExecutionPolicy` to:

```go
func EnsureAskAgentExecutionPolicy(presetKey, prompt string) string {
    prompt = strings.TrimSpace(prompt)
    if strings.TrimSpace(presetKey) != model.AgentPresetAskAgent {
        return prompt
    }
    prompt = strings.TrimSpace(strings.ReplaceAll(prompt, askAgentExecutionPolicy, ""))
    if prompt == "" {
        return askAgentExecutionPolicy
    }
    return prompt + "\n\n" + askAgentExecutionPolicy
}
```

The policy must retain required repository selector/pagination rules while adding the smallest-action, post-result reassessment, evidence, stopping, planning, communication, and bounded-delegation rules from the spec. Do not add a numerical tool-step limit.

- [ ] **Step 4: Remove generic policy duplication from the preset prompt**

Keep page context, references, read-only repository boundary, CRM prerequisites, approval behavior, orchestration mechanics, child results, agent creation, and Helpin-link formatting in `askAgentSystemPrompt()`. Remove the generic direct-work, planning, exploration, narration, stopping, and delegation policy now owned by `EnsureAskAgentExecutionPolicy`.

- [ ] **Step 5: Run focused tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 6: Commit Task 1**

```bash
git add server/internal/agentcontract/skill_catalog.go server/internal/service/agent_presets.go server/internal/service/agent_system_prompts_test.go server/internal/service/agent_runtime_coding_delegation_test.go server/internal/service/agent_policy_test.go
git commit -m "fix: tighten ask agent execution policy"
```

### Task 2: Add optional assistant-progress compaction to normalized transcripts

**Files:**
- Modify: `frontend/src/components/agents/transcript/segments.ts`
- Test: `frontend/src/components/agents/transcript/__tests__/segments.test.ts`

- [ ] **Step 1: Write failing pure segment tests**

Extend `CollectSegmentsOptions` in the test's desired API with `compactAssistantProgress: true`. Add tests proving:

```text
user → assistant progress → tool → assistant progress → tool → assistant final
becomes
user → tool → tool → assistant final
```

Also cover:

- two user turns retain one assistant response per turn;
- a `review_decision` creates the same boundary as a user message;
- cumulative live input keeps the pre-approval prompt before the decision and only the latest resumed progress after it;
- a clarification prompt remains visible as the latest response while waiting;
- a child-launch confirmation remains visible until a later response supersedes it;
- failed and cancelled terminal-shaped inputs retain their latest assistant prose without inventing a synthetic final response;
- all tool segments remain;
- `compactAssistantProgress` omitted/false preserves the existing full transcript.

- [ ] **Step 2: Run the focused Vitest file and verify RED**

Run:

```bash
pnpm --dir frontend exec vitest run src/components/agents/transcript/__tests__/segments.test.ts
```

Expected: FAIL because the option and compaction behavior do not exist.

- [ ] **Step 3: Implement boundary-aware compaction**

Add the option:

```ts
export interface CollectSegmentsOptions {
  includeLive: boolean;
  include?: ReadonlySet<TranscriptSegmentKind>;
  leadingContext?: CodingSessionTranscriptMessage | null;
  compactAssistantProgress?: boolean;
}
```

Add a pure helper that processes each interval independently and retains only its last assistant segment:

```ts
function compactAssistantProgress(segments: TranscriptSegment[]): TranscriptSegment[] {
  const keep = new Array<boolean>(segments.length).fill(true);
  let latestAssistantIndex: number | null = null;
  for (let index = 0; index < segments.length; index += 1) {
    const segment = segments[index];
    if (segment.kind === 'user' || segment.kind === 'review_decision') {
      latestAssistantIndex = null;
      continue;
    }
    if (segment.kind !== 'assistant') continue;
    if (latestAssistantIndex !== null) keep[latestAssistantIndex] = false;
    latestAssistantIndex = index;
  }
  return segments.filter((_, index) => keep[index]);
}
```

Apply it to both non-live and live return values only when requested. During live reconciliation, calculate the settled-tail index using the latest `user` **or** `review_decision`, not only the latest user, when compaction is enabled. Preserve existing default behavior byte-for-byte when the option is false.

- [ ] **Step 4: Run focused tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit Task 2**

```bash
git add frontend/src/components/agents/transcript/segments.ts frontend/src/components/agents/transcript/__tests__/segments.test.ts
git commit -m "feat: compact ask agent progress segments"
```

### Task 3: Opt in only the root Ask chat and verify the live handoff

**Files:**
- Modify: `frontend/src/components/agents/dock/DockTranscript.tsx:127-220`
- Modify: `frontend/src/components/agents/dock/ChatView.tsx:542-549`
- Test: `frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx`
- Test: `frontend/src/components/agents/__tests__/AskAgentsDock.test.tsx`
- Test: `frontend/src/components/agents/dock/__tests__/ExecutionStrip.test.tsx`

- [ ] **Step 1: Write failing component and call-site tests**

Add a `compactAssistantProgress?: boolean` prop to the desired `DockTranscript` API in tests. Assert:

- with the prop true, tool activity and the latest assistant response render while earlier progress prose does not;
- changing `active` from true to false with equivalent persisted data keeps the final response and tool rows visible;
- without the prop, all assistant messages still render;
- the root Ask dock test observes compacted output;
- a full `DockRunView` case renders multiple assistant segments unchanged;
- an `ExecutionStrip` child-run case renders multiple assistant segments unchanged.

- [ ] **Step 2: Run focused component tests and verify RED**

Run:

```bash
pnpm --dir frontend exec vitest run \
  src/components/agents/dock/__tests__/DockTranscript.test.tsx \
  src/components/agents/dock/__tests__/ExecutionStrip.test.tsx \
  src/components/agents/__tests__/AskAgentsDock.test.tsx
```

Expected: FAIL because the prop is missing and `ChatView` does not opt in.

- [ ] **Step 3: Wire the option through the root chat only**

Add `compactAssistantProgress = false` to `DockTranscript`, pass it to `collectSegments`, and set it only here:

```tsx
<DockTranscript
  stream={transformed.stream}
  active={runActive}
  workspaceId={workspaceId}
  fallbackActor={streamController.session?.triggered_by_user}
  subAgentRuns={subAgentTimelineItems}
  compactAssistantProgress
/>
```

Do not change either `DockRunView` or `ExecutionStrip` call site.

- [ ] **Step 4: Run focused component tests and verify GREEN**

Run the command from Step 2. Expected: PASS.

- [ ] **Step 5: Commit Task 3**

```bash
git add frontend/src/components/agents/dock/DockTranscript.tsx frontend/src/components/agents/dock/ChatView.tsx frontend/src/components/agents/dock/__tests__/DockTranscript.test.tsx frontend/src/components/agents/dock/__tests__/ExecutionStrip.test.tsx frontend/src/components/agents/__tests__/AskAgentsDock.test.tsx
git commit -m "fix: coalesce ask chat progress messages"
```

### Task 4: Regression verification and branch delivery

**Files:**
- Verify only; modify production files only if a failing relevant regression requires it.

- [ ] **Step 1: Rebase onto the current remote branch**

```bash
git pull --rebase origin waqar-fixes
```

Expected: local implementation commits are replayed cleanly on the latest `origin/waqar-fixes`. Resolve only overlaps belonging to this work; stop for user direction if unrelated changes conflict.

- [ ] **Step 2: Run backend agent tests**

```bash
(cd server && go test ./internal/agentcontract ./internal/service -count=1)
```

Expected: PASS.

- [ ] **Step 3: Run frontend transcript tests**

```bash
pnpm --dir frontend exec vitest run \
  src/components/agents/transcript/__tests__/segments.test.ts \
  src/components/agents/transcript/__tests__/segmentRenderers.test.tsx \
  src/components/agents/dock/__tests__/DockTranscript.test.tsx \
  src/components/agents/dock/__tests__/ExecutionStrip.test.tsx \
  src/components/agents/__tests__/AskAgentsDock.test.tsx
```

Expected: PASS.

- [ ] **Step 4: Run static verification**

```bash
git diff --check origin/waqar-fixes...HEAD
pnpm --dir frontend exec tsc -b --pretty false
```

Expected: `git diff --check` passes. TypeScript should pass once the worktree's declared `konva` and `react-konva` dependencies are installed; if the existing missing-module condition remains, report it separately with the exact diagnostics.

- [ ] **Step 5: Confirm scope**

Review `git diff --stat origin/waqar-fixes...HEAD`, `git diff origin/waqar-fixes...HEAD`, and `git status --short`. Confirm there is no model change, no new tool-step limit, no runtime persistence/projection change, and no opt-in outside root Ask `ChatView`.

- [ ] **Step 6: Push the completed branch**

```bash
git push origin waqar-fixes
```

Expected: the new Task 1-3 commits are present on `origin/waqar-fixes`.
