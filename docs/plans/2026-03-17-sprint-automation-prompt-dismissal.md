# Sprint automation prompt dismissal implementation plan

> Historical implementation plan, source-compared on 2026-09-17. Team-wide
> sprint prompt dismissal is implemented. This record is for contributors tracing
> the change; its unchecked create-file steps and expected initial failures are
> not current work instructions.

## Current implementation

The [helper](../../frontend/src/components/pm/sprintAutomationPrompt.ts) suppresses
prompting whenever the team has a `sprint_auto_create` row, including a disabled
one. It accepts null/undefined lists, creates a disabled request with the prompt's
count, duration, and start day, and leaves move-unfinished automation untouched.
The async helper throws on an API error and returns response **data**, rather
than the full result object in this plan's sample implementation.

[GlobalCreateModals](../../frontend/src/components/pm/GlobalCreateModals.tsx)
uses a separate dismissal loading state. Both **No thanks** and closing the
prompt call the persistence handler; success shows “Sprint automation dismissed”
and closes, while failure keeps the prompt open with an error toast. The current
[helper tests](../../frontend/src/components/pm/__tests__/sprintAutomationPrompt.test.ts)
exist, so the original missing-file RED step no longer describes this checkout.

See the [reviewed design](../specs/2026-03-17-sprint-automation-prompt-dismissal-design.md)
for the current team-settings location and the fetch-error/overview-label
limitations. The [frontend manifest](../../frontend/package.json) currently has
Vitest but does not declare the named DOM-test packages; this narrow dependency
observation is not a ban on future component tests. Old build failures and the
npm/npx verification sequence below are historical, not newly reproduced results.
No application code or automation records were changed in this review.

## Original implementation record

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist `No thanks` on the sprint automation prompt as a team-wide dismissal so the same team is not prompted again on later manual sprint creation.

**Architecture:** Keep the existing sprint creation flow and PM automation API intact. Extract the prompt decision, dismissal payload shaping, and dismiss-action request flow into a small frontend helper, then wire the modal to use that helper when the user declines. This reuses existing PM automation storage, keeps the new behavior visible in Settings, and stays testable with the repo's current non-DOM Vitest setup.

**Tech Stack:** React 19, TypeScript 5.9, Vite 7, Vitest 2, existing PM automation service layer

---

### Task 1: Persist Team-Wide Sprint Prompt Dismissal

**Files:**
- Create: `frontend/src/components/pm/sprintAutomationPrompt.ts`
- Create: `frontend/src/components/pm/__tests__/sprintAutomationPrompt.test.ts`
- Modify: `frontend/src/components/pm/GlobalCreateModals.tsx`

- [ ] **Step 1: Write the failing tests**

Create `frontend/src/components/pm/__tests__/sprintAutomationPrompt.test.ts` with focused unit tests for the extracted helper logic:

```ts
import { describe, expect, it } from 'vitest';

import {
  buildSprintAutomationDismissRequest,
  dismissSprintAutomationPrompt,
  shouldPromptSprintAutomation,
} from '../sprintAutomationPrompt';

describe('shouldPromptSprintAutomation', () => {
  it('returns true when a team has no sprint_auto_create record', () => {
    expect(shouldPromptSprintAutomation([], 'team-1')).toBe(true);
  });

  it('returns false when a team has a disabled sprint_auto_create record', () => {
    expect(
      shouldPromptSprintAutomation(
        [
          {
            id: 'auto-1',
            workspace_id: 'ws-1',
            automation_type: 'sprint_auto_create',
            enabled: false,
            team_id: 'team-1',
            config_int: 2,
            config_int2: 2,
            config_int3: 1,
            created_at: '',
            updated_at: '',
          },
        ],
        'team-1',
      ),
    ).toBe(false);
  });
});

describe('buildSprintAutomationDismissRequest', () => {
  it('builds a disabled sprint_auto_create request from the prompt state', () => {
    expect(
      buildSprintAutomationDismissRequest('ws-1', {
        teamId: 'team-1',
        teamName: 'Engineering',
        sprintCount: 3,
        weeks: 2,
        startDay: 1,
        moveUnfinished: true,
      }),
    ).toEqual({
      workspace_id: 'ws-1',
      automation_type: 'sprint_auto_create',
      enabled: false,
      team_id: 'team-1',
      config_int: 3,
      config_int2: 2,
      config_int3: 1,
    });
  });

  it('never turns the dismiss request into sprint_move_unfinished', () => {
    expect(
      buildSprintAutomationDismissRequest('ws-1', {
        teamId: 'team-1',
        teamName: 'Engineering',
        sprintCount: 3,
        weeks: 2,
        startDay: 1,
        moveUnfinished: true,
      }).automation_type,
    ).toBe('sprint_auto_create');
  });
});

describe('dismissSprintAutomationPrompt', () => {
  it('sends a disabled sprint_auto_create upsert request for the team', async () => {
    const requests: unknown[] = [];

    await dismissSprintAutomationPrompt({
      workspaceId: 'ws-1',
      prompt: {
        teamId: 'team-1',
        teamName: 'Engineering',
        sprintCount: 3,
        weeks: 2,
        startDay: 1,
        moveUnfinished: true,
      },
      upsert: async (_workspaceId, payload) => {
        requests.push([_workspaceId, payload]);
        return { data: null, error: null };
      },
    });

    expect(requests).toEqual([
      [
        'ws-1',
        {
          workspace_id: 'ws-1',
          automation_type: 'sprint_auto_create',
          enabled: false,
          team_id: 'team-1',
          config_int: 3,
          config_int2: 2,
          config_int3: 1,
        },
      ],
    ]);
  });

  it('throws when the upsert response includes an error', async () => {
    await expect(
      dismissSprintAutomationPrompt({
        workspaceId: 'ws-1',
        prompt: {
          teamId: 'team-1',
          teamName: 'Engineering',
          sprintCount: 3,
          weeks: 2,
          startDay: 1,
          moveUnfinished: true,
        },
        upsert: async () => ({ data: null, error: 'save failed' }),
      }),
    ).rejects.toThrow('save failed');
  });
});
```

