// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  ActivityTableHeader,
  ActivityTableRow,
  NeedsAttentionTableHeader,
  NeedsAttentionTableRow,
} from '../AutomationActivity';
import { TooltipProvider } from '@/components/ui/tooltip';
import type { AgentRun } from '@/lib/pmTypes';
import type { AutomationTriggerExecutionListItem } from '@/lib/types';

vi.mock('@/components/pm/CodingSession/CodingSessionDrawer', () => ({
  CodingSessionDrawer: () => null,
}));
vi.mock('@/hooks/queries', () => ({
  useAgents: vi.fn(),
  useAutomationActivity: vi.fn(),
  useAutomationOverview: vi.fn(),
  useAutomationTriggerCatalog: vi.fn(),
  usePermissions: vi.fn(),
  useWorkspaceAccess: vi.fn(),
}));
vi.mock('@/stores/workspaceStore', () => ({
  useWorkspaceStore: vi.fn(),
}));

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

let container: HTMLDivElement | null = null;
let root: Root | null = null;

afterEach(() => {
  act(() => root?.unmount());
  root = null;
  container?.remove();
  container = null;
});

function render(node: React.ReactNode) {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  act(() => root?.render(<TooltipProvider>{node}</TooltipProvider>));
}

const execution: AutomationTriggerExecutionListItem = {
  execution_id: 'execution-1',
  agent_id: 'agent-1',
  agent_name: 'Atlas',
  binding_id: 'binding-1',
  binding_kind: 'manual',
  binding_title: 'Manual run',
  target_type: 'workspace',
  target_id: 'workspace-1',
  target_title: 'Usermaven',
  run_id: 'run-1',
  status: 'completed',
  fired_at: '2026-08-11T09:00:00Z',
  started_at: '2026-08-11T09:00:00Z',
  completed_at: '2026-08-11T09:01:00Z',
};

const pausedRun: AgentRun = {
  id: 'run-paused',
  workspace_id: 'workspace-1',
  agent_id: 'agent-1',
  target_type: 'task',
  target_id: 'task-1',
  target_info: {
    target_type: 'task',
    target_id: 'task-1',
    task_key: 'HELP-42',
    title: 'Fix login',
  },
  runtime_kind: 'native_sdk',
  invocation_mode: 'interactive',
  approval_state: 'pending',
  pause_reason: 'human_approval',
  status: 'paused',
  input: {},
  output_summary: {},
  cached_input_tokens: 0,
  input_tokens: 0,
  output_tokens: 0,
  tokens_used: 0,
  created_at: '2026-08-11T09:00:00Z',
  updated_at: '2026-08-11T09:01:00Z',
};

describe('Activity table', () => {
  it('uses comparison columns without an outer card treatment', () => {
    render(<ActivityTableHeader />);

    expect(container?.textContent).toContain('Activity');
    expect(container?.textContent).toContain('Agent');
    expect(container?.textContent).toContain('Status');
    expect(container?.textContent).toContain('Started');
    expect(container?.textContent).toContain('Duration');
    expect(container?.firstElementChild?.className).toContain('border-b');
    expect(container?.firstElementChild?.className).not.toContain('rounded');
    expect(container?.firstElementChild?.className).not.toContain('bg-card');
  });

  it('opens from the row keyboard target without hijacking its nested action', () => {
    let openCount = 0;
    render(
      <ActivityTableRow
        item={execution}
        onOpenRun={() => { openCount += 1; }}
        onOpenFlow={() => {}}
        onOpenTarget={() => {}}
      />,
    );

    const row = container?.querySelector('[role="button"]');
    const action = container?.querySelector('[aria-label="View run"]');
    expect(row).not.toBeNull();
    expect(action).not.toBeNull();
    expect(row?.className).toContain('border-b');
    expect(row?.className).not.toContain('bg-card');

    act(() => {
      action?.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true }));
    });
    expect(openCount).toBe(0);

    act(() => {
      row?.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true }));
    });
    expect(openCount).toBe(1);
  });

  it('keeps failure detail out of the row and exposes it from the Failed badge', () => {
    render(
      <ActivityTableRow
        item={{ ...execution, status: 'failed', error_message: 'Connection refused' }}
        onOpenRun={() => {}}
        onOpenFlow={() => {}}
        onOpenTarget={() => {}}
      />,
    );

    const row = container?.querySelector('[role="button"]');
    const failedBadge = container?.querySelector('[data-slot="tooltip-trigger"]');
    expect(row?.textContent).not.toContain('Connection refused');
    expect(failedBadge?.textContent).toBe('Failed');
    expect(failedBadge?.getAttribute('tabindex')).toBe('0');
  });
});

describe('Needs attention table', () => {
  it('uses attention-specific comparison columns without an outer card', () => {
    render(<NeedsAttentionTableHeader />);

    expect(container?.textContent).toContain('Activity');
    expect(container?.textContent).toContain('Agent');
    expect(container?.textContent).toContain('Status');
    expect(container?.textContent).toContain('Waiting');
    expect(container?.textContent).toContain('Started');
    expect(container?.textContent).toContain('Actions');
    expect(container?.firstElementChild?.className).toContain('border-b');
    expect(container?.firstElementChild?.className).not.toContain('rounded');
    expect(container?.firstElementChild?.className).not.toContain('bg-card');
  });

  it('keeps approval actions independent from row keyboard activation', () => {
    const openRun = vi.fn();
    const approveRun = vi.fn().mockResolvedValue(undefined);
    render(
      <NeedsAttentionTableRow
        run={pausedRun}
        onOpenRun={openRun}
        onOpenTarget={() => {}}
        onApprove={approveRun}
        approving={false}
      />,
    );

    const row = container?.querySelector('[aria-label="Open run for HELP-42 · Fix login"]');
    const viewRun = Array.from(container?.querySelectorAll('button') ?? [])
      .find((button) => button.textContent?.trim() === 'View run');
    const approve = Array.from(container?.querySelectorAll('button') ?? [])
      .find((button) => button.textContent?.trim() === 'Approve');

    expect(row?.className).toContain('border-b');
    expect(row?.className).not.toContain('rounded');
    expect(row?.className).not.toContain('bg-card');

    act(() => viewRun?.click());
    expect(openRun).toHaveBeenCalledTimes(1);

    act(() => approve?.click());
    expect(approveRun).toHaveBeenCalledWith('run-paused');
    expect(openRun).toHaveBeenCalledTimes(1);

    act(() => row?.dispatchEvent(new KeyboardEvent('keydown', { key: ' ', bubbles: true })));
    expect(openRun).toHaveBeenCalledTimes(2);
  });

  it('shows Respond for input pauses and keeps error detail in the status tooltip', () => {
    render(
      <NeedsAttentionTableRow
        run={{
          ...pausedRun,
          approval_state: 'not_required',
          pause_reason: 'human_input',
          error_message: 'More context is required',
        }}
        onOpenRun={() => {}}
        onOpenTarget={() => {}}
        onApprove={async () => {}}
        approving={false}
      />,
    );

    const row = container?.querySelector('[aria-label="Open run for HELP-42 · Fix login"]');
    const attentionBadges = Array.from(container?.querySelectorAll('[data-slot="tooltip-trigger"]') ?? [])
      .filter((trigger) => trigger.textContent === 'Needs your input');

    expect(row?.textContent).toContain('Respond');
    expect(row?.textContent).not.toContain('Approve');
    expect(row?.textContent).not.toContain('More context is required');
    expect(attentionBadges.length).toBeGreaterThan(0);
    expect(attentionBadges[0]?.getAttribute('tabindex')).toBe('0');
  });
});
