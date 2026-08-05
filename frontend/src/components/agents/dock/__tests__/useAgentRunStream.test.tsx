// @vitest-environment jsdom
import React, { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { useAgentRunStream } from '../useAgentRunStream';
import type { CodingSessionEvent } from '@/lib/pmTypes';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

const mocks = vi.hoisted(() => ({
  getRunSnapshot: vi.fn(),
  listRunEvents: vi.fn(),
}));

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

function Probe({ runId = 'run-1' }: { runId?: string }) {
  latestState = useAgentRunStream('ws-1', runId, true, 0);
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
      await vi.advanceTimersByTimeAsync(100);
    });

    expect(mocks.listRunEvents).toHaveBeenCalledTimes(callsBeforeGap + 1);
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
