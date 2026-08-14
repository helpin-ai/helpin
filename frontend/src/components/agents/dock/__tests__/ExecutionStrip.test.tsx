// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { ExecutionStrip } from '../ExecutionStrip';
import type { AgentRun, CodingSessionInteraction } from '@/lib/pmTypes';
import type { CommandBarRunPlan } from '../planSummary';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  pendingInteraction: null as CodingSessionInteraction | null,
}));

vi.mock('../useAgentRunStream', () => ({
  useAgentRunStream: () => ({
    streamState: null,
    currentPlan: null,
    pendingInteraction: mocks.pendingInteraction,
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
  mocks.pendingInteraction = null;
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

function plan(overrides: Partial<CommandBarRunPlan> = {}): CommandBarRunPlan {
  return {
    id: 'plan-1',
    steps: [
      {
        agent_id: 'agent-1',
        agent_name: 'Writer',
        instructions: 'Draft the customer reply',
      },
    ],
    runIdsByStep: { 0: 'run-1' },
    status: 'running',
    planKind: 'one_shot_command',
    currentStepIndex: 0,
    prompt: 'Draft a reply',
    createdAt: '2026-05-01T00:00:00Z',
    updatedAt: '2026-05-01T00:00:00Z',
    ...overrides,
  };
}

describe('ExecutionStrip actions', () => {
  it('shows the agent, task, status, and stable plan time while collapsed', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="plan"
          workspaceId="ws-1"
          plan={plan({ status: 'failed', runIdsByStep: {} })}
          runsById={{}}
        />,
      );
    });

    expect(container.textContent).toContain('Writer');
    expect(container.textContent).toContain('Draft the customer reply');
    expect(container.textContent).toContain('Failed');
    expect(container.textContent).not.toContain('AGENT');
    expect(container.querySelector('time')?.getAttribute('dateTime')).toBe('2026-05-01T00:00:00.000Z');
  });

  it('shows a useful failure when expanded without repeating the collapsed details', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="plan"
          workspaceId="ws-1"
          plan={plan({
            status: 'failed',
            runIdsByStep: {},
            errorMessage: 'Model unavailable under current pricing',
          })}
          runsById={{}}
          defaultOpen
        />,
      );
    });

    expect(container.textContent?.match(/Writer/g)).toHaveLength(1);
    expect(container.textContent?.match(/Draft the customer reply/g)).toHaveLength(1);
    expect(container.textContent).toContain('Model unavailable under current pricing');
  });

  it('renders running actions with Open primary and Cancel low emphasis', () => {
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

    const buttons = [...container.querySelectorAll<HTMLButtonElement>('button')]
      .map((button) => button.textContent?.trim())
      .filter(Boolean);
    expect(buttons.indexOf('Open')).toBeLessThan(buttons.indexOf('Cancel'));
    expect(buttonNamed('Open').className).toContain('bg-primary');
    expect(buttonNamed('Cancel').className).toContain('text-muted-foreground');
    expect(buttonNamed('Cancel').className).not.toContain('border-destructive');
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

  it('does not show stale pending approval controls for a completed run', () => {
    mocks.pendingInteraction = {
      interaction_id: 'interaction-1',
      interaction_kind: 'command_execution_approval',
      status: 'pending',
      request_schema_version: 'codex.v2',
      request_payload: { command: 'git push' },
      title: 'Codex needs approval',
    };

    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({ status: 'completed' })}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).not.toContain('Codex needs approval');
    expect(container.textContent).toContain('Re-run');
  });

  it('labels automation flow runs with the flow name while running', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="run"
          workspaceId="ws-1"
          run={run({
            status: 'running',
            target_type: 'workspace',
            target_id: 'ws-1',
            input: {
              trigger: { source: 'automation_rule', trigger_type: 'cron' },
              event: { reason: 'automation rule "Competitors Changelog Tracking Report"' },
            },
          })}
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).toContain('Competitors Changelog Tracking Report');
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

  it('hides resume for a running plan while a step is active', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="plan"
          workspaceId="ws-1"
          plan={plan()}
          runsById={{ 'run-1': run({ status: 'running' }) }}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(container.textContent).not.toContain('Resume');
    expect(container.textContent).not.toContain('Continue');
    expect(buttonNamed('Cancel')).toBeTruthy();
  });

  it('shows continue for a running plan with no active step to advance', () => {
    act(() => {
      root.render(
        <ExecutionStrip
          kind="plan"
          workspaceId="ws-1"
          plan={plan({ runIdsByStep: {} })}
          runsById={{}}
          defaultOpen
          onAction={vi.fn()}
        />,
      );
    });

    expect(buttonNamed('Continue')).toBeTruthy();
    expect(container.textContent).not.toContain('Resume');
    expect(buttonNamed('Cancel')).toBeTruthy();
  });
});
