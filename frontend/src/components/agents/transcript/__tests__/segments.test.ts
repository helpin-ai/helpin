import { describe, expect, it } from 'vitest';

import {
  ALL_SEGMENT_KINDS,
  DOCK_SEGMENT_KINDS,
  collectSegments,
  hasRenderableSegments,
  type TranscriptStreamInput,
} from '../segments';
import type {
  CodingSessionLiveToolCall,
  CodingSessionLiveTurnSegment,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';

function toolCall(overrides: Partial<CodingSessionLiveToolCall> = {}): CodingSessionLiveToolCall {
  return {
    tool_call_id: 'tc-1',
    tool_name: 'read_file',
    args_text: '{"path":"a.ts"}',
    status: 'completed',
    ...overrides,
  };
}

function assistantSegment(id: string, content: string): CodingSessionLiveTurnSegment {
  return {
    segment_id: id,
    kind: 'assistant_message',
    assistant_message: { message_id: id, content, status: 'completed', tool_calls: [] },
  };
}

function toolSegment(id: string, tc: CodingSessionLiveToolCall): CodingSessionLiveTurnSegment {
  return { segment_id: id, kind: 'tool_call', tool_call: tc };
}

function message(overrides: Partial<CodingSessionTranscriptMessage> = {}): CodingSessionTranscriptMessage {
  return {
    event_id: 'event-1',
    role: 'assistant',
    content: '',
    timestamp: '2026-06-15T00:00:00Z',
    sequence_no: 1,
    ...overrides,
  };
}

function stream(overrides: Partial<TranscriptStreamInput> = {}): TranscriptStreamInput {
  return {
    transcript_messages: [],
    live_turn_segments: [],
    live_reasoning_message: null,
    ...overrides,
  };
}

describe('collectSegments', () => {
  it('interleaves assistant text and tool calls from turn_segments in order', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          turn_segments: [
            assistantSegment('a1', 'First'),
            toolSegment('t1', toolCall({ tool_call_id: 'tc-1' })),
            assistantSegment('a2', 'Second'),
          ],
        })],
      }),
      { includeLive: false },
    );

    expect(segments.map((s) => s.kind)).toEqual(['assistant', 'tool', 'assistant']);
    expect(segments[0]).toMatchObject({ kind: 'assistant', content: 'First' });
    expect(segments[2]).toMatchObject({ kind: 'assistant', content: 'Second' });
  });

  it('drops update_plan tool calls (including mcp-prefixed names)', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          turn_segments: [
            toolSegment('t1', toolCall({ tool_call_id: 'tc-keep', tool_name: 'read_file' })),
            toolSegment('t2', toolCall({ tool_call_id: 'tc-drop', tool_name: 'mcp__helpin__update_plan' })),
          ],
        })],
      }),
      { includeLive: false },
    );

    expect(segments).toHaveLength(1);
    expect(segments[0]).toMatchObject({ kind: 'tool' });
  });

  it('emits persisted tool_calls absent from the segment timeline, deduped by content', () => {
    const sharedArgs = '{"path":"frontend/src/App.tsx"}';
    const segments = collectSegments(
      stream({
        transcript_messages: [message({
          turn_segments: [
            assistantSegment('a1', 'Applying the change.'),
            toolSegment('t1', toolCall({ tool_call_id: 'seg-read', tool_name: 'read_file', args_text: sharedArgs })),
          ],
          tool_calls: [
            // Same content as the timeline read → deduped away.
            toolCall({ tool_call_id: 'fallback-read', tool_name: 'read_file', args_text: sharedArgs }),
            // Only persisted on the message → must still render.
            toolCall({ tool_call_id: 'fallback-patch', tool_name: 'apply_patch', args_text: '*** Begin Patch' }),
          ],
        })],
      }),
      { includeLive: false },
    );

    const toolIds = segments.filter((s) => s.kind === 'tool').map((s) => (s.kind === 'tool' ? s.toolCall.tool_call_id : ''));
    expect(toolIds).toEqual(['seg-read', 'fallback-patch']);
  });

  it('includes live reasoning + turn segments only when includeLive is true', () => {
    const input = stream({
      live_reasoning_message: { message_id: 'r1', content: 'thinking', status: 'streaming' },
      live_turn_segments: [{
        segment_id: 'live-a',
        kind: 'assistant_message',
        assistant_message: { message_id: 'live-a', content: 'Live answer', status: 'streaming', tool_calls: [] },
      }],
    });

    expect(collectSegments(input, { includeLive: false })).toHaveLength(0);

    const live = collectSegments(input, { includeLive: true });
    expect(live.map((s) => s.kind)).toEqual(['reasoning', 'assistant']);
    expect(live[1]).toMatchObject({ kind: 'assistant', content: 'Live answer', streaming: true });
  });

  it('scopes kinds via the include set (dock = assistant + tool only)', () => {
    const input = stream({
      transcript_messages: [
        message({ message_type: 'status', role: 'assistant', content: 'Preparing workspace' }),
        message({ event_id: 'event-2', turn_segments: [assistantSegment('a1', 'Done')] }),
      ],
    });

    const all = collectSegments(input, { includeLive: false, include: ALL_SEGMENT_KINDS });
    expect(all.map((s) => s.kind)).toEqual(['status', 'assistant']);

    const dock = collectSegments(input, { includeLive: false, include: DOCK_SEGMENT_KINDS });
    expect(dock.map((s) => s.kind)).toEqual(['assistant']);
  });

  it('classifies status / context / user / review-decision messages', () => {
    const segments = collectSegments(
      stream({
        transcript_messages: [
          message({ event_id: 'ctx', role: 'user', message_type: 'developer_prompt', content: 'prompt' }),
          message({ event_id: 'status', message_type: 'status', content: 'Working' }),
          message({ event_id: 'user', role: 'user', message_type: 'message', content: 'hi' }),
          message({ event_id: 'review', role: 'user', message_type: 'approval_request_resolution', content: 'Approved.' }),
        ],
      }),
      { includeLive: false },
    );

    expect(segments.map((s) => s.kind)).toEqual(['context', 'status', 'user', 'review_decision']);
  });

  it('prepends leadingContext when context is in scope', () => {
    const segments = collectSegments(
      stream({ transcript_messages: [message({ turn_segments: [assistantSegment('a1', 'Hi')] })] }),
      {
        includeLive: false,
        leadingContext: message({ event_id: 'lead', message_type: 'system_prompt', content: 'system' }),
      },
    );

    expect(segments[0]).toMatchObject({ kind: 'context', id: 'context:lead' });
    expect(segments[1]).toMatchObject({ kind: 'assistant' });
  });
});

describe('hasRenderableSegments', () => {
  it('is false for a null stream and true once a segment exists', () => {
    expect(hasRenderableSegments(null, { includeLive: true })).toBe(false);
    expect(
      hasRenderableSegments(
        stream({ transcript_messages: [message({ turn_segments: [assistantSegment('a1', 'Hi')] })] }),
        { includeLive: false, include: DOCK_SEGMENT_KINDS },
      ),
    ).toBe(true);
  });
});
