// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ExecutionStrip } from '../ExecutionStrip';
import type { AgentRun } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

vi.mock('../useAgentRunStream', () => ({
  useAgentRunStream: () => ({
    streamState: null,
    currentPlan: null,
    pendingInteraction: null,
    clearPendingInteraction: vi.fn(),
    refetch: vi.fn(),
  }),
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

function run(overrides: Partial<AgentRun> = {}): AgentRun {
  return {
    id: 'run-1',
    workspace_id: 'ws-1',
    agent_id: 'agent-1',
    target_type: 'task',
    target_id: 'task-1',
    runtime_kind: 'native_sdk',
    invocation_mode: 'autonomous',
    approval_state: 'not_required',
    pause_reason: 'none',
    status: 'running',
    input: {},
    output_summary: {},
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-05-01T00:00:00Z',
    updated_at: '2026-05-01T00:00:00Z',
    ...overrides,
  };
}

function buttonNamed(name: string): HTMLButtonElement {
  const match = [...container.querySelectorAll<HTMLButtonElement>('button')]
    .find((button) => button.textContent?.trim() === name);
  if (!match) throw new Error(`Missing button: ${name}`);
  return match;
}

describe('ExecutionStrip actions', () => {
  it('renders running cancel as a minimal destructive action', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({ status: 'running' })}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(buttonNamed('Cancel').className).toContain('text-destructive');
    expect(buttonNamed('Cancel').className).toContain('border-destructive/30');
  });

  it('starts a running run collapsed until the user expands it', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({
            status: 'running',
            target_info: {
              target_type: 'task',
              target_id: 'task-1',
              task_key: 'HLP-123',
              title: 'Clarify billing copy',
            },
          })}
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).not.toContain('Task · HLP-123');
    expect(container.textContent).not.toContain('Cancel');

    const toggle = container.querySelector<HTMLButtonElement>('button');
    if (!toggle) throw new Error('toggle not found');
    act(() => {
      toggle.click();
    });

    expect(container.textContent).toContain('Task · HLP-123');
    expect(container.textContent).toContain('Cancel');
  });

  it('renders retry with a distinct retry treatment', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({ status: 'failed', error_message: 'Failed' })}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(buttonNamed('Retry').className).toContain('text-orange-700');
    expect(buttonNamed('Retry').className).toContain('border-orange-500/30');
  });

  it('labels the run target with its entity type', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({
            status: 'running',
            target_type: 'task',
            target_info: {
              target_type: 'task',
              target_id: 'task-1',
              task_key: 'HLP-123',
              title: 'Clarify billing copy',
            },
          })}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('Task · HLP-123');
  });

  it('hides detail actions when a compact running run is collapsed', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({ status: 'running' })}
          compact
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).not.toContain('Cancel');
    expect(container.textContent).not.toContain('Open');
  });

  it('shows open next to cancel in expanded running details', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({ status: 'running' })}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(buttonNamed('Cancel')).toBeTruthy();
    expect(buttonNamed('Open')).toBeTruthy();
  });

  it('collapses all running run details when the chevron is toggled', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({
            status: 'running',
            target_info: {
              target_type: 'task',
              target_id: 'task-1',
              task_key: 'HLP-123',
              title: 'Clarify billing copy',
            },
          })}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('Task · HLP-123');
    expect(container.textContent).toContain('Cancel');

    const toggle = container.querySelector<HTMLButtonElement>('button');
    if (!toggle) throw new Error('toggle not found');
    act(() => {
      toggle.click();
    });

    expect(container.textContent).not.toContain('Task · HLP-123');
    expect(container.textContent).not.toContain('Cancel');
  });
});
