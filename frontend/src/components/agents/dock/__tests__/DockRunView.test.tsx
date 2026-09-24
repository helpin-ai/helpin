// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import type {
  AgentRun,
  CodingSession,
  CodingSessionStreamState,
  RunPlanArtifact,
} from '@/lib/pmTypes';
import type { DockRunSummary } from '@/lib/dockTypes';
import { DockRunView } from '../DockRunView';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  session: null as CodingSession | null,
  currentPlan: null as RunPlanArtifact | null,
  streamState: null as CodingSessionStreamState | null,
}));

vi.mock('../useAgentRunStream', () => ({
  useAgentRunStream: () => ({
    session: mocks.session,
    currentPlan: mocks.currentPlan,
    streamState: mocks.streamState,
    pendingInteraction: null,
    loading: false,
    refetch: vi.fn(),
    clearPendingInteraction: vi.fn(),
  }),
}));

vi.mock('../DockTranscript', () => ({
  DockTranscript: (props: Record<string, unknown>) => (
    <div
      data-testid="dock-transcript"
      data-active={String(props.active)}
      data-runtime={String(props.useRuntimeTimeline)}
      data-compact={String(props.compactAssistantProgress)}
      data-completed-run={String(props.completedRun)}
      data-actor={(props.fallbackActor as { full_name?: string } | undefined)?.full_name ?? ''}
    />
  ),
}));

vi.mock('@/components/pm/CodingSession/CodingPlanPanel', () => ({
  CodingPlanPanel: ({ runStatus, title }: { runStatus?: string; title?: string }) => (
    <div data-testid="run-plan" data-status={runStatus}>{title}</div>
  ),
}));

vi.mock('../DockInput', () => ({
  DockInput: ({ disabled, placeholder, onStop, onPause, onResume }: {
    disabled?: boolean;
    placeholder?: string;
    onStop?: () => void;
    onPause?: () => void;
    onResume?: () => void;
  }) => (
    <div
      data-testid="dock-input"
      data-disabled={String(disabled)}
      data-can-stop={String(Boolean(onStop))}
      data-can-pause={String(Boolean(onPause))}
      data-can-resume={String(Boolean(onResume))}
    >
      {placeholder}
    </div>
  ),
}));

vi.mock('@/components/agents/transcript', () => ({
  deriveLiveStatusLabel: () => null,
  ScrollToLatestButton: () => null,
}));

let container: HTMLDivElement;
let root: Root;

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  mocks.session = null;
  mocks.currentPlan = null;
  mocks.streamState = null;
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
  vi.clearAllMocks();
});

function agentRun(overrides: Partial<AgentRun> = {}): AgentRun {
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
    status: 'completed',
    input: {},
    output_summary: {},
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    created_at: '2026-08-21T19:50:00Z',
    updated_at: '2026-08-21T19:50:00Z',
    ...overrides,
  };
}

function codingSession(overrides: Partial<CodingSession> = {}): CodingSession {
  return {
    id: 'run-1',
    run_id: 'run-1',
    workspace_id: 'ws-1',
    target_type: 'task',
    target_id: 'task-1',
    agent_id: 'agent-1',
    runtime_kind: 'native_sdk',
    invocation_mode: 'autonomous',
    status: 'running',
    pause_reason: 'none',
    approval_state: 'not_required',
    title: 'Fix timeline rendering',
    capabilities: {
      live_text_streaming: true,
      tool_streaming: true,
      repo_diff_streaming: true,
      plan_streaming: true,
      approvals: true,
      human_input: true,
      authentication: true,
      previews: true,
      terminal_output: true,
      checkpoints: true,
    },
    repo: { is_dirty: false, changed_file_count: 0 },
    cached_input_tokens: 0,
    input_tokens: 0,
    output_tokens: 0,
    tokens_used: 0,
    triggered_by_user: {
      id: 'user-1',
      email: 'waqar@example.com',
      full_name: 'Waqar Azeem',
    },
    created_at: '2026-08-21T19:50:00Z',
    updated_at: '2026-08-21T19:52:00Z',
    ...overrides,
  };
}

