# PM Automation Access And Task Autosave Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** PM task, board, and list views can consume PM-relevant automation data without requiring direct Automation module access, and failed task autosaves no longer loop forever.

**Architecture:** Keep `/api/automation/*` as the full Automation module surface gated by `ModuleAutomation`. Move PM-context reads to existing `/api/pm/*` endpoints guarded by PM permissions, and make task autosave treat persistent failures as blocked pending changes instead of an automatic retry trigger. Backend task update should return correct HTTP status for forbidden errors and avoid rejecting unrelated field edits because of pre-existing workflow/state inconsistency.

**Tech Stack:** Go 1.24, Chi, GORM, React 19, Vite, TypeScript, TanStack Query, Vitest.

---

## File Map

- Modify: `frontend/src/lib/services/automationRuleService.ts`
  - Responsibility: PM-facing automation rule service wrapper.
  - Change it to call `/pm/automation-rules` directly instead of delegating to `/automation/flows`.
- Modify: `frontend/src/lib/services/agentService.ts`
  - Responsibility: PM-facing agent service wrapper.
  - Change list/get/usage/create/update/delete methods used from PM contexts to call `/pm/agents` endpoints instead of `/automation/agents`.
- Modify: `frontend/src/hooks/queries/useAgents.ts`
  - Responsibility: shared agents query hook.
  - Make it use `agentService.list`, not `automationService.listAgents`, so PM screens do not require Automation module access.
- Modify: `frontend/src/hooks/queries/useAutomationRules.ts`
  - Responsibility: PM automation-rule query hook currently imported by PM components.
  - Make it use `automationRuleService`, not `automationService`.
- Modify: `frontend/src/hooks/queries/useAutomation.ts`
  - Responsibility: full Automation module hooks.
  - Leave these on `/automation/*`; do not change Automation pages to PM endpoints.
- Modify: `frontend/src/components/pm/TaskDetailPanel.tsx`
  - Responsibility: task detail UI and autosave.
  - Add failed-patch blocking so one persistent `400` does not retry forever.
- Create: `frontend/src/components/pm/task-detail/taskAutosaveFailure.ts`
  - Responsibility: small pure helper for stable patch signature/blocking behavior.
- Create: `frontend/src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts`
  - Responsibility: pure unit coverage for failed-patch blocking.
- Modify: `server/internal/handler/pm_task.go`
  - Responsibility: PM task HTTP status mapping.
  - Return `403` for `model.ErrForbidden` instead of `400`.
- Modify: `server/internal/service/pm_task.go`
  - Responsibility: PM task update validation.
  - Only validate workflow/state ownership when `workflow_id` or `workflow_state_id` is changed.
- Modify: `server/internal/service/pm_task_extended_test.go`
  - Responsibility: service regression tests for task update behavior.
  - Add test for unrelated update with pre-existing workflow/state mismatch.
- Optional create: `server/internal/handler/pm_task_test.go`
  - Responsibility: handler status-code test if no suitable existing handler test exists.

---

### Task 1: Prove PM Automation Reads Should Not Require Automation Module

**Files:**
- Test: `frontend/src/hooks/queries/__tests__/useAutomationRules.test.tsx`
- Test: `frontend/src/hooks/queries/__tests__/useAgents.test.tsx`

- [ ] **Step 1: Write a failing test for PM automation rules hook**

Create `frontend/src/hooks/queries/__tests__/useAutomationRules.test.tsx`.

Test intent:
- Render `useAutomationRulesByWorkflow('ws-1', 'wf-1')` inside a `QueryClientProvider`.
- Mock `automationRuleService.listByWorkflow`.
- Assert the hook calls `automationRuleService.listByWorkflow('ws-1', 'wf-1')`.
- Do not mock or expect `automationService.listFlowsByWorkflow`.

Suggested test structure:

