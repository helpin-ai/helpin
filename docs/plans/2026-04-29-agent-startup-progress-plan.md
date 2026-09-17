# Agent Startup Progress Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Move agent startup progress into the left transcript section and show the right-side Agent Plan panel only after a real agent plan is available.

**Architecture:** Keep `update_plan` as the only source of the right-side plan artifact. Use existing run status and `execution_stage` to render deterministic startup progress in the transcript while the backend prepares context/workspace/runtime. Add finer backend stage updates at existing checkpoints without changing schema or agent-run lifecycle semantics.

**Tech Stack:** Go 1.24, Temporal activities, React 19, TypeScript, TanStack Query/WebSocket-backed coding session state, Vitest/Testing Library.

---

## File Structure

- Modify `server/internal/temporalapp/activities.go`
  - Add stage updates around existing startup checkpoints: loading context, preparing workspace, syncing branch, starting runtime.
  - Do not change run status transitions, Temporal workflow shape, or persisted schema.
- Modify `frontend/src/components/pm/CodingSession/CodingPlanPanel.tsx`
  - Return `null` when no plan exists and the run is non-terminal.
  - Preserve existing terminal empty-plan messaging for completed/failed/cancelled runs.
- Modify `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`
  - Add transcript-visible startup progress rows derived from `session.status` and `session.execution_stage`.
  - Keep system/developer prompt details available but avoid making the system prompt the first prominent runtime item during startup.
- Modify or add tests near:
  - `frontend/src/components/pm/CodingSession/__tests__/CodingPlanPanel.test.tsx`
  - `frontend/src/components/pm/CodingSession/__tests__/CodingTranscriptPane.test.tsx` if one exists; otherwise add a focused test file for startup progress rendering.
  - `server/internal/temporalapp/activities_test.go` or a focused existing activity test if stage assertions already exist.

## Task 1: Hide Plan Panel Until Real Plan Exists

**Files:**
- Modify: `frontend/src/components/pm/CodingSession/CodingPlanPanel.tsx`
- Test: `frontend/src/components/pm/CodingSession/__tests__/CodingPlanPanel.test.tsx`

- [ ] **Step 1: Update the failing frontend test**

Change the existing no-plan running expectation from “Waiting for the agent to publish its first plan update.” to asserting the component renders nothing for a non-terminal run with `plan = null`.

Test intent:

```tsx
const { container } = render(null, 'running');
expect(container).toBeEmptyDOMElement();
```

Keep or add a separate test proving terminal no-plan behavior still renders:

```tsx
const { container } = render(null, 'completed');
expect(container.textContent).toContain('Agent ended the run without publishing a plan.');
```

- [ ] **Step 2: Run the focused test and verify it fails**

Run:

```bash
cd frontend
npm test -- CodingPlanPanel
```

Expected: FAIL because the component still renders the waiting panel for running runs.

- [ ] **Step 3: Implement minimal component change**

In `CodingPlanPanel`, after computing `terminal` and `hasPlan`, add:

```tsx
if (!hasPlan && !terminal) {
  return null;
}
```

Do not change the existing rendering path for real plans or terminal no-plan states.

- [ ] **Step 4: Run the focused test**

Run:

```bash
cd frontend
npm test -- CodingPlanPanel
```

Expected: PASS.

## Task 2: Render Startup Progress In The Left Transcript

**Files:**
- Modify: `frontend/src/components/pm/CodingSession/CodingTranscriptPane.tsx`
- Test: add or modify `frontend/src/components/pm/CodingSession/__tests__/CodingTranscriptPane.test.tsx`

- [ ] **Step 1: Write tests for startup progress**

Add tests that render the transcript with no assistant/tool activity and a running session:

```tsx
session.status = 'running';
session.execution_stage = 'preparing_workspace';
```

Assert the left transcript contains a startup progress row such as `Preparing workspace`.

Add another test where a live assistant segment exists and assert the startup row still does not duplicate or obscure the live assistant/tool activity.

- [ ] **Step 2: Run the focused test and verify it fails**

Run:

```bash
cd frontend
npm test -- CodingTranscriptPane
```

Expected: FAIL because no startup progress row exists yet.

- [ ] **Step 3: Add a small stage-label helper**

In `CodingTranscriptPane.tsx`, add a local helper near other formatting helpers:

```tsx
function startupProgressLabel(stage?: string | null, status?: string | null) {
  if (status === 'queued') return 'Queued';
  switch ((stage ?? '').trim()) {
    case 'preparing':
    case 'preparing_target':
      return 'Preparing target';
    case 'loading_context':
      return 'Loading context';
    case 'preparing_workspace':
      return 'Preparing workspace';
    case 'syncing_branch':
      return 'Syncing branch';
    case 'starting':
    case 'starting_runtime':
    case 'native_sdk_starting':
      return 'Starting agent';
    case 'assistant_started':
    case 'thinking':
      return 'Thinking';
    default:
      return status === 'running' ? 'Starting agent' : null;
  }
}
```

Use existing icon and muted styling patterns from the transcript pane. Do not introduce a new dependency.

- [ ] **Step 4: Insert a startup virtual item before prompt/debug content**

Add a virtual item like `{ kind: 'startup-progress'; label }` when:

- the run is `queued` or `running`
- there is no visible assistant/tool content yet, or the current stage is still a startup stage
- the label helper returns a non-empty label

