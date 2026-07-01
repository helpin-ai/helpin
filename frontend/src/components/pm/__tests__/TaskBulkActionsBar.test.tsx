// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const toastSuccess = vi.fn();
const toastWarning = vi.fn();
const toastError = vi.fn();

vi.mock('sonner', () => ({
  toast: {
    success: (...args: unknown[]) => toastSuccess(...args),
    warning: (...args: unknown[]) => toastWarning(...args),
    error: (...args: unknown[]) => toastError(...args),
  },
}));

const updateMock = vi.fn<(workspaceId: string, id: string, payload: unknown) => Promise<{ error: string | null }>>();
const removeMock = vi.fn<(workspaceId: string, id: string) => Promise<{ error: string | null }>>();

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: {
    update: (workspaceId: string, id: string, payload: unknown) => updateMock(workspaceId, id, payload),
    remove: (workspaceId: string, id: string) => removeMock(workspaceId, id),
  },
}));

import {
  TaskBulkActionsBar,
  BULK_SOFT_CAP,
  buildBulkPatch,
  resolveSelectionWorkflow,
  resolveTeamWorkflow,
  deriveSetField,
  getLabelIdsAfterAdd,
  getLabelIdsAfterRemove,
  getOwnerIdsAfterAdd,
  getOwnerIdsAfterRemove,
  type BulkStagedChanges,
} from '../TaskBulkActionsBar';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { Label, Task, WorkflowWithStates } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
vi.stubGlobal(
  'ResizeObserver',
  class ResizeObserver {
    observe() {}
    unobserve() {}
    disconnect() {}
  },
);
window.HTMLElement.prototype.scrollIntoView = vi.fn();