```tsx
// @vitest-environment jsdom
import { act } from 'react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createRoot } from 'react-dom/client'
import { afterEach, describe, expect, it, vi } from 'vitest'

const captured = {
  rules: null as ReturnType<typeof import('@/hooks/queries/useAutomationRules').useAutomationRulesByWorkflow> | null,
}

vi.mock('@/lib/services/automationRuleService', () => ({
  automationRuleService: {
    listByWorkflow: vi.fn(),
  },
}))

import { automationRuleService } from '@/lib/services/automationRuleService'
import { useAutomationRulesByWorkflow } from '@/hooks/queries/useAutomationRules'

Object.assign(globalThis, { IS_REACT_ACT_ENVIRONMENT: true })

function Harness() {
  captured.rules = useAutomationRulesByWorkflow('ws-1', 'wf-1')
  return null
}

describe('useAutomationRulesByWorkflow', () => {
  afterEach(() => {
    captured.rules = null
    vi.clearAllMocks()
  })

  it('uses the PM automation-rule service for PM workflow metadata', async () => {
    vi.mocked(automationRuleService.listByWorkflow).mockResolvedValue({
      data: [],
      error: null,
      status: 200,
    } as never)

    const client = new QueryClient()
    const container = document.createElement('div')
    document.body.appendChild(container)
    const root = createRoot(container)

    await act(async () => {
      root.render(
        <QueryClientProvider client={client}>
          <Harness />
        </QueryClientProvider>,
      )
    })

    await act(async () => {
      await captured.rules?.refetch()
    })

    expect(automationRuleService.listByWorkflow).toHaveBeenCalledWith('ws-1', 'wf-1')
    root.unmount()
    container.remove()
  })
})
```

- [ ] **Step 2: Write a failing test for `useAgents`**

Create `frontend/src/hooks/queries/__tests__/useAgents.test.tsx`.

Test intent:
- Render `useAgents('ws-1')`.
- Mock `agentService.list`.
- Assert it calls `agentService.list('ws-1')`.
- This proves PM consumers of `useAgents` no longer call `/automation/agents`.

- [ ] **Step 3: Run tests to verify they fail**

Run:

```bash
pnpm --dir frontend test src/hooks/queries/__tests__/useAutomationRules.test.tsx src/hooks/queries/__tests__/useAgents.test.tsx
```

Expected:
- Both tests fail because the hooks currently import/call `automationService`.

- [ ] **Step 4: Implement PM-scoped service calls**

Modify `frontend/src/lib/services/automationRuleService.ts`:

```ts
import { api } from '../api';
import type { AutomationRule, CreateAutomationRuleRequest, UpdateAutomationRuleRequest } from '../pmTypes';

const qs = (workspaceId: string) => `?workspace_id=${encodeURIComponent(workspaceId)}`;

export const automationRuleService = {
  list: (workspaceId: string) =>
    api.get<AutomationRule[]>(`/pm/automation-rules${qs(workspaceId)}`),

  listByWorkflow: (workspaceId: string, workflowId: string) =>
    api.get<AutomationRule[]>(`/pm/automation-rules${qs(workspaceId)}&workflow_id=${encodeURIComponent(workflowId)}`),

  create: (workspaceId: string, data: CreateAutomationRuleRequest) =>
    api.post<AutomationRule>(`/pm/automation-rules${qs(workspaceId)}`, data),

  get: (workspaceId: string, ruleId: string) =>
    api.get<AutomationRule>(`/pm/automation-rules/${ruleId}${qs(workspaceId)}`),

  update: (workspaceId: string, ruleId: string, data: UpdateAutomationRuleRequest) =>
    api.put<AutomationRule>(`/pm/automation-rules/${ruleId}${qs(workspaceId)}`, data),

  remove: (workspaceId: string, ruleId: string) =>
    api.del(`/pm/automation-rules/${ruleId}${qs(workspaceId)}`),
};
```

Modify `frontend/src/hooks/queries/useAutomationRules.ts`:

```ts
import { automationRuleService } from '@/lib/services/automationRuleService'
```

Use `automationRuleService.list` and `automationRuleService.listByWorkflow`.

Modify `frontend/src/lib/services/agentService.ts`:

