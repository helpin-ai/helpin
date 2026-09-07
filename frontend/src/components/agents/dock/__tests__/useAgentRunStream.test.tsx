// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useAgentRunStream } from '../useAgentRunStream';
import { useSupportPresenceStore } from '@/stores/supportPresenceStore';
import type { CodingSessionEvent } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  getRunSnapshot: vi.fn(),
  listRunEvents: vi.fn(),
}));

// This hook needs the authenticated identity, not auth initialization/analytics.
vi.mock('@/stores/authStore', async () => {
  const { create } = await import('zustand');
  return { useAuthStore: create<{ user: { id: string } | null }>(() => ({ user: null })) };
});

vi.mock('@/lib/services/agentService', () => ({
  agentService: {
    getRunSnapshot: mocks.getRunSnapshot,
    listRunEvents: mocks.listRunEvents,
  },
}));

let container: HTMLDivElement;
let root: Root;
let latestState: ReturnType<typeof useAgentRunStream> | null = null;

function interactionEvent(sequenceNo: number, interactionId: string, status = 'pending'): CodingSessionEvent {
  return {
    id: `event-${interactionId}-${sequenceNo}`,
    session_id: 'run-1',
    run_id: 'run-1',
    sequence_no: sequenceNo,
    timestamp: `2026-05-15T00:00:0${sequenceNo}Z`,
    type: status === 'pending' ? 'interaction.requested' : 'interaction.resolved',
    runtime_kind: 'native_sdk',
    payload: {
      interaction_id: interactionId,
      interaction_kind: 'request_user_input',
      status,
      request_schema_version: 'helpin.v1',
      request_payload: { questions: [{ id: 'q1', text: 'Need input?' }] },
      title: `Question ${interactionId}`,
      assistant_message_sequence_no: sequenceNo,
    },
  };
}

function assistantEvent(
  sequenceNo: number,
  content: string,
  overrides: Partial<CodingSessionEvent> = {},
): CodingSessionEvent {
  return {
    id: `runtime-event-${sequenceNo}`,
    session_id: 'run-1',
    run_id: 'run-1',
    sequence_no: sequenceNo,
    timestamp: `2026-05-15T00:00:${String(sequenceNo).padStart(2, '0')}Z`,
    type: sequenceNo === 1 ? 'assistant.message.started' : 'assistant.message.delta',
    runtime_kind: 'native_sdk',
    payload: {
      message_id: 'assistant-1',
      text: content,
      content,
      host_run_id: 'run-1',
    },
    runtime_metadata: { source: 'agent-runtime-v2' },
    ...overrides,
  };
}

function Probe({ runId = 'run-1', active = true, pollMs = 0 }: { runId?: string; active?: boolean; pollMs?: number }) {
  latestState = useAgentRunStream('ws-1', runId, active, pollMs);
  return (
    <div data-testid="pending">
      {latestState.pendingInteraction?.interaction_id ?? 'none'}
    </div>
  );
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
  latestState = null;
  mocks.getRunSnapshot.mockResolvedValue({
    data: { stream_state_snapshot: null },
    error: null,
  });
  mocks.listRunEvents.mockResolvedValue({
    data: { events: [interactionEvent(1, 'interaction-1')], next_sequence_no: 1 },
    error: null,
  });
});

afterEach(() => {
  act(() => {
    root.unmount();
  });
  container.remove();
  document.body.innerHTML = '';
  vi.clearAllMocks();
});

async function renderProbe() {
  await act(async () => {
    root.render(<Probe />);
  });
  await waitForPending('interaction-1');
}

async function waitForPending(expected: string) {
  for (let i = 0; i < 20; i += 1) {
    if (container.textContent === expected) return;
    await act(async () => {
      await Promise.resolve();
    });
  }
  throw new Error(`Expected pending ${expected}, got ${container.textContent}`);
}

async function waitFor(check: () => boolean, message: string) {
  for (let i = 0; i < 30; i += 1) {
    if (check()) return;
    await act(async () => {
      await Promise.resolve();
    });
  }
  throw new Error(message);
}

function liveAssistantContent() {
  return latestState?.streamState?.live_assistant_message?.content
    ?? latestState?.streamState?.live_turn_segments.find(
      (segment) => segment.kind === 'assistant_message',
    )?.assistant_message.content
    ?? '';
}

function dispatchSessionEvent(event: CodingSessionEvent, parentId = event.run_id) {
  window.dispatchEvent(new CustomEvent('coding_session_event-created', {
    detail: {
      entity_id: event.id,
      parent_id: parentId,
      data: event,
    },
  }));
}

