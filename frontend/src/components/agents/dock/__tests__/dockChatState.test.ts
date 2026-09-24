import { describe, expect, it } from 'vitest';
import { isDockTranscriptStreaming, resolveDockComposerState, transformDockStream } from '../dockChatState';
import { parseDockChildResult, parseDockPlanConfirm, stripDockPageContext } from '@/lib/dockTypes';
import { collectSegments, DOCK_SEGMENT_KINDS } from '@/components/agents/transcript';
import type { CodingSessionStreamState } from '@/lib/pmTypes';

function emptyStream(): CodingSessionStreamState {
  return {
    transcript_messages: [],
    live_assistant_message: null,
    live_reasoning_message: null,
    live_turn_segments: [],
    activity_events: [],
    current_plan: null,
    completed_tool_calls: [],
  };
}

describe('resolveDockComposerState', () => {
  it('enables the composer for a chat with no run yet', () => {
    const state = resolveDockComposerState(null, false, false);
    expect(state.visible).toBe(true);
    expect(state.enabled).toBe(true);
  });

  it('enables replies while paused awaiting the user', () => {
    const state = resolveDockComposerState({ status: 'paused', pause_reason: 'awaiting_user_message' }, false, false);
    expect(state.enabled).toBe(true);
  });

  it('disables while the agent is working', () => {
    const state = resolveDockComposerState({ status: 'running', pause_reason: 'none' }, false, false);
    expect(state.visible).toBe(true);
    expect(state.enabled).toBe(false);
  });

  it('keeps the composer mounted but disabled during a structured interaction', () => {
    const state = resolveDockComposerState({ status: 'paused', pause_reason: 'human_approval' }, true, false);
    expect(state.visible).toBe(true);
    expect(state.enabled).toBe(false);
  });

  it('enables continuation after the run ended', () => {
    const state = resolveDockComposerState({ status: 'completed', pause_reason: 'none' }, false, false);
    expect(state.enabled).toBe(true);
  });
});

describe('isDockTranscriptStreaming', () => {
  it('treats queued and running runs as streaming', () => {
    expect(isDockTranscriptStreaming({ status: 'queued', pause_reason: '' })).toBe(true);
    expect(isDockTranscriptStreaming({ status: 'running', pause_reason: '' })).toBe(true);
  });

  it('treats an awaiting-user pause as idle', () => {
    expect(isDockTranscriptStreaming({ status: 'paused', pause_reason: 'awaiting_user_message' })).toBe(false);
  });
});

describe('transformDockStream', () => {
  it('strips page context from user messages and extracts child results', () => {
    const stream = emptyStream();
    stream.transcript_messages = [
      {
        event_id: 'e1',
        role: 'user',
        message_type: 'prompt',
        content: 'what is up?\n\n<page_context>{"entity_type":"task"}</page_context>',
        timestamp: 't',
        sequence_no: 1,
      },
      {
        event_id: 'e2',
        role: 'user',
        content: '<child_run_result>{"plan_id":"p1","status":"completed","runs":[]}</child_run_result>',
        timestamp: 't',
        sequence_no: 2,
      },
      { event_id: 'e3', role: 'assistant', content: 'done', timestamp: 't', sequence_no: 3 },
    ];
    const result = transformDockStream(stream);
    expect(result.stream.transcript_messages).toHaveLength(2);
    expect(result.stream.transcript_messages[0].content).toBe('what is up?');
    expect(collectSegments(result.stream, { includeLive: false, include: DOCK_SEGMENT_KINDS })[0])
      .toMatchObject({ kind: 'user', message: { content: 'what is up?' } });
    expect(result.childResults).toHaveLength(1);
    expect(result.childResults[0].result.plan_id).toBe('p1');
  });

  it('keeps Ask media as attachment metadata, not prompt text', () => {
    const stream = emptyStream();
    stream.transcript_messages = [{
      event_id: 'media-user-turn',
      role: 'user',
      content: 'What is shown here?\n\n<attachments>[{"id":"media-1","file_name":"screen.png","file_type":"image/png","file_size":123}]</attachments>',
      timestamp: 't',
      sequence_no: 1,
    }];

    const result = transformDockStream(stream);

    expect(result.stream.transcript_messages[0]).toMatchObject({
      content: 'What is shown here?',
      attachments: [{ id: 'media-1', file_name: 'screen.png', file_type: 'image/png' }],
    });
  });

  it('hides successor-run carry-forward blocks while preserving the latest user turn', () => {
    const stream = emptyStream();
    stream.transcript_messages = [
      {
        event_id: 'successor-user-turn',
        role: 'user',
        content: [
          '<previous_conversation>',
          'This chat continues an earlier conversation whose run ended (completed). Recent transcript:',
          'user: Which tasks are stale?',
          'assistant: Three tasks are stale.',
          '</previous_conversation>',
          '',
          '<child_run_result>{"plan_id":"p1","status":"completed","runs":[]}</child_run_result>',
          '',
          'Can you assign those tasks to me?',
          '',
          '<page_context>{"entity_type":"task"}</page_context>',
        ].join('\n'),
        timestamp: '2026-08-05T10:00:00Z',
        sequence_no: 1,
      },
    ];

    const result = transformDockStream(stream);

    expect(result.stream.transcript_messages).toHaveLength(1);
    expect(result.stream.transcript_messages[0].content).toBe('Can you assign those tasks to me?');
  });
});