```ts
list: (workspaceId: string) =>
  api.get<Agent[]>(`/pm/agents${qs(workspaceId)}`),
get: (workspaceId: string, id: string) =>
  api.get<Agent>(`/pm/agents/${id}${qs(workspaceId)}`),
getUsage: (workspaceId: string, id: string) =>
  api.get<AgentTriggerUsageSummary>(`/pm/agents/${id}/usage${qs(workspaceId)}`),
create: (workspaceId: string, payload: CreateAgentRequest) =>
  api.post<Agent>(`/pm/agents${qs(workspaceId)}`, payload),
update: (workspaceId: string, id: string, payload: UpdateAgentRequest) =>
  api.put<Agent>(`/pm/agents/${id}${qs(workspaceId)}`, payload),
delete: (workspaceId: string, id: string) =>
  api.del(`/pm/agents/${id}${qs(workspaceId)}`),
```

Add `Agent` and `AgentTriggerUsageSummary` to the `agentService.ts` type import list if missing.

Modify `frontend/src/hooks/queries/useAgents.ts`:

```ts
import { agentService } from '@/lib/services/agentService'
```

Use `agentService.list(workspaceId)`.

- [ ] **Step 5: Run tests to verify they pass**

Run:

```bash
pnpm --dir frontend test src/hooks/queries/__tests__/useAutomationRules.test.tsx src/hooks/queries/__tests__/useAgents.test.tsx
```

Expected:
- PASS.

- [ ] **Step 6: Search for accidental PM usage of `/automation/*`**

Run:

```bash
rg "automationService\\.listAgents|automationService\\.listFlowsByWorkflow|/automation/agents|/automation/flows" frontend/src/components/pm frontend/src/hooks/queries frontend/src/lib/services
```

Expected:
- PM components/hooks should no longer call Automation-module endpoints.
- `frontend/src/hooks/queries/useAutomation.ts` and `frontend/src/lib/services/automationService.ts` may still use `/automation/*` because those are full Automation module surfaces.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/services/automationRuleService.ts frontend/src/lib/services/agentService.ts frontend/src/hooks/queries/useAutomationRules.ts frontend/src/hooks/queries/useAgents.ts frontend/src/hooks/queries/__tests__/useAutomationRules.test.tsx frontend/src/hooks/queries/__tests__/useAgents.test.tsx
git commit -m "fix: use pm scoped automation data in pm views"
```

---

### Task 2: Stop Failed Task Autosave From Retrying Forever

**Files:**
- Create: `frontend/src/components/pm/task-detail/taskAutosaveFailure.ts`
- Test: `frontend/src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts`
- Modify: `frontend/src/components/pm/TaskDetailPanel.tsx`

- [ ] **Step 1: Write failing helper tests**

Create `frontend/src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts`.

Test cases:
- Same patch object with different key order gets the same signature.
- A patch is blocked only when its signature equals the failed signature.
- Changing a value produces a different signature and is not blocked.

Suggested assertions:

```ts
import { describe, expect, it } from 'vitest'
import { getTaskPatchSignature, isBlockedTaskPatch } from '@/components/pm/task-detail/taskAutosaveFailure'

