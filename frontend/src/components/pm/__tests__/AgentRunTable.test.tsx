// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { AgentRunTable } from '../AgentRunTable';
import type { AgentRun } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock('@/components/ui/quick-tooltip', () => ({
  QuickTooltip: ({ label, children }: { label: string; children: React.ReactNode }) => (
    <span data-quick-tooltip-label={label}>{children}</span>
  ),
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
});

function run(overrides: Partial<AgentRun> = {}): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    runtime_kind: 'codex',
    target_type: 'task',
    target_id: 'task-1',
    invocation_mode: 'interactive',
    status: 'completed',
    pause_reason: 'none',
    approval_state: 'not_required',
    input: {},
    output: null,
    output_summary: null,
    error: null,
    tokens_used: 0,
    input_tokens: 0,
    output_tokens: 0,
    cached_input_tokens: 0,
    created_at: '2026-05-12T10:00:00Z',
    updated_at: '2026-05-12T10:00:00Z',
    started_at: null,
    completed_at: null,
    ...overrides,
  };
}

describe('AgentRunTable', () => {
  it('anchors the row view tooltip to the first column value', () => {
    act(() => {
      root.render(
        <AgentRunTable
          runs={[run()]}
          agents={[]}
          selectedRunId={null}
          onSelectRun={vi.fn()}
          loading={false}
        />,
      );
    });

    const rowButton = container.querySelector<HTMLButtonElement>('button');
    const viewRunTooltip = container.querySelector<HTMLElement>('[data-quick-tooltip-label="View run"]');

    expect(rowButton?.getAttribute('title')).toBeNull();
    expect(viewRunTooltip).toBeTruthy();
    expect(viewRunTooltip?.closest('button')).toBe(rowButton);
    expect(viewRunTooltip?.textContent).toContain('Completed');
  });
});