describe('useAgentRunStream', () => {
  it('optimistically clears a submitted pending interaction across stale refetches', async () => {
    await renderProbe();

    await act(async () => {
      latestState?.clearPendingInteraction('interaction-1');
    });
    expect(container.textContent).toBe('none');

    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 1 },
      error: null,
    });
    await act(async () => {
      await latestState?.refetch();
    });

    expect(container.textContent).toBe('none');
  });

  it('shows a later pending interaction after a submitted one is cleared', async () => {
    await renderProbe();

    await act(async () => {
      latestState?.clearPendingInteraction('interaction-1');
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [interactionEvent(2, 'interaction-2')], next_sequence_no: 2 },
      error: null,
    });
    await act(async () => {
      await latestState?.refetch();
    });

    await waitForPending('interaction-2');
  });

  it('hydrates live stream state from the run-identified session snapshot', async () => {
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: {
        id: 'run-1',
        status: 'running',
        stream_state_snapshot: {
          live_turn_segments: [{
            segment_id: 'assistant-live-1:segment:1',
            kind: 'assistant_message',
            assistant_message: {
              message_id: 'assistant-live-1',
              content: 'Still inspecting the workspace',
              status: 'streaming',
              tool_calls: [],
            },
          }],
        },
      },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: {
        events: [],
        next_sequence_no: 0,
      },
      error: null,
    });

    await act(async () => {
      root.render(<Probe />);
    });

    for (let i = 0; i < 20; i += 1) {
      if (latestState?.streamState?.live_turn_segments[0]?.kind === 'assistant_message') break;
      await act(async () => {
        await Promise.resolve();
      });
    }

    const segment = latestState?.streamState?.live_turn_segments[0];
    expect(segment?.kind).toBe('assistant_message');
    if (segment?.kind === 'assistant_message') {
      expect(segment.assistant_message.content).toBe('Still inspecting the workspace');
    }
  });

  it('does not merge an event-list snapshot that may belong to a successor run', async () => {
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: {
        id: 'run-1',
        status: 'running',
        stream_state_snapshot: liveSnapshot('run one'),
      },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: {
        events: [assistantEvent(1, 'run two', {
          session_id: 'run-2',
          run_id: 'run-2',
          payload: { message_id: 'assistant-2', content: 'run two' },
        })],
        next_sequence_no: 1,
        stream_state_snapshot: liveSnapshot('run two'),
      },
      error: null,
    });

    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => liveAssistantContent() === 'run one', 'validated snapshot did not win');

    expect(liveAssistantContent()).toBe('run one');
  });

  it('applies v2 websocket deltas immediately using the parent run id', async () => {
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => !latestState?.loading, 'initial stream fetch did not finish');
    const snapshotCalls = mocks.getRunSnapshot.mock.calls.length;
    const eventCalls = mocks.listRunEvents.mock.calls.length;

    await act(async () => {
      dispatchSessionEvent(assistantEvent(1, ''));
      dispatchSessionEvent(assistantEvent(2, 'Hello'));
      dispatchSessionEvent(assistantEvent(3, ' '));
      dispatchSessionEvent(assistantEvent(4, 'world'));
    });

    expect(liveAssistantContent()).toBe('Hello world');
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(snapshotCalls);
    expect(mocks.listRunEvents).toHaveBeenCalledTimes(eventCalls);
  });

  it('replaces duplicate event ids and ignores events for another host run', async () => {
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => !latestState?.loading, 'initial stream fetch did not finish');

    const delta = assistantEvent(2, 'Hi');
    await act(async () => {
      dispatchSessionEvent(assistantEvent(1, ''));
      dispatchSessionEvent(delta);
      dispatchSessionEvent(assistantEvent(2, 'Hello', { id: delta.id }));
      dispatchSessionEvent(assistantEvent(3, 'wrong run', {
        session_id: 'run-2',
        run_id: 'run-2',
        payload: { message_id: 'assistant-2', text: 'wrong run', host_run_id: 'run-2' },
      }), 'run-2');
    });

    expect(liveAssistantContent()).toBe('Hello');
  });

  it('preserves v2 tool relationships while streaming', async () => {
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => !latestState?.loading, 'initial stream fetch did not finish');

    await act(async () => {
      dispatchSessionEvent(assistantEvent(1, ''));
      dispatchSessionEvent(assistantEvent(2, '', {
        id: 'tool-start',
        type: 'tool.call.started',
        payload: {
          host_run_id: 'run-1',
          parent_message_id: 'assistant-1',
          tool_call_id: 'tool-1',
          tool_name: 'fetch_url',
          args_text: '{"url":"https://example.com"}',
        },
      }));
      dispatchSessionEvent(assistantEvent(3, '', {
        id: 'tool-complete',
        type: 'tool.call.completed',
        payload: {
          host_run_id: 'run-1',
          parent_message_id: 'assistant-1',
          result_message_id: 'tool-result-1',
          tool_call_id: 'tool-1',
          tool_name: 'fetch_url',
          output_summary: 'Fetched page',
        },
      }));
    });

    const toolSegment = latestState?.streamState?.live_turn_segments.find(
      (segment) => segment.kind === 'tool_call',
    );
    expect(toolSegment?.kind).toBe('tool_call');
    if (toolSegment?.kind === 'tool_call') {
      expect(toolSegment.tool_call).toMatchObject({
        tool_call_id: 'tool-1',
        parent_message_id: 'assistant-1',
        status: 'completed',
        result: { output_summary: 'Fetched page' },
      });
    }
  });

  it('does not let a stale snapshot replace newer websocket state', async () => {
    const initialSnapshot = {
      through_sequence: 1,
      live_assistant_message: {
        message_id: 'assistant-1',
        content: 'Hello',
        status: 'streaming' as const,
        tool_calls: [],
      },
    };
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: initialSnapshot },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0, stream_state_snapshot: initialSnapshot },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => liveAssistantContent() === 'Hello', 'initial snapshot did not load');

    await act(async () => {
      dispatchSessionEvent(assistantEvent(2, ' world'));
    });
    expect(liveAssistantContent()).toBe('Hello world');

    const staleSnapshot = {
      ...initialSnapshot,
      live_assistant_message: {
        ...initialSnapshot.live_assistant_message,
        content: 'Stale',
      },
    };
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: staleSnapshot },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0, stream_state_snapshot: staleSnapshot },
      error: null,
    });
    await act(async () => {
      await latestState?.refetch();
    });

    expect(liveAssistantContent()).toBe('Hello world');
  });

  it('does not let a stale empty snapshot clear newer websocket state', async () => {
    const initialSnapshot = liveSnapshot('Hello', 1);
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: initialSnapshot },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => liveAssistantContent() === 'Hello', 'initial snapshot did not load');

    await act(async () => {
      dispatchSessionEvent(assistantEvent(2, ' world'));
    });
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: null },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      await latestState?.refetch();
    });

    expect(liveAssistantContent()).toBe('Hello world');
  });

  it('reconciles when the first observed v2 event starts after sequence one', async () => {
    vi.useFakeTimers();
    mocks.listRunEvents.mockResolvedValue({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
      await Promise.resolve();
    });
    const callsBeforeEvent = mocks.listRunEvents.mock.calls.length;

    await act(async () => {
      dispatchSessionEvent(assistantEvent(8, 'joined late'));
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(mocks.listRunEvents).toHaveBeenCalledTimes(callsBeforeEvent + 1);
    vi.useRealTimers();
  });

  it('holds an out-of-order v2 delta until the missing sequence arrives', async () => {
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => !latestState?.loading, 'initial stream fetch did not finish');

    await act(async () => {
      dispatchSessionEvent(assistantEvent(1, ''));
      dispatchSessionEvent(assistantEvent(3, 'world'));
    });
    expect(liveAssistantContent()).toBe('');

    await act(async () => {
      dispatchSessionEvent(assistantEvent(2, 'Hello '));
    });
    expect(liveAssistantContent()).toBe('Hello world');
  });

  it('uses an authoritative snapshot to bridge a non-visual runtime sequence gap', async () => {
    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: liveSnapshot('Hello ', 1) },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
    });
    await waitFor(() => liveAssistantContent() === 'Hello ', 'initial snapshot did not load');

    await act(async () => {
      dispatchSessionEvent(assistantEvent(3, 'world'));
    });
    expect(liveAssistantContent()).toBe('Hello ');

    mocks.getRunSnapshot.mockResolvedValueOnce({
      data: { id: 'run-1', status: 'running', stream_state_snapshot: liveSnapshot('Hello world', 3) },
      error: null,
    });
    mocks.listRunEvents.mockResolvedValueOnce({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      await latestState?.refetch();
    });

    expect(liveAssistantContent()).toBe('Hello world');
  });

  it('reconciles once when the v2 runtime sequence has a gap', async () => {
    vi.useFakeTimers();
    mocks.listRunEvents.mockResolvedValue({
      data: { events: [], next_sequence_no: 0 },
      error: null,
    });
    await act(async () => {
      root.render(<Probe />);
      await Promise.resolve();
    });
    const callsBeforeGap = mocks.listRunEvents.mock.calls.length;

    await act(async () => {
      dispatchSessionEvent(assistantEvent(1, ''));
      dispatchSessionEvent(assistantEvent(3, 'after gap'));
      dispatchSessionEvent(assistantEvent(4, ' once'));
      expect(liveAssistantContent()).toBe('');
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(mocks.listRunEvents).toHaveBeenCalledTimes(callsBeforeGap + 1);
    expect(liveAssistantContent()).toBe('');
    vi.useRealTimers();
  });

  it('does not let a predecessor run request overwrite a successor run', async () => {
    let resolveOldSnapshot!: (value: ReturnType<typeof snapshotResponse>) => void;
    let resolveOldEvents!: (value: ReturnType<typeof eventResponse>) => void;
    const oldSnapshot = new Promise<ReturnType<typeof snapshotResponse>>((resolve) => {
      resolveOldSnapshot = resolve;
    });
    const oldEvents = new Promise<ReturnType<typeof eventResponse>>((resolve) => {
      resolveOldEvents = resolve;
    });
    mocks.getRunSnapshot.mockImplementation((_workspaceId, runId) => (
      runId === 'run-1' ? oldSnapshot : Promise.resolve(snapshotResponse('run-2'))
    ));
    mocks.listRunEvents.mockImplementation((_workspaceId, runId) => (
      runId === 'run-1'
        ? oldEvents
        : Promise.resolve(eventResponse([assistantEvent(1, 'new run', {
          id: 'run-2-message',
          session_id: 'run-2',
          run_id: 'run-2',
          type: 'assistant.message.completed',
          payload: { message_id: 'assistant-2', content: 'new run' },
          runtime_metadata: { source: 'agent_run_message' },
        })]))
    ));

    await act(async () => {
      root.render(<Probe runId="run-1" />);
      await Promise.resolve();
      root.render(<Probe runId="run-2" />);
    });
    await waitFor(
      () => latestState?.streamState?.transcript_messages[0]?.content === 'new run',
      'successor run did not load',
    );

    await act(async () => {
      resolveOldSnapshot(snapshotResponse('run-1'));
      resolveOldEvents(eventResponse([assistantEvent(1, 'old run', {
        type: 'assistant.message.completed',
        runtime_metadata: { source: 'agent_run_message' },
      })]));
      await Promise.resolve();
    });

    expect(latestState?.streamState?.transcript_messages.map((message) => message.content))
      .toEqual(['new run']);
  });
});