const labels: Label[] = [
  {
    id: 'label-1',
    workspace_id: 'ws-1',
    name: 'Bug',
    color: '#d33',
    archived: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'label-2',
    workspace_id: 'ws-1',
    name: 'Frontend',
    color: '#36c',
    archived: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  {
    id: 'label-3',
    workspace_id: 'ws-1',
    name: 'Blocked',
    color: '#fc0',
    archived: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
];

const baseTask = {
  id: 'task-1',
  workspace_id: 'ws-1',
  display_id: 1,
  task_key: 'PM-1',
  name: 'Task one',
  task_type: 'feature',
  workflow_id: 'workflow-1',
  workflow_state_id: 'state-1',
  priority: 'medium',
  severity: 'minor',
  position: 1,
  started: false,
  completed: false,
  blocked: false,
  archived: false,
  labels: [labels[0], labels[1]],
  owner_member_ids: ['user-1'],
  created_at: '2026-01-01T00:00:00Z',
  updated_at: '2026-01-01T00:00:00Z',
} satisfies Task;

const workflow = {
  workflow: {
    id: 'workflow-1',
    workspace_id: 'ws-1',
    name: 'Default',
    description: '',
    auto_assign_owner: false,
    created_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:00Z',
  },
  states: [
    {
      id: 'state-1',
      workflow_id: 'workflow-1',
      name: 'Todo',
      state_type: 'unstarted',
      position: 1,
      is_default: true,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
  ],
} satisfies WorkflowWithStates;

function renderBar(props: Partial<React.ComponentProps<typeof TaskBulkActionsBar>> = {}) {
  const container = document.createElement('div');
  document.body.appendChild(container);
  const root = createRoot(container);
  const defaults: React.ComponentProps<typeof TaskBulkActionsBar> = {
    selectedTasks: [baseTask, { ...baseTask, id: 'task-2', task_key: 'PM-2', owner_member_ids: ['user-2'] }],
    workspaceId: 'ws-1',
    workflow,
    assignableMembers: [],
    epics: [],
    sprints: [],
    labels,
    onComplete: vi.fn(),
    onClearSelection: vi.fn(),
  };
  const merged = { ...defaults, ...props };
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  act(() => {
    root.render(
      <QueryClientProvider client={queryClient}>
        <TooltipProvider>
          <TaskBulkActionsBar {...merged} />
        </TooltipProvider>
      </QueryClientProvider>,
    );
  });
  return { container, root, props: merged };
}

function unmount(root: Root, container: HTMLElement) {
  act(() => {
    root.unmount();
  });
  container.remove();
}

describe('TaskBulkActionsBar helpers', () => {
  it('computes label additions as a per-task union', () => {
    expect(getLabelIdsAfterAdd(baseTask, ['label-2', 'label-3'])).toEqual([
      'label-1',
      'label-2',
      'label-3',
    ]);
  });

  it('computes label removals as a per-task difference', () => {
    expect(getLabelIdsAfterRemove(baseTask, ['label-1', 'missing-label'])).toEqual(['label-2']);
  });

  it('computes owner additions as a per-task union', () => {
    expect(getOwnerIdsAfterAdd(baseTask, ['user-2', 'user-1'])).toEqual(['user-1', 'user-2']);
  });

  it('computes owner removals as a per-task difference', () => {
    expect(getOwnerIdsAfterRemove(baseTask, ['user-1'])).toEqual([]);
  });

  it('exposes a soft cap', () => {
    expect(BULK_SOFT_CAP).toBe(25);
  });

  it('derives intersection/union/partial for a fully-shared set', () => {
    const f = deriveSetField([['a', 'b'], ['b', 'a']]);
    expect([...f.intersection].sort()).toEqual(['a', 'b']);
    expect([...f.union].sort()).toEqual(['a', 'b']);
    expect(f.partial).toEqual([]);
    expect(f.shared).toBe(true);
  });

  it('derives intersection/union/partial when tasks differ', () => {
    const f = deriveSetField([['a', 'b'], ['a', 'c']]);
    expect(f.intersection).toEqual(['a']);
    expect([...f.union].sort()).toEqual(['a', 'b', 'c']);
    expect([...f.partial].sort()).toEqual(['b', 'c']);
    expect(f.shared).toBe(false);
  });

  it('handles empty inputs', () => {
    expect(deriveSetField([])).toEqual({ intersection: [], union: [], partial: [], shared: true });
    expect(deriveSetField([[], []])).toEqual({ intersection: [], union: [], partial: [], shared: true });
  });
});

function makeStaged(overrides: Partial<BulkStagedChanges> = {}): BulkStagedChanges {
  return {
    status: undefined,
    priority: undefined,
    severity: undefined,
    epicId: undefined,
    sprintId: undefined,
    deadline: undefined,
    teamId: undefined,
    newTeamWorkflowId: undefined,
    ownerAdds: new Set(),
    ownerRemoves: new Set(),
    labelAdds: new Set(),
    labelRemoves: new Set(),
    ...overrides,
  };
}

describe('resolveTeamWorkflow', () => {
  const wf = (id: string, teamId?: string): WorkflowWithStates => ({
    workflow: {
      id,
      workspace_id: 'ws-1',
      name: id,
      description: '',
      team_id: teamId,
      auto_assign_owner: false,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    states: [],
  });

  it('prefers a workflow owned by the team', () => {
    const list = [wf('shared'), wf('team-a-wf', 'team-a'), wf('team-b-wf', 'team-b')];
    expect(resolveTeamWorkflow(list, 'team-a')?.workflow.id).toBe('team-a-wf');
  });

  it('falls back to the shared (team_id-less) default workflow', () => {
    const list = [wf('team-b-wf', 'team-b'), wf('shared')];
    expect(resolveTeamWorkflow(list, 'team-a')?.workflow.id).toBe('shared');
  });

  it('returns null when neither a team nor a shared workflow exists', () => {
    const list = [wf('team-b-wf', 'team-b')];
    expect(resolveTeamWorkflow(list, 'team-a')).toBeNull();
  });

  it('returns null for empty / undefined inputs', () => {
    expect(resolveTeamWorkflow([], 'team-a')).toBeNull();
    expect(resolveTeamWorkflow(undefined, 'team-a')).toBeNull();
  });
});

describe('resolveSelectionWorkflow', () => {
  const wf = (id: string, stateId: string, teamId?: string): WorkflowWithStates => ({
    workflow: {
      id,
      workspace_id: 'ws-1',
      name: id,
      description: '',
      team_id: teamId,
      auto_assign_owner: false,
      created_at: '2026-01-01T00:00:00Z',
      updated_at: '2026-01-01T00:00:00Z',
    },
    states: [
      {
        id: stateId,
        workflow_id: id,
        name: `${id} status`,
        state_type: 'unstarted',
        position: 1,
        is_default: true,
        created_at: '2026-01-01T00:00:00Z',
        updated_at: '2026-01-01T00:00:00Z',
      },
    ],
  });

  it('uses the selected task workflow before the page fallback workflow', () => {
    const defaultWorkflow = wf('default-workflow', 'default-state');
    const teamWorkflow = wf('team-workflow', 'team-state', 'team-a');
    const task = {
      ...baseTask,
      workflow_id: 'team-workflow',
      workflow_state_id: 'team-state',
      team_id: 'team-a',
    };

    expect(resolveSelectionWorkflow([task], [defaultWorkflow, teamWorkflow], defaultWorkflow)?.workflow.id).toBe('team-workflow');
  });

  it('falls back to the page workflow when selected tasks span workflows', () => {
    const defaultWorkflow = wf('default-workflow', 'default-state');
    const teamWorkflow = wf('team-workflow', 'team-state', 'team-a');
    const taskA = { ...baseTask, id: 'task-a', workflow_id: 'team-workflow', workflow_state_id: 'team-state' };
    const taskB = { ...baseTask, id: 'task-b', workflow_id: 'default-workflow', workflow_state_id: 'default-state' };

    expect(resolveSelectionWorkflow([taskA, taskB], [defaultWorkflow, teamWorkflow], defaultWorkflow)?.workflow.id).toBe('default-workflow');
  });
});

describe('buildBulkPatch', () => {
  it('emits only explicitly staged fields when team is unchanged', () => {
    const patch = buildBulkPatch(baseTask, makeStaged({ status: 'state-9', priority: 'high' }));
    expect(patch).toEqual({ workflow_state_id: 'state-9', priority: 'high' });
  });

  it('clears epic/sprint when staged to null without a team change', () => {
    const patch = buildBulkPatch(baseTask, makeStaged({ epicId: null, sprintId: null }));
    expect(patch.epic_id).toBe('');
    expect(patch.sprint_id).toBe('');
  });

  it('computes label_ids as a per-task union when labels are staged (no team change)', () => {
    const patch = buildBulkPatch(
      baseTask, // labels: label-1, label-2
      makeStaged({ labelAdds: new Set(['label-3']), labelRemoves: new Set(['label-1']) }),
    );
    expect([...(patch.label_ids ?? [])].sort()).toEqual(['label-2', 'label-3']);
  });

  it('on team change: sets team_id, clears epic/sprint/labels, and requires workflow + state', () => {
    const patch = buildBulkPatch(
      baseTask,
      makeStaged({ teamId: 'team-2', newTeamWorkflowId: 'workflow-2', status: 'state-b2' }),
    );
    expect(patch.team_id).toBe('team-2');
    expect(patch.epic_id).toBe('');
    expect(patch.sprint_id).toBe('');
    expect(patch.label_ids).toEqual([]);
    expect(patch.workflow_id).toBe('workflow-2');
    expect(patch.workflow_state_id).toBe('state-b2');
  });

  it('on team change: keeps user-picked epic/sprint and newly chosen labels', () => {
    const patch = buildBulkPatch(
      baseTask,
      makeStaged({
        teamId: 'team-2',
        newTeamWorkflowId: 'workflow-2',
        status: 'state-b2',
        epicId: 'epic-b',
        sprintId: 'sprint-b',
        labelAdds: new Set(['label-shared']),
      }),
    );
    expect(patch.epic_id).toBe('epic-b');
    expect(patch.sprint_id).toBe('sprint-b');
    expect(patch.label_ids).toEqual(['label-shared']);
  });

  it('applies team-independent fields (priority/severity/deadline/owners) on team change', () => {
    const patch = buildBulkPatch(
      baseTask, // owners: user-1
      makeStaged({
        teamId: 'team-2',
        newTeamWorkflowId: 'workflow-2',
        status: 'state-b2',
        priority: 'urgent',
        severity: 'critical',
        deadline: '2026-07-01',
        ownerAdds: new Set(['user-9']),
        ownerRemoves: new Set(['user-1']),
      }),
    );
    expect(patch.priority).toBe('urgent');
    expect(patch.severity).toBe('critical');
    expect(patch.deadline).toBe('2026-07-01');
    expect(patch.owner_member_ids).toEqual(['user-9']);
  });
});

describe('TaskBulkActionsBar', () => {
  beforeEach(() => {
    updateMock.mockReset();
    removeMock.mockReset();
    toastSuccess.mockReset();
    toastWarning.mockReset();
    toastError.mockReset();
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  it('renders the bulk edit trigger with the selected count', () => {
    const { container, root } = renderBar();
    expect(container.textContent).toContain('Edit 2 tasks');
    unmount(root, container);
  });

  it('renders nothing when nothing is selected', () => {
    const { container, root } = renderBar({ selectedTasks: [] });
    expect(container.textContent).toBe('');
    unmount(root, container);
  });

  it('singularises the trigger label for one task', () => {
    const { container, root } = renderBar({ selectedTasks: [baseTask] });
    expect(container.textContent).toContain('Edit 1 task');
    expect(container.textContent).not.toContain('Edit 1 tasks');
    unmount(root, container);
  });

  it('renders the union of labels as chips when label sets differ', () => {
    const taskA = { ...baseTask, id: 'task-1', task_key: 'PM-1', labels: [labels[0], labels[1]] };
    const taskB = { ...baseTask, id: 'task-2', task_key: 'PM-2', labels: [labels[0], labels[2]] };
    const { container, root } = renderBar({ selectedTasks: [taskA, taskB] });
    act(() => {
      (container.querySelector('button') as HTMLButtonElement).click();
    });
    // All three labels visible as chips (intersection: Bug; partial: Frontend, Blocked).
    expect(document.body.textContent).toContain('Bug');
    expect(document.body.textContent).toContain('Frontend');
    expect(document.body.textContent).toContain('Blocked');
    unmount(root, container);
  });

  it('renders disjoint label sets all as chips', () => {
    const taskA = { ...baseTask, id: 'task-1', task_key: 'PM-1', labels: [labels[0]] };
    const taskB = { ...baseTask, id: 'task-2', task_key: 'PM-2', labels: [labels[1]] };
    const { container, root } = renderBar({ selectedTasks: [taskA, taskB] });
    act(() => {
      (container.querySelector('button') as HTMLButtonElement).click();
    });
    expect(document.body.textContent).toContain('Bug');
    expect(document.body.textContent).toContain('Frontend');
    unmount(root, container);
  });

  it('shows "No labels" when no selected task has any labels', () => {
    const taskA = { ...baseTask, id: 'task-1', task_key: 'PM-1', labels: [] };
    const taskB = { ...baseTask, id: 'task-2', task_key: 'PM-2', labels: [] };
    const { container, root } = renderBar({ selectedTasks: [taskA, taskB] });
    act(() => {
      (container.querySelector('button') as HTMLButtonElement).click();
    });
    expect(document.body.textContent).toContain('No labels');
    unmount(root, container);
  });

  it('renders a Team selector at the top with the workspace teams', () => {
    const teams = [
      { id: 'team-1', workspace_id: 'ws-1', name: 'Engineering' },
      { id: 'team-2', workspace_id: 'ws-1', name: 'Design' },
    ];
    const { container, root } = renderBar({ teams });
    act(() => {
      (container.querySelector('button') as HTMLButtonElement).click();
    });
    const labelNodes = Array.from(document.querySelectorAll('span')).map((s) => s.textContent);
    expect(labelNodes).toContain('Team');
    // Team appears before Status in the field list.
    const text = document.body.textContent ?? '';
    expect(text.indexOf('Team')).toBeLessThan(text.indexOf('Status'));
    unmount(root, container);
  });

  it('shows an interactive "Multiple" trigger for fields whose values differ', () => {
    const taskA = { ...baseTask, id: 'task-1', task_key: 'PM-1', priority: 'high' as const };
    const taskB = { ...baseTask, id: 'task-2', task_key: 'PM-2', priority: 'low' as const };
    const { container, root } = renderBar({ selectedTasks: [taskA, taskB] });
    act(() => {
      (container.querySelector('button') as HTMLButtonElement).click();
    });
    expect(document.body.textContent).toContain('Multiple');
    // The mixed-value triggers are real, enabled Select triggers (not disabled spans),
    // so the dropdown can be opened — this is the fix for the positioning bug.
    const triggers = Array.from(
      document.querySelectorAll('[data-slot="select-trigger"]'),
    ) as HTMLButtonElement[];
    expect(triggers.length).toBeGreaterThan(0);
    expect(triggers.every((t) => !t.disabled)).toBe(true);
    unmount(root, container);
  });

  it('disables the Apply button when there are no staged changes', () => {
    const { container, root } = renderBar();
    act(() => {
      (container.querySelector('button') as HTMLButtonElement).click();
    });
    const applyBtn = Array.from(document.querySelectorAll('button')).find(
      (btn) => btn.textContent?.trim() === 'Apply',
    ) as HTMLButtonElement | undefined;
    expect(applyBtn).toBeDefined();
    expect(applyBtn?.disabled).toBe(true);
    unmount(root, container);
  });
});
