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
  getLabelIdsAfterAdd,
  getLabelIdsAfterRemove,
  getOwnerIdsAfterAdd,
  getOwnerIdsAfterRemove,
} from '../TaskBulkActionsBar';
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
  act(() => {
    root.render(
      <TooltipProvider>
        <TaskBulkActionsBar {...merged} />
      </TooltipProvider>,
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
});