function snapshotResponse(runId = 'run-1') {
  return {
    data: { id: runId, status: 'running', stream_state_snapshot: null },
    error: null,
  };
}

function eventResponse(events: CodingSessionEvent[] = []) {
  return {
    data: { events, next_sequence_no: events.length },
    error: null,
  };
}

function liveSnapshot(content: string, throughSequence = 1) {
  return {
    through_sequence: throughSequence,
    live_assistant_message: {
      message_id: 'assistant-1',
      content,
      status: 'streaming' as const,
      tool_calls: [],
    },
  };
}

describe('stream request lifecycle', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
    useSupportPresenceStore.getState().setWsConnected(false);
    mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-1', status: 'paused', pause_reason: 'awaiting_user_message' }, error: null });
    mocks.listRunEvents.mockResolvedValue({ data: { events: [], next_sequence_no: 0 }, error: null });
  });

  afterEach(() => {
    vi.useRealTimers();
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
    Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
  });

  it('shares the initial transcript read across two consumers of the same run', async () => {
    let finish!: (value: unknown) => void;
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    await act(async () => { root.render(<><Probe /><Probe /></>); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
    expect(mocks.listRunEvents).toHaveBeenCalledTimes(1);
    await act(async () => { finish({ data: { id: 'run-1', status: 'completed' }, error: null }); });
    expect(latestState?.session?.status).toBe('completed');
  });

  it('gets a fresh post-action response when another dock has a pre-action read pending', async () => {
    const consumers: Array<ReturnType<typeof useAgentRunStream>> = [];
    function Consumer({ index }: { index: number }) {
      consumers[index] = useAgentRunStream('ws-1', 'run-1', true, 0);
      return null;
    }
    await act(async () => { root.render(<><Consumer index={0} /><Consumer index={1} /></>); });
    mocks.getRunSnapshot.mockClear();
    let finish!: (value: unknown) => void;
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    let first!: Promise<void>;
    let second!: Promise<void>;
    await act(async () => { first = consumers[0].refetch(); });
    // A message has now been accepted while the other dock is still fetching.
    mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-1', status: 'running' }, error: null });
    await act(async () => { second = consumers[1].refetch(); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
    await act(async () => {
      finish({ data: { id: 'run-1', status: 'paused' }, error: null });
      await Promise.all([first, second]);
    });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
    expect(consumers[1].session?.status).toBe('running');
  });

  it('shares a fresh follow-up when both docks were invalidated during the same read', async () => {
    const consumers: Array<ReturnType<typeof useAgentRunStream>> = [];
    function Consumer({ index }: { index: number }) {
      consumers[index] = useAgentRunStream('ws-1', 'run-1', true, 0);
      return null;
    }
    let finish!: (value: unknown) => void;
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    await act(async () => { root.render(<><Consumer index={0} /><Consumer index={1} /></>); });
    let followups: Promise<void>[] = [];
    await act(async () => { followups = consumers.map((consumer) => consumer.refetch()); });
    await act(async () => {
      finish({ data: { id: 'run-1', status: 'running' }, error: null });
      await Promise.all(followups);
    });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
  });

  it.each(['notification', 'focus', 'poll', 'visibility'])(
    'shares one refresh across idle consumers after %s', async (trigger) => {
      mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-1', status: 'running' }, error: null });
      await act(async () => { root.render(<><Probe pollMs={5000} /><Probe pollMs={5000} /></>); });
      expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
      let finish!: (value: unknown) => void;
      mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
      if (trigger === 'visibility') {
        await act(async () => {
          Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' });
          document.dispatchEvent(new Event('visibilitychange'));
        });
      }
      await act(async () => {
        if (trigger === 'notification') window.dispatchEvent(new CustomEvent('agent_run-updated', { detail: { entity_id: 'run-1' } }));
        if (trigger === 'focus') window.dispatchEvent(new Event('focus'));
        if (trigger === 'visibility') {
          Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
          document.dispatchEvent(new Event('visibilitychange'));
        }
        await vi.advanceTimersByTimeAsync(trigger === 'poll' ? 5000 : 100);
      });
      await act(async () => { finish({ data: { id: 'run-1', status: 'running' }, error: null }); });
      expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
    },
  );

  it('coalesces requests arriving during a fetch into one ordered follow-up', async () => {
    let finish!: (value: unknown) => void;
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    await act(async () => { root.render(<Probe />); });
    let followups: Array<Promise<void> | undefined> = [];
    await act(async () => { followups = [latestState?.refetch(), latestState?.refetch(), latestState?.refetch()]; });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
    expect(mocks.listRunEvents).toHaveBeenCalledTimes(1);
    mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-1', status: 'completed' }, error: null });
    await act(async () => {
      finish({ data: { id: 'run-1', status: 'running' }, error: null });
      await Promise.all(followups);
    });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
    expect(latestState?.session?.status).toBe('completed');
  });

  it('settles an explicit refresh after its own cycle while slow background polling continues', async () => {
    mocks.getRunSnapshot.mockImplementation(() => new Promise((resolve) => {
      setTimeout(() => resolve({ data: { id: 'run-1', status: 'running' }, error: null }), 10000);
    }));
    await act(async () => { root.render(<Probe pollMs={5000} />); });
    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    let finished = false;
    await act(async () => { void latestState?.refetch().then(() => { finished = true; }); });
    await act(async () => { await vi.advanceTimersByTimeAsync(24000); });
    expect(mocks.getRunSnapshot.mock.calls.length).toBeGreaterThanOrEqual(3);
    expect(finished).toBe(true);
  });

  it('drops an automatic queued follow-up when the dock closes', async () => {
    let finish!: (value: unknown) => void;
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    await act(async () => { root.render(<Probe />); });
    await act(async () => {
      window.dispatchEvent(new CustomEvent('agent_run-updated', { detail: { entity_id: 'run-1' } }));
      await vi.advanceTimersByTimeAsync(100);
    });
    await act(async () => { root.render(<Probe active={false} />); });
    await act(async () => { finish({ data: { id: 'run-1', status: 'running' }, error: null }); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
  });

  it('retires hung network work with staggered polling consumers', async () => {
    mocks.getRunSnapshot.mockImplementation(() => new Promise(() => {}));
    await act(async () => { root.render(<><Probe key="first" pollMs={5000} /></>); });
    await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
    await act(async () => { root.render(<><Probe key="first" pollMs={5000} /><Probe key="second" pollMs={5000} /></>); });
    const firstSignal = mocks.getRunSnapshot.mock.calls[0][2] as AbortSignal;
    await act(async () => { await vi.advanceTimersByTimeAsync(120000); });
    expect(firstSignal.aborted).toBe(true);
    expect(mocks.getRunSnapshot.mock.calls.length).toBeGreaterThanOrEqual(4);
  });

  it('releases a timed-out read and allows explicit recovery', async () => {
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise(() => {}));
    await act(async () => { root.render(<Probe />); });
    const signal = mocks.getRunSnapshot.mock.calls[0][2] as AbortSignal;
    await act(async () => { await vi.advanceTimersByTimeAsync(30000); });
    expect(signal.aborted).toBe(true);
    expect(latestState?.loading).toBe(false);
    await act(async () => { await latestState?.refetch(); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
  });

  it('does not poll paused chats every five seconds but retains a slow recovery fallback', async () => {
    await act(async () => { root.render(<Probe pollMs={5000} />); });
    await act(async () => { await vi.advanceTimersByTimeAsync(59000); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
    await act(async () => { await vi.advanceTimersByTimeAsync(1000); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
  });

  it('keeps the five-second recovery fallback for an active run', async () => {
    mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-1', status: 'running' }, error: null });
    await act(async () => { root.render(<Probe pollMs={5000} />); });
    await act(async () => { await vi.advanceTimersByTimeAsync(5000); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
  });

  it('suppresses hidden automatic fetches including socket notifications and recovers on return', async () => {
    Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'hidden' });
    await act(async () => { root.render(<Probe pollMs={5000} />); });
    await act(async () => {
      window.dispatchEvent(new CustomEvent('agent_run-updated', { detail: { entity_id: 'run-1', status: 'completed' } }));
      await vi.advanceTimersByTimeAsync(65000);
    });
    expect(mocks.getRunSnapshot).not.toHaveBeenCalled();
    await act(async () => {
      Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
      document.dispatchEvent(new Event('visibilitychange'));
      await vi.advanceTimersByTimeAsync(100);
    });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
  });

  it('reconciles completed runs after offline recovery and websocket reconnect', async () => {
    mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-1', status: 'completed' }, error: null });
    await act(async () => { root.render(<Probe />); });
    await act(async () => {
      Object.defineProperty(navigator, 'onLine', { configurable: true, value: false });
      window.dispatchEvent(new Event('offline'));
    });
    await act(async () => {
      Object.defineProperty(navigator, 'onLine', { configurable: true, value: true });
      window.dispatchEvent(new Event('online'));
      await vi.advanceTimersByTimeAsync(100);
    });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
    await act(async () => {
      useSupportPresenceStore.getState().setWsConnected(true);
      await vi.advanceTimersByTimeAsync(100);
    });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(3);
  });

  it('reconciles on focus without requiring a polling timer', async () => {
    await act(async () => { root.render(<Probe />); });
    await act(async () => { window.dispatchEvent(new Event('focus')); await vi.advanceTimersByTimeAsync(100); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(2);
  });

  it('keeps explicit action reconciliation available while inactive', async () => {
    await act(async () => { root.render(<Probe active={false} />); });
    expect(mocks.getRunSnapshot).not.toHaveBeenCalled();
    await act(async () => { await latestState?.refetch(); });
    expect(mocks.getRunSnapshot).toHaveBeenCalledTimes(1);
  });

  it('aborts obsolete requests on a run switch and rejects their late response', async () => {
    let finish!: (value: unknown) => void;
    mocks.getRunSnapshot.mockReturnValueOnce(new Promise((resolve) => { finish = resolve; }));
    await act(async () => { root.render(<Probe />); });
    const signal = mocks.getRunSnapshot.mock.calls[0][2] as AbortSignal;
    mocks.getRunSnapshot.mockResolvedValue({ data: { id: 'run-2', status: 'completed' }, error: null });
    await act(async () => { root.render(<Probe runId="run-2" />); });
    expect(signal?.aborted).toBe(true);
    await act(async () => { finish({ data: { id: 'run-1', status: 'running' }, error: null }); });
    expect(latestState?.session?.id).toBe('run-2');
  });
});
