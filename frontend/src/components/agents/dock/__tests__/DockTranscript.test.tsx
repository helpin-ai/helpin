// @vitest-environment jsdom
import { act } from 'react';
import { createRoot, type Root } from 'react-dom/client';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';

import type { CodingSessionStreamState, CodingSessionTranscriptMessage } from '@/lib/pmTypes';
import { DockTranscript } from '../DockTranscript';

(globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

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
});

function assistantMessage(id: string, content: string, sequenceNo: number): CodingSessionTranscriptMessage {
  return {
    event_id: id,
    message_id: id,
    role: 'assistant',
    message_type: 'message',
    content,
    timestamp: `2026-08-06T00:00:0${sequenceNo}Z`,
    sequence_no: sequenceNo,
  };
}

function streamWithMessages(messages: CodingSessionTranscriptMessage[]): CodingSessionStreamState {
  return {
    transcript_messages: messages,
    live_assistant_message: null,
    live_reasoning_message: null,
    live_turn_segments: [],
    activity_events: [],
    current_plan: null,
    completed_tool_calls: [],
  };
}

describe('DockTranscript', () => {
  it('never collapses the latest assistant message', () => {
    const longOlderMessage = `Older response ${'old '.repeat(180)}`;
    const longLatestMessage = `Latest response ${'new '.repeat(180)}`;

    act(() => {
      root.render(
        <DockTranscript
          stream={streamWithMessages([
            assistantMessage('assistant-1', longOlderMessage, 1),
            assistantMessage('assistant-2', longLatestMessage, 2),
          ])}
          active={false}
        />,
      );
    });

    const assistantMessages = container.querySelectorAll('.group\\/assistant');
    expect(assistantMessages).toHaveLength(2);
    expect(assistantMessages[0]?.textContent).toContain('Show more');
    expect(assistantMessages[1]?.textContent).not.toContain('Show more');
    expect(assistantMessages[1]?.textContent).not.toContain('Show less');
  });
});
