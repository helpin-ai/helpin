// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { GlobalTaskPanel } from '../GlobalTaskPanel';
import { useWorkspaceStore } from '@/stores/workspaceStore';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  navigate: vi.fn(),
  getRun: vi.fn(),
  getTask: vi.fn(),
  listWorkflows: vi.fn(),
  getRecurringByTask: vi.fn(),
}));

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mocks.navigate,
  useLocation: () => ({
    pathname: window.location.pathname,
    href: window.location.href,
  }),
}));

vi.mock('@/components/command-bar/pageContext', () => ({
  useRegisterPageContext: vi.fn(),
}));

vi.mock('@/components/pm/TaskDetailPanel', () => ({
  TaskDetailPanel: () => null,
}));

vi.mock('@/components/pm/CodingSession/CodingSessionDrawer', () => ({
  CodingSessionDrawer: ({ sessionId, open }: { sessionId: string | null; open: boolean }) => (
    <div data-testid="coding-session-drawer" data-session-id={sessionId ?? ''} data-open={String(open)} />
  ),
}));

vi.mock('@/lib/services/agentService', () => ({
  agentService: {
    getRun: mocks.getRun,
  },
}));

vi.mock('@/lib/services/pmTaskService', () => ({
  pmTaskService: {
    get: mocks.getTask,
    getByDisplayId: vi.fn(),
  },
}));

vi.mock('@/lib/services/pmWorkflowService', () => ({
  pmWorkflowService: {
    list: mocks.listWorkflows,
  },
}));

vi.mock('@/lib/services/pmRecurringTemplateService', () => ({
  pmRecurringTemplateService: {
    getByTask: mocks.getRecurringByTask,
  },
}));

describe('GlobalTaskPanel run deep links', () => {
  let container: HTMLDivElement;
  let root: Root;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
    window.history.replaceState({}, '', '/w/usermaven/pm/tasks?team=team-1&run=run-1');
    useWorkspaceStore.setState({
      currentWorkspace: { id: 'ws-1', slug: 'usermaven', name: 'Usermaven' } as never,
    });
    mocks.getRun.mockResolvedValue({
      data: {
        id: 'run-1',
        target_type: 'workspace',
        target_id: 'ws-1',
      },
      error: null,
    });
    mocks.getTask.mockResolvedValue({
      data: {
        task: {
          id: 'task-1',
          name: 'Fix deep link',
          workflow_id: 'workflow-1',
          recurring_template_id: null,
        },
      },
      error: null,
    });
    mocks.listWorkflows.mockResolvedValue({
      data: [{ workflow: { id: 'workflow-1' }, states: [] }],
      error: null,
    });
    mocks.getRecurringByTask.mockResolvedValue({ data: null, error: null });
  });

  afterEach(() => {
    act(() => {
      root.unmount();
    });
    container.remove();
    document.body.innerHTML = '';
    useWorkspaceStore.setState({ currentWorkspace: null });
    vi.clearAllMocks();
  });

  async function renderPanel() {
    await act(async () => {
      root.render(<GlobalTaskPanel workspaceId="ws-1" />);
    });
    await act(async () => {
      await Promise.resolve();
    });
  }

  it('opens a generic run drawer when a shared run link is not task-targeted', async () => {
    await renderPanel();

    expect(mocks.getRun).toHaveBeenCalledWith('ws-1', 'run-1');
    expect(mocks.navigate).not.toHaveBeenCalled();
    expect(container.querySelector('[data-testid="coding-session-drawer"]')).toMatchObject({
      dataset: {
        sessionId: 'run-1',
        open: 'true',
      },
    });
  });

  it('redirects task-targeted run links to the canonical task route', async () => {
    mocks.getRun.mockResolvedValue({
      data: {
        id: 'run-1',
        target_type: 'task',
        target_id: 'task-1',
      },
      error: null,
    });

    await renderPanel();

    expect(mocks.getTask).toHaveBeenCalledWith('ws-1', 'task-1');
    expect(mocks.navigate).toHaveBeenCalledWith({
      to: '/w/$slug/pm/tasks/$taskId',
      params: { slug: 'usermaven', taskId: 'task-1' },
      search: { run: 'run-1', team: 'team-1' },
      replace: true,
    });
    expect(container.querySelector('[data-testid="coding-session-drawer"]')).toMatchObject({
      dataset: {
        sessionId: '',
        open: 'false',
      },
    });
  });
});