Render it as a compact left-aligned status row. The row should look like runtime activity, not an assistant message bubble.

- [ ] **Step 5: De-emphasize the system prompt during startup**

Keep `PromptTranscriptCard` available, but ensure startup progress appears before it. If the prompt card currently dominates the first viewport, make the prompt card collapsed and visually secondary during startup. Do not remove developer prompt rendering.

- [ ] **Step 6: Run focused transcript tests**

Run:

```bash
cd frontend
npm test -- CodingTranscriptPane
```

Expected: PASS.

## Task 3: Emit More Precise Backend Startup Stages

**Files:**
- Modify: `server/internal/temporalapp/activities.go`
- Test: `server/internal/temporalapp/prepare_run_activity_test.go` and/or `server/internal/temporalapp/activities_test.go`

- [ ] **Step 1: Add backend test coverage for stage progression**

Add focused assertions around existing activity behavior. The minimum non-breaking coverage:

- `PrepareRunActivity` still marks the run `running`.
- Initial stage remains compatible as `preparing` during prepare.
- `ExecuteRunActivity` publishes or persists stage updates before runtime execution when dependencies are stubbed.

If fully testing `ExecuteRunActivity` is too heavy with existing fixtures, add coverage to the smallest helper boundary available and rely on frontend fallback for unknown stages.

- [ ] **Step 2: Run the backend focused tests and verify current baseline**

Run:

```bash
cd server
go test ./internal/temporalapp -run 'TestPrepareRunActivity|Test.*Stage' -count=1
```

Expected: current tests pass before implementation unless a new failing test was added for the finer stages.

- [ ] **Step 3: Add stage updates at existing checkpoints**

In `ExecuteRunActivity`, set `state.run.ExecutionStage` and notify via existing persistence mechanisms at checkpoints:

- before `resolvePlanningRunInput` or immediately after loading state: `loading_context`
- before `preparePlanningRepository`: `loading_context`
- before `PrepareWorkspaceForRun` / `PrepareWorkspace`: `preparing_workspace`
- before `checkoutRunRef` / `syncBaseIntoWorkingBranch`: `syncing_branch`
- immediately before `adapter.Execute`: `starting_runtime`

Prefer using an existing helper if available. If not, add a small local helper in `activities.go`:

```go
func (a *AgentRunActivities) updateRunExecutionStage(ctx context.Context, run *model.AgentRun, stage string) error {
  now := time.Now()
  run.ExecutionStage = strPtr(stage)
  run.LastHeartbeatAt = &now
  if err := a.runRepo.UpdateStage(ctx, run.WorkspaceID, run.ID, stage, &now); err != nil {
    return err
  }
  a.runRepo.Notify(ctx, run)
  return nil
}
```

Use it only after the run has been persisted by `PrepareRunActivity`; do not replace existing status transitions.

- [ ] **Step 4: Preserve existing runtime stages**

Do not rename current runtime heartbeats such as `native_sdk_starting`, `assistant_started`, `tool_<name>`, or terminal stages. The frontend helper should tolerate both old and new names.

- [ ] **Step 5: Run backend tests**

Run:

```bash
cd server
go test ./internal/temporalapp -count=1
```

Expected: PASS.

## Task 4: Wire Session Stage Into Transcript Rendering Safely

**Files:**
- Modify: whichever parent component passes `session` into `CodingTranscriptPane`, if needed.
- Search with: `rg -n "CodingTranscriptPane" frontend/src/components frontend/src/pages`

- [ ] **Step 1: Confirm existing props include session status and stage**

Check `CodingTranscriptPane` props and parent usage. If `session` already includes `status` and `execution_stage`, no parent change is needed.

- [ ] **Step 2: Add minimal prop only if needed**

If the pane does not receive `execution_stage`, add an optional prop:

```tsx
executionStage?: string | null;
runStatus?: AgentRunStatus | null;
```

Prefer using existing `session` data to avoid unnecessary prop churn.

- [ ] **Step 3: Run TypeScript build**

Run:

```bash
cd frontend
npm run build
```

Expected: PASS.

## Task 5: Regression Verification

**Files:**
- No additional files unless tests reveal issues.

- [ ] **Step 1: Run targeted frontend tests**

Run:

```bash
cd frontend
npm test -- CodingPlanPanel CodingTranscriptPane codingSessionStream
```

Expected: PASS.

- [ ] **Step 2: Run backend targeted tests**

Run:

```bash
cd server
go test ./internal/temporalapp ./internal/service -count=1
```

Expected: PASS.

- [ ] **Step 3: Manual verification in browser**

Start the app if needed:

```bash
cd frontend
npm run dev
```

Start backend separately if needed:

```bash
cd server
go run ./cmd/api
```

Manual checks:

- Start an agent run.
- The left transcript immediately shows startup progress.
- The system prompt does not dominate the initial view.
- The right Agent Plan panel is hidden until the first real plan update.
- When `update_plan` arrives, the right panel appears and behaves as before.
- Terminal runs with no plan still explain that no plan was published.

## Safety Notes

- No database migration is needed; `execution_stage` already exists.
- Do not change agent prompt semantics or force the model to call `update_plan` earlier.
- Do not synthesize fake right-side plan steps from startup stages.
- Keep frontend fallbacks for existing stage names so in-flight or older runs still render sensibly.
- Treat backend stage names as additive. Existing consumers should continue to work with `status`, `pause_reason`, and old `execution_stage` values.