describe('transformDockStream ordering', () => {
  it('orders messages by timestamp when sequence lags conversation order', () => {
    const stream = emptyStream();
    stream.transcript_messages = [
      // Assistant reply arrived at the projection first (lower sequence)
      // even though the user message it answers is older.
      { event_id: 'a', role: 'assistant', content: 'answer', timestamp: '2026-08-03T07:22:27Z', sequence_no: 9 },
      { event_id: 'u', role: 'user', content: 'question', timestamp: '2026-08-03T07:22:19Z', sequence_no: 10 },
    ];
    const result = transformDockStream(stream);
    expect(result.stream.transcript_messages.map((m) => m.content)).toEqual(['question', 'answer']);
    expect(collectSegments(result.stream, { includeLive: false, include: DOCK_SEGMENT_KINDS }).map((segment) => segment.kind))
      .toEqual(['user', 'assistant']);
  });

  it('interleaves turns when assistant replies were projected in one late batch', () => {
    const stream = emptyStream();
    stream.transcript_messages = [
      { event_id: 'u1', role: 'user', content: 'q1', timestamp: '2026-08-03T07:00:00Z', sequence_no: 1 },
      { event_id: 'u2', role: 'user', content: 'q2', timestamp: '2026-08-03T07:05:00Z', sequence_no: 2 },
      // Replayed assistant events keep their original sent_at even though they
      // were persisted together after both user messages.
      { event_id: 'a1', role: 'assistant', content: 'a1', timestamp: '2026-08-03T07:01:00Z', sequence_no: 3 },
      { event_id: 'a2', role: 'assistant', content: 'a2', timestamp: '2026-08-03T07:06:00Z', sequence_no: 4 },
    ];
    const result = transformDockStream(stream);
    expect(result.stream.transcript_messages.map((m) => m.content)).toEqual(['q1', 'a1', 'q2', 'a2']);
  });

  it('falls back to sequence for identical timestamps', () => {
    const stream = emptyStream();
    stream.transcript_messages = [
      { event_id: 'b', role: 'assistant', content: 'second', timestamp: '2026-08-03T07:00:00Z', sequence_no: 2 },
      { event_id: 'a', role: 'user', content: 'first', timestamp: '2026-08-03T07:00:00Z', sequence_no: 1 },
    ];
    const result = transformDockStream(stream);
    expect(result.stream.transcript_messages.map((m) => m.content)).toEqual(['first', 'second']);
  });
});

describe('dock marker parsing', () => {
  it('stripDockPageContext leaves plain content unchanged', () => {
    expect(stripDockPageContext('hello')).toBe('hello');
  });

  it('stripDockPageContext hides page context and supplemental references', () => {
    expect(stripDockPageContext([
      'compare these',
      '<page_context>{"entity_type":"workspace"}</page_context>',
      '<references>[{"entity_type":"task","entity_id":"task-1"}]</references>',
    ].join('\n\n'))).toBe('compare these');
  });

  it('parseDockChildResult rejects non-result content', () => {
    expect(parseDockChildResult('hello')).toBeNull();
    expect(parseDockChildResult('<child_run_result>not json</child_run_result>')).toBeNull();
  });

  it('parseDockChildResult preserves result recovery metadata', () => {
    const parsed = parseDockChildResult(
      '<child_run_result>{"plan_id":"p1","status":"completed","runs":[{"run_id":"r1","status":"completed","summary":"short","summary_truncated":true,"summary_char_count":4200,"result_available":true,"artifacts":[{"artifact_id":"a1","artifact_type":"review_findings","format":"json","storage_mode":"inline"}]}]}</child_run_result>',
    );
    expect(parsed?.runs[0]).toMatchObject({
      run_id: 'r1',
      summary_truncated: true,
      summary_char_count: 4200,
      result_available: true,
    });
    expect(parsed?.runs[0].artifacts?.[0].artifact_id).toBe('a1');
  });

  it('parseDockPlanConfirm requires the dock kind', () => {
    expect(parseDockPlanConfirm({ kind: 'other' })).toBeNull();
    expect(parseDockPlanConfirm({ kind: 'dock_plan_confirm', summary: 's' })?.summary).toBe('s');
    expect(parseDockPlanConfirm({
      phase: 'dock_execution_confirm',
      summary: 'Create a document',
      action: { proposal_id: 'proposal-1' },
    })?.action?.proposal_id).toBe('proposal-1');
  });
});

it('cleans chat answers and live segments without modifying the source used by agent runs', () => {
  const marker = '<!-- helpin_follow_up_suggestions ["Review the task"] -->';
  const content = `Done\n\n${marker}`;
  const stream = emptyStream();
  const live = { message_id: 'reply', content, status: 'streaming' as const, tool_calls: [] };
  const segment = { segment_id: 'seg', kind: 'assistant_message' as const, assistant_message: live };
  stream.transcript_messages = [
    { event_id: 'user', role: 'user', content: marker, timestamp: '', sequence_no: 1 },
    { event_id: 'assistant', role: 'assistant', content, timestamp: '', sequence_no: 2, turn_segments: [segment] },
  ];
  stream.live_assistant_message = live;
  stream.live_turn_segments = [segment];
  const result = transformDockStream(stream).stream;
  expect(result.transcript_messages[0].content).toBe(marker);
  expect(result.transcript_messages[1].content).toBe('Done');
  expect(result.live_assistant_message?.content).toBe('Done');
  expect(result.live_turn_segments[0]).toMatchObject({ assistant_message: { content: 'Done' } });
  expect(result.transcript_messages[1].turn_segments?.[0]).toMatchObject({ assistant_message: { content: 'Done' } });
  expect(stream.transcript_messages[1].content).toBe(content);
  expect(stream.live_assistant_message.content).toBe(content);
  expect(stream.live_turn_segments[0]).toMatchObject({ assistant_message: { content } });
});
