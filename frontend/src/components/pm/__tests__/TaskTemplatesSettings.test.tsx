// @vitest-environment jsdom
import { act } from 'react';
import { createRoot } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { TaskTemplatesSettings } from '@/components/pm/TaskTemplatesSettings';
import { pmTaskTemplateService } from '@/lib/services/pmTaskTemplateService';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';

vi.mock('sonner', () => ({
  toast: {
    error: vi.fn(),
    success: vi.fn(),
  },
}));

vi.mock('@/components/ui/button', () => ({
  Button: ({ children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement>) => (
    <button {...props}>{children}</button>
  ),
}));

vi.mock('@/components/ui/tooltip', () => ({
  Tooltip: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  TooltipContent: ({ children }: { children: React.ReactNode }) => <span>{children}</span>,
  TooltipTrigger: ({ children }: { children: React.ReactNode }) => <>{children}</>,
}));

vi.mock('@/components/ui/select', () => ({
  Select: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectContent: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectItem: ({ children }: { children: React.ReactNode }) => <div>{children}</div>,
  SelectTrigger: ({ children }: { children: React.ReactNode }) => <button>{children}</button>,
  SelectValue: () => <span />,
}));

vi.mock('@/components/pm/CreateTaskModal', () => ({
  CreateTaskModal: () => null,
}));

vi.mock('@/hooks/useAccessibleTeams', () => ({
  useAccessibleTeams: () => ({
    teams: [{ id: 'team-1', name: 'Engineering' }],
  }),
}));

vi.mock('@/lib/services/pmTaskTemplateService', () => ({
  pmTaskTemplateService: {
    list: vi.fn(),
    create: vi.fn(),
    remove: vi.fn(),
  },
}));

vi.mock('@/lib/services/pmWorkflowService', () => ({
  pmWorkflowService: {
    list: vi.fn(),
  },
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

describe('TaskTemplatesSettings', () => {
  let container: HTMLDivElement;
  let root: ReturnType<typeof createRoot>;

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);

    vi.mocked(pmTaskTemplateService.list).mockResolvedValue({
      data: [
        {
          id: 'template-1',
          workspace_id: 'workspace-1',
          team_id: 'team-1',
          name: 'Bug report',
          description: '<p>Collect reproduction steps and expected behavior.</p>',
          workflow_state_id: 'state-ready',
          archived: false,
          created_at: '2026-05-06T00:00:00Z',
          updated_at: '2026-05-06T00:00:00Z',
        },
      ],
      error: null,
    } as any);
    vi.mocked(pmWorkflowService.list).mockResolvedValue({
      data: [
        {
          workflow: {
            id: 'workflow-1',
            workspace_id: 'workspace-1',
            name: 'Engineering workflow',
            team_id: 'team-1',
            auto_assign_owner: false,
            created_at: '2026-05-06T00:00:00Z',
            updated_at: '2026-05-06T00:00:00Z',
          },
          states: [
            {
              id: 'state-ready',
              workflow_id: 'workflow-1',
              name: 'Ready',
              state_type: 'unstarted',
              position: 0,
              is_default: true,
              created_at: '2026-05-06T00:00:00Z',
              updated_at: '2026-05-06T00:00:00Z',
            },
          ],
        },
      ],
      error: null,
    } as any);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
    vi.clearAllMocks();
  });

  it('renders templates as row cards with team and default state metadata', async () => {
    await act(async () => {
      root.render(<TaskTemplatesSettings workspaceId="workspace-1" />);
    });

    await act(async () => {
      await Promise.resolve();
    });

    expect(container.textContent).toContain('Bug report');
    expect(container.textContent).toContain('Collect reproduction steps and expected behavior.');
    expect(container.textContent).toContain('Engineering');
    expect(container.textContent).toContain('Ready');
    expect(container.textContent).toContain('Create template');
  });

  it('hides create and row actions when the user cannot manage templates', async () => {
    await act(async () => {
      root.render(
        <TaskTemplatesSettings
          workspaceId="workspace-1"
          canManageSharedTemplates={false}
          managedTeamIds={[]}
        />,
      );
    });

    await act(async () => {
      await Promise.resolve();
    });

    expect(container.textContent).toContain('Bug report');
    expect(container.textContent).not.toContain('Create template');
    expect(container.textContent).not.toContain('Edit');
    expect(container.textContent).not.toContain('Duplicate');
    expect(container.textContent).not.toContain('Delete');
  });

  it('shows row actions for team templates managed by the user', async () => {
    await act(async () => {
      root.render(
        <TaskTemplatesSettings
          workspaceId="workspace-1"
          canManageSharedTemplates={false}
          managedTeamIds={['team-1']}
        />,
      );
    });

    await act(async () => {
      await Promise.resolve();
    });

    expect(container.textContent).toContain('Bug report');
    expect(container.textContent).toContain('Create template');
    expect(container.textContent).toContain('Edit');
    expect(container.textContent).toContain('Duplicate');
    expect(container.textContent).toContain('Delete');
  });
});