- [ ] **Step 2: Run the new test to verify RED**

Run:

```bash
cd frontend && npx vitest run src/components/pm/__tests__/sprintAutomationPrompt.test.ts
```

Expected: FAIL because `../sprintAutomationPrompt` does not exist yet.

- [ ] **Step 3: Add the minimal helper implementation**

Create `frontend/src/components/pm/sprintAutomationPrompt.ts` with the smallest reusable logic needed by the modal:

```ts
import type { PMAutomation, UpsertAutomationRequest } from '@/lib/pmTypes';

export interface SprintAutomationPromptState {
  teamId: string;
  teamName: string;
  sprintCount: number;
  weeks: number;
  startDay: number;
  moveUnfinished: boolean;
}

type PMAutomationUpsert = (
  workspaceId: string,
  payload: UpsertAutomationRequest,
) => Promise<{ data: unknown; error: string | null }>;

export function shouldPromptSprintAutomation(automations: PMAutomation[] | undefined, teamId: string): boolean {
  return !automations?.some(
    (automation) => automation.automation_type === 'sprint_auto_create' && automation.team_id === teamId,
  );
}

export function buildSprintAutomationDismissRequest(
  workspaceId: string,
  prompt: SprintAutomationPromptState,
): UpsertAutomationRequest {
  return {
    workspace_id: workspaceId,
    automation_type: 'sprint_auto_create',
    enabled: false,
    team_id: prompt.teamId,
    config_int: prompt.sprintCount,
    config_int2: prompt.weeks,
    config_int3: prompt.startDay,
  };
}

export async function dismissSprintAutomationPrompt(args: {
  workspaceId: string;
  prompt: SprintAutomationPromptState;
  upsert: PMAutomationUpsert;
}) {
  const result = await args.upsert(
    args.workspaceId,
    buildSprintAutomationDismissRequest(args.workspaceId, args.prompt),
  );
  if (result.error) {
    throw new Error(result.error);
  }
  return result;
}
```

- [ ] **Step 4: Wire the modal to use the helper and persist dismissal**

Update `frontend/src/components/pm/GlobalCreateModals.tsx`:

- Import the new helper functions and shared prompt-state type.
- Replace the inline `hasAutoCreate` check with `shouldPromptSprintAutomation(...)`.
- Keep the existing enable path unchanged.
- Add a dedicated `dismissAutomations` handler that:
  - returns early if there is no prompt state
  - sets the existing loading state
  - calls `dismissSprintAutomationPrompt({ workspaceId, prompt: automationPrompt, upsert: pmAutomationService.upsert })`
  - treats `{ error: ... }` responses as failure via the helper throwing, rather than as success
  - on success, shows a short toast such as `Sprint automation preference saved for ${teamName}`
  - closes the modal
  - on failure, keeps the modal open and shows an error toast
  - always clears the loading state before returning
- Point the `No thanks` button at `dismissAutomations` instead of the plain `onClose`.
- Keep `sprint_move_unfinished` untouched on dismiss.
- Do not add a component-level DOM test stack for this change. The repo does not currently have `jsdom`, `happy-dom`, or React Testing Library installed, so the dismissal flow should be verified through the extracted async helper instead.

- [ ] **Step 5: Run the targeted tests to verify GREEN**

Run:

```bash
cd frontend && npx vitest run src/components/pm/__tests__/sprintAutomationPrompt.test.ts
```

Expected: PASS.

- [ ] **Step 6: Run focused regression verification**

Run:

```bash
cd frontend && npx vitest run src/components/pm/__tests__/sprintAutomationPrompt.test.ts src/lib/__tests__/teamPresets.test.ts
```

Expected: PASS. This confirms the new sprint prompt helper behavior and the existing workspace team preset regression both remain green.

- [ ] **Step 7: Run a TypeScript/build verification pass**

Run:

```bash
cd frontend && npm run build
```

Expected:
- If the command exits `0`, the change is build-clean.
- If it fails, identify whether the failure is the known pre-existing `TeamsTab.tsx` unused symbol issue or a new error introduced by this work. Do not claim build success without a clean exit.