describe('task autosave failure helpers', () => {
  it('creates stable signatures independent of key order', () => {
    expect(getTaskPatchSignature({ estimate: 3, priority: 'high' } as never))
      .toBe(getTaskPatchSignature({ priority: 'high', estimate: 3 } as never))
  })

  it('blocks only the failed patch signature', () => {
    const failed = getTaskPatchSignature({ estimate: 3 } as never)

    expect(isBlockedTaskPatch({ estimate: 3 } as never, failed)).toBe(true)
    expect(isBlockedTaskPatch({ estimate: 5 } as never, failed)).toBe(false)
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
pnpm --dir frontend test src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts
```

Expected:
- FAIL because helper file does not exist.

- [ ] **Step 3: Implement helper**

Create `frontend/src/components/pm/task-detail/taskAutosaveFailure.ts`:

```ts
import type { UpdateTaskRequest } from '@/lib/pmTypes'

function stableStringify(value: unknown): string {
  if (Array.isArray(value)) {
    return `[${value.map(stableStringify).join(',')}]`
  }
  if (value && typeof value === 'object') {
    const entries = Object.entries(value as Record<string, unknown>)
      .sort(([left], [right]) => left.localeCompare(right))
    return `{${entries.map(([key, entry]) => `${JSON.stringify(key)}:${stableStringify(entry)}`).join(',')}}`
  }
  return JSON.stringify(value)
}

export function getTaskPatchSignature(patch: UpdateTaskRequest): string {
  return stableStringify(patch)
}

export function isBlockedTaskPatch(patch: UpdateTaskRequest, blockedSignature: string | null): boolean {
  return blockedSignature !== null && getTaskPatchSignature(patch) === blockedSignature
}
```

- [ ] **Step 4: Run helper test to verify it passes**

Run:

```bash
pnpm --dir frontend test src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts
```

Expected:
- PASS.

- [ ] **Step 5: Integrate into `TaskDetailPanel` autosave**

Modify imports in `frontend/src/components/pm/TaskDetailPanel.tsx`:

```ts
import { getTaskPatchSignature, isBlockedTaskPatch } from '@/components/pm/task-detail/taskAutosaveFailure';
```

Add a ref near existing refs:

```ts
const blockedAutosavePatchSignatureRef = useRef<string | null>(null);
```

Clear the blocked signature when the user makes a materially different patch:

```ts
const queuePatch = (patch: UpdateTaskRequest) => {
  setPendingPatch((current) => {
    const next = { ...current, ...patch };
    if (!isBlockedTaskPatch(next, blockedAutosavePatchSignatureRef.current)) {
      blockedAutosavePatchSignatureRef.current = null;
    }
    return next;
  });
};
```

Update the autosave effect guard:

```ts
const flushablePatch = getFlushablePendingTaskPatch(pendingPatch, descriptionPendingUploads);
if (saving || !flushablePatch || isBlockedTaskPatch(flushablePatch, blockedAutosavePatchSignatureRef.current)) {
  return;
}
```

Inside the timer, use the flushable patch and block on failure:

```ts
const patch = flushablePatch;
...
if (error || !data) {
  setSaveError(error ?? 'Failed to save changes');
  blockedAutosavePatchSignatureRef.current = getTaskPatchSignature(patch);
  setPendingPatch((current) => ({ ...patch, ...current }));
} else {
  blockedAutosavePatchSignatureRef.current = null;
  ...
}
```

Do not discard the user’s unsaved values; only stop retrying the identical failed payload.

- [ ] **Step 6: Run focused frontend tests**

Run:

```bash
pnpm --dir frontend test src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts src/components/pm/task-detail/__tests__/taskPendingPatch.test.ts
```

Expected:
- PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/components/pm/TaskDetailPanel.tsx frontend/src/components/pm/task-detail/taskAutosaveFailure.ts frontend/src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts
git commit -m "fix: stop task autosave retry loop"
```

---

### Task 3: Return Correct Status For Forbidden PM Task Updates

**Files:**
- Modify: `server/internal/handler/pm_task.go`
- Test: `server/internal/handler/pm_task_test.go` or existing handler test file if present.

- [ ] **Step 1: Search existing handler test patterns**

Run:

```bash
rg "PMTaskHandler|ErrForbidden|StatusForbidden|writeError" server/internal/handler server/internal/router -n
```

Expected:
- Identify whether a suitable handler test file exists.

- [ ] **Step 2: Write failing test**

If practical in existing test setup, add a handler-level test that stubs `taskService.Update` to return `&model.ErrForbidden{Message: "you do not have access to this team's resources"}` and asserts HTTP `403`.

If the handler is hard to unit test because `PMTaskHandler` stores concrete services, write a router/integration test around the service path with a non-admin actor lacking team access.

Desired behavior:
- `PUT /api/pm/tasks/{id}` returns `403`, not `400`, when service returns `model.ErrForbidden`.

- [ ] **Step 3: Run test to verify it fails**

Run the smallest applicable Go test package:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/handler -run 'Test.*PMTask.*Forbidden' -count=1
```

If the test lives elsewhere, adjust package and `-run`.

Expected:
- FAIL because handler currently writes `400` for all update service errors.

- [ ] **Step 4: Implement status mapping**

Modify `server/internal/handler/pm_task.go`.

Add import if needed:

```go
import "errors"
```

Update `Update` error handling:

```go
task, err := h.taskService.Update(r.Context(), id, req, userID)
if err != nil {
    var forbidden *model.ErrForbidden
    if errors.As(err, &forbidden) {
        writeError(w, http.StatusForbidden, forbidden.Error())
        return
    }
    writeError(w, http.StatusBadRequest, err.Error())
    return
}
```

Use existing project formatting.

- [ ] **Step 5: Run focused backend test**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/handler -run 'Test.*PMTask.*Forbidden' -count=1
```

Expected:
- PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/handler/pm_task.go server/internal/handler/pm_task_test.go
git commit -m "fix: return forbidden for inaccessible task updates"
```

If no handler test file was created because the integration test lives elsewhere, adjust `git add`.

---

### Task 4: Allow Unrelated Task Updates When Stored Workflow/State Is Already Inconsistent

**Files:**
- Modify: `server/internal/service/pm_task.go`
- Test: `server/internal/service/pm_task_extended_test.go`

- [ ] **Step 1: Write failing service regression test**

Add a test near `TestPMTaskService_Update`.

Test intent:
- Create a task normally.
- Insert a second workflow and state.
- Manually corrupt the task row so `workflow_id` remains original but `workflow_state_id` points to the second workflow’s state.
- Update an unrelated field such as `estimate` or `description`.
- Expect update succeeds.
- Then attempt to explicitly update `workflow_state_id` to an invalid state and expect rejection remains.

Suggested shape:

```go
func TestPMTaskService_UpdateAllowsUnrelatedEditWithExistingWorkflowStateMismatch(t *testing.T) {
    t.Parallel()
    env := newTaskTestEnv(t)
    ctx := context.Background()
    created := createTestTask(t, env, "Mismatched State Story")

    now := time.Now()
    otherWorkflowID := "wf-story-other"
    otherStateID := "state-story-other"
    mustExec(t, env.db, `INSERT INTO pm_workflows (id, workspace_id, name, default_state_id, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
        otherWorkflowID, env.wsID, "Other Workflow", otherStateID, now, now)
    mustExec(t, env.db, `INSERT INTO pm_workflow_states (id, workflow_id, name, state_type, position, is_default, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
        otherStateID, otherWorkflowID, "Other", "unstarted", 0, true, now, now)
    mustExec(t, env.db, `UPDATE pm_tasks SET workflow_state_id = ? WHERE id = ?`, otherStateID, created.Task.ID)

    estimate := 5
    updated, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{Estimate: &estimate}, env.userID)
    if err != nil {
        t.Fatalf("update unrelated estimate: %v", err)
    }
    if updated.Task.Estimate == nil || *updated.Task.Estimate != estimate {
        t.Fatalf("estimate = %v, want %d", updated.Task.Estimate, estimate)
    }
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run TestPMTaskService_UpdateAllowsUnrelatedEditWithExistingWorkflowStateMismatch -count=1
```

Expected:
- FAIL with `workflow_state_id must belong to workflow_id`.

- [ ] **Step 3: Implement minimal validation change**

Modify `server/internal/service/pm_task.go` in `Update`.

Current behavior validates `StateBelongsToWorkflow` unconditionally after resolving `workflowID` and `stateID`.

Change it so validation only runs if the request changes `WorkflowID` or `WorkflowStateID`:

```go
workflowOrStateChanged := req.WorkflowID != nil || req.WorkflowStateID != nil
if workflowOrStateChanged {
    ok, err := s.workflowRepo.StateBelongsToWorkflow(ctx, stateID, workflowID)
    if err != nil {
        return nil, err
    }
    if !ok {
        return nil, fmt.Errorf("workflow_state_id must belong to workflow_id")
    }
}
current.WorkflowID = workflowID
current.WorkflowStateID = stateID
```

Keep `stateChanged` logic unchanged for activity, automation, notifications, and websocket events.

- [ ] **Step 4: Add explicit invalid state-change test**

Add a second test or subtest proving bad workflow/state changes are still rejected:

```go
_, err := env.svc.Update(ctx, created.Task.ID, model.UpdateTaskRequest{
    WorkflowStateID: &otherStateID,
}, env.userID)
if err == nil || !strings.Contains(err.Error(), "workflow_state_id must belong to workflow_id") {
    t.Fatalf("expected workflow/state validation error, got %v", err)
}
```

Import `strings` if needed.

- [ ] **Step 5: Run focused service tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestPMTaskService_Update' -count=1
```

Expected:
- PASS.

- [ ] **Step 6: Commit**

```bash
git add server/internal/service/pm_task.go server/internal/service/pm_task_extended_test.go
git commit -m "fix: allow unrelated task edits with legacy state mismatch"
```

---

### Task 5: Optional Diagnostic For Existing Bad Task Rows

**Files:**
- No production code required unless the team wants a reusable script.

- [ ] **Step 1: Run read-only diagnostic query in the affected environment**

Run against the target database:

```sql
SELECT
  t.id,
  t.display_id,
  t.name,
  t.workspace_id,
  t.workflow_id,
  t.workflow_state_id,
  s.workflow_id AS state_workflow_id,
  t.team_id
FROM pm_tasks t
LEFT JOIN pm_workflow_states s ON s.id = t.workflow_state_id
WHERE s.id IS NULL
   OR s.workflow_id <> t.workflow_id
ORDER BY t.updated_at DESC
LIMIT 100;
```

Expected:
- If rows return, those tasks could have triggered unrelated update `400`s before Task 4.

- [ ] **Step 2: Decide whether to add a repair migration**

Only add a migration if real inconsistent rows are found.

Safe repair strategy:
- For rows whose `workflow_state_id` points to a valid state in another workflow, either update `workflow_id` to `s.workflow_id` or move `workflow_state_id` to the default state of `t.workflow_id`.
- Choose based on product meaning:
  - If state is the source of truth, align `workflow_id` to the state.
  - If workflow is the source of truth, move state to the workflow default.

Do not write this migration unless diagnostics prove data exists.

---

### Task 6: Full Verification

**Files:**
- No new files.

- [ ] **Step 1: Run focused frontend tests**

Run:

```bash
pnpm --dir frontend test src/hooks/queries/__tests__/useAutomationRules.test.tsx src/hooks/queries/__tests__/useAgents.test.tsx src/components/pm/task-detail/__tests__/taskAutosaveFailure.test.ts src/components/pm/task-detail/__tests__/taskPendingPatch.test.ts
```

Expected:
- PASS.

- [ ] **Step 2: Run focused backend tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service -run 'TestPMTaskService_Update' -count=1
```

Expected:
- PASS.

- [ ] **Step 3: Run TypeScript build**

Run:

```bash
pnpm --dir frontend build
```

Expected:
- PASS.

- [ ] **Step 4: Run backend package tests**

Run:

```bash
cd server
GOCACHE=/tmp/go-build-cache go test ./internal/service ./internal/handler -count=1
```

Expected:
- PASS.

- [ ] **Step 5: Manual browser verification**

Use a non-admin member who has PM access but no Automation module access.

Verify:
- Opening a PM story does not produce `403` for `/automation/flows` or `/automation/agents`.
- Network shows PM endpoints instead:
  - `/api/pm/automation-rules?...`
  - `/api/pm/agents?...`
- Estimate update succeeds on a normal task.
- If an update fails, Network shows one failed `PUT /api/pm/tasks/{id}` and does not repeat forever.
- Automation module route/page remains inaccessible without Automation module access.

- [ ] **Step 6: Final commit if verification-only fixes were needed**

If any fixes were made during verification:

```bash
git add <changed-files>
git commit -m "fix: harden pm task automation access"
```

---

## Rollout Notes

- This does not grant broad Automation module access to all PM users.
- PM users get PM-relevant automation metadata through PM-scoped routes.
- Full Automation pages remain gated by Automation module access.
- Existing admins/owners should see no feature loss.
- Existing members/viewers should stop seeing forbidden console errors from PM pages.

## Known Follow-Up

- Decide whether `/api/automation/runs` should remain accessible from PM task run panels for users without Automation module access, or whether equivalent PM-scoped run endpoints should be used consistently. The screenshot showed `runs` returning `200`, so do not include it in the first fix unless new evidence shows it failing.
