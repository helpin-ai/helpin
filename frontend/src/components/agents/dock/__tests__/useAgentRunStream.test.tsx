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

function Probe() {
  latestState = useAgentRunStream('ws-1', 'run-1', true, 0);
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
});