function streamState(): CodingSessionStreamState {
  return {
    transcript_messages: [],
    live_assistant_message: null,
    live_reasoning_message: null,
    live_turn_segments: [],
    activity_events: [],
    current_plan: mocks.currentPlan,
    completed_tool_calls: [],
  };
}

describe('DockRunView timeline parity', () => {
  it('uses the fresh session and the shared Ask Agent timeline presentation', () => {
    mocks.session = codingSession();
    mocks.currentPlan = { plan: [{ step: 'Reconcile timeline events', status: 'in_progress' }] };
    mocks.streamState = streamState();
    const summary: DockRunSummary = {
      run: agentRun({ status: 'completed' }),
      agent: { id: 'agent-1', name: 'Scribe' },
      last_activity_at: '2026-08-21T19:50:00Z',
    };

    act(() => {
      root.render(
        <DockRunView
          workspaceId="ws-1"
          summary={summary}
          draft=""
          onDraftChange={vi.fn()}
          onRunChanged={vi.fn()}
          onRunContinued={vi.fn()}
        />,
      );
    });

    const transcript = container.querySelector('[data-testid="dock-transcript"]');
    expect(transcript?.getAttribute('data-active')).toBe('true');
    expect(transcript?.getAttribute('data-runtime')).toBe('true');
    expect(transcript?.getAttribute('data-compact')).toBe('true');
    expect(transcript?.getAttribute('data-completed-run')).toBe('false');
    expect(transcript?.getAttribute('data-actor')).toBe('Waqar Azeem');
    expect(container.querySelector('[data-testid="run-plan"]')?.getAttribute('data-status')).toBe('running');
    expect(container.textContent).toContain('Current work plan');
    expect(container.querySelector('[data-current-work-plan]')).not.toBeNull();
    expect(container.querySelector('[data-testid="dock-input"]')?.getAttribute('data-disabled')).toBe('true');
    expect(container.querySelector('[data-testid="dock-input"]')?.getAttribute('data-can-stop')).toBe('true');
    expect(container.querySelector('[data-testid="dock-input"]')?.getAttribute('data-can-pause')).toBe('true');
    expect(container.textContent).toContain('Agent is working…');
  });

  it('offers resume for a manually paused run', () => {
    mocks.session = codingSession({ status: 'paused', pause_reason: 'manual' });
    const summary: DockRunSummary = {
      run: agentRun({ status: 'paused', pause_reason: 'manual' }),
      agent: { id: 'agent-1', name: 'Scribe' },
      last_activity_at: '2026-08-21T19:53:00Z',
    };
    act(() => root.render(<DockRunView workspaceId="ws-1" summary={summary} draft="" onDraftChange={vi.fn()} onRunChanged={vi.fn()} onRunContinued={vi.fn()} />));
    const input = container.querySelector('[data-testid="dock-input"]');
    expect(input?.getAttribute('data-can-resume')).toBe('true');
    expect(input?.getAttribute('data-can-pause')).toBe('false');
    expect(input?.getAttribute('data-disabled')).toBe('true');
    expect(container.textContent).toContain('Agent paused');
  });

  it('delegates completed work disclosure to the transcript instead of adding a passive footer', () => {
    mocks.session = codingSession({
      status: 'completed',
      started_at: '2026-08-21T19:50:00Z',
      updated_at: '2026-08-21T19:53:00Z',
    });
    mocks.streamState = streamState();
    const summary: DockRunSummary = {
      run: agentRun({ status: 'completed' }),
      agent: { id: 'agent-1', name: 'Scribe' },
      last_activity_at: '2026-08-21T19:53:00Z',
    };

    act(() => {
      root.render(
        <DockRunView
          workspaceId="ws-1"
          summary={summary}
          draft=""
          onDraftChange={vi.fn()}
          onRunChanged={vi.fn()}
          onRunContinued={vi.fn()}
        />,
      );
    });

    expect(container.querySelector('[data-testid="dock-transcript"]')?.getAttribute('data-completed-run')).toBe('true');
    expect(container.querySelector('[data-agent-live-status-region]')).toBeNull();
  });
});
