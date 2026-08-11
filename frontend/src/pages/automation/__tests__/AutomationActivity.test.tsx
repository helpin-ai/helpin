// @vitest-environment jsdom

import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ActivityTableHeader, ActivityTableRow } from '../AutomationActivity';
import { TooltipProvider } from '@/components/ui/tooltip';
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
