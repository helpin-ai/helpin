import { describe, expect, it } from 'vitest';
import type {
  AgentRunMessage,
  CodingSessionLiveTurnSegment,
  CodingSessionStreamState,
  CodingSessionTranscriptMessage,
} from '@/lib/pmTypes';
import {
  hasAuthoritativeDockRuntimeTimeline,
  mergeMessagePages,
  mergePersistedChatMessages,
  resolveVisiblePendingEcho,
} from '../dockChatTimeline';

function persisted(sequence: number, content: string, createdAt: string): AgentRunMessage {
  return {
    id: `persisted-${sequence}`,
    workspace_id: 'ws-1',
    run_id: 'run-1',
    dock_chat_id: 'chat-1',
    dock_chat_sequence: sequence,
    role: sequence % 2 ? 'user' : 'assistant',
    content,
    message_type: sequence % 2 ? 'user_reply' : 'assistant_turn',
    sequence_no: sequence,
    created_at: createdAt,
  };
}

function transcript(sequence: number, content: string, createdAt: string): CodingSessionTranscriptMessage {
  return {
    event_id: `runtime-${sequence}`,
    message_id: `runtime-${sequence}`,
    role: sequence % 2 ? 'user' : 'assistant',
    content,
    message_type: 'message',
    sequence_no: sequence,
    timestamp: createdAt,
  };
}

function stream(
  transcriptMessages: CodingSessionTranscriptMessage[],
  liveTurnSegments: CodingSessionLiveTurnSegment[] = [],
): CodingSessionStreamState {
  return {
    transcript_messages: transcriptMessages,
    live_assistant_message: null,
    live_reasoning_message: null,
    live_turn_segments: liveTurnSegments,
    activity_events: [],
    current_plan: null,
    completed_tool_calls: [],
  };
}

describe('mergePersistedChatMessages', () => {
  it('does not append an uncorrelated runtime user echo beside the accepted message', () => {
    const saved = { ...persisted(1, 'Continue', '2026-09-21T09:00:00Z'), client_message_id: 'submission-1' };
    const echo = transcript(3, 'Continue', '2026-09-21T09:00:00.100Z');
    expect(mergePersistedChatMessages(stream([echo]), [saved])?.transcript_messages.map(m => m.content)).toEqual(['Continue']);
  });

  it('keeps distinct identical submissions and accepts a durable websocket row before its page', () => {
    const saved = { ...persisted(1, 'Continue', '2026-09-21T09:00:00Z'), client_message_id: 'submission-1' };
    const incoming = { ...transcript(3, 'Continue', '2026-09-21T09:00:01Z'), event_id: 'msg:saved-2', client_message_id: 'submission-2' };
    expect(mergePersistedChatMessages(stream([incoming]), [saved])?.transcript_messages).toHaveLength(2);
  });

  it('reconciles message pages by submission identity even when row IDs differ', () => {
    const saved = { ...persisted(1, 'Continue', '2026-09-21T09:00:00Z'), client_message_id: 'submission-1' };
    const confirmed = { ...saved, id: 'canonical-row', delivery_status: 'sent' as const };
    expect(mergeMessagePages([saved], [confirmed])).toEqual([confirmed]);
  });

  it('restores a locally failed message when delivery is authoritatively confirmed', () => {
    const confirmed = { ...persisted(2, 'Delivered', '2026-08-15T10:00:00Z'),
      role: 'user' as const, client_message_id: 'lost-ack', delivery_status: 'sent' as const };
    const result = mergePersistedChatMessages(null, [confirmed], { failedClientMessageIds: new Set(['lost-ack']) });
    expect(result?.transcript_messages.map((message) => message.content)).toEqual(['Delivered']);
  });

  it('keeps one client-keyed user row through optimistic, realtime and saved delivery', () => {
    const pending = { id: 'client-new', content: 'yes', timestamp: '2026-09-07T10:00:01Z', actor_user_id: 'user-1' };
    const previous = { ...persisted(1, 'yes', '2026-09-07T10:00:00Z'), client_message_id: 'client-old' };
    const incoming = { ...transcript(3, 'yes', '2026-09-07T10:00:02Z'), event_id: 'msg:saved-new',
      message_id: 'saved-new', client_message_id: pending.id, delivery_status: 'pending' as const };
    const saved = { ...persisted(3, 'yes', incoming.timestamp), id: 'saved-new',
      client_message_id: pending.id, delivery_status: 'sent' as const };
    const states = [
      mergePersistedChatMessages(null, [previous], { pendingMessage: pending }),
      mergePersistedChatMessages(stream([incoming]), [previous], { pendingMessage: pending }),
      mergePersistedChatMessages(stream([incoming]), [previous, saved]),
    ];
    for (const state of states) {
      expect(state?.transcript_messages.map((message) => message.content)).toEqual(['yes', 'yes']);
      expect(state?.transcript_messages[1].event_id).toBe('client:client-new');
      expect(state?.transcript_messages[1].client_message_id).toBe(pending.id);
    }
    expect(states[1]?.transcript_messages[1].delivery_status).toBe('pending');
    expect(states[2]?.transcript_messages[1].delivery_status).toBe('sent');
  });

  it('keeps rejected realtime rows out of the transcript after a failed send', () => {
    const incoming = { ...transcript(3, 'Rejected reply', '2026-09-07T10:00:02Z'),
      client_message_id: 'failed-client', delivery_status: 'pending' as const };
    const previous = persisted(1, 'Earlier question', '2026-09-07T10:00:00Z');
    expect(mergePersistedChatMessages(stream([incoming]), [previous], {
      failedClientMessageIds: new Set(['failed-client']),
    })?.transcript_messages.map((message) => message.content)).toEqual(['Earlier question']);
  });

  it('preserves a compact historical work summary for lazy expansion', () => {
    const summary: AgentRunMessage = {
      ...persisted(2, '', '2026-08-15T08:23:04Z'),
      id: 'work:assistant-final',
      message_type: 'status',
      dock_work_summary: {
        message_id: 'assistant-final',
        duration_ms: 9_000,
        activity_count: 2,
      },
    };

    const merged = mergePersistedChatMessages(null, [summary]);

    expect(merged?.transcript_messages).toEqual([
      expect.objectContaining({
        message_type: 'status',
        dock_work_summary: summary.dock_work_summary,
      }),
    ]);
  });

  it('never appends older runtime history after the newest persisted page', () => {
    const merged = mergePersistedChatMessages(
      stream([
        transcript(1, 'how are clipping credits calculated?', '2026-08-13T20:50:21Z'),
        transcript(74, 'Did you create the deal?', '2026-08-14T12:07:06Z'),
      ]),
      [
        persisted(27, 'newest page starts here', '2026-08-13T20:56:25Z'),
        persisted(74, 'Did you create the deal?', '2026-08-14T12:07:06Z'),
      ],
    );

    expect(merged?.transcript_messages.map((message) => message.content)).toEqual([
      'newest page starts here',
      'Did you create the deal?',
    ]);
  });

  it('keeps only a genuinely newer unpersisted runtime tail', () => {
    const merged = mergePersistedChatMessages(
      stream([
        transcript(1, 'older snapshot history', '2026-08-13T20:50:21Z'),
        { ...transcript(3, 'new live tail', '2026-08-14T12:08:07Z'), event_id: 'msg:new-live-tail' },
      ]),
      [persisted(2, 'durable tail', '2026-08-14T12:08:04Z')],
    );

    expect(merged?.transcript_messages.map((message) => [message.sequence_no, message.content])).toEqual([
      [2, 'durable tail'],
      [3, 'new live tail'],
    ]);
  });

  it('drops unmatched historical live segments but preserves the active tail', () => {
    const historical: CodingSessionLiveTurnSegment = {
      segment_id: 'historical',
      kind: 'assistant_message',
      assistant_message: {
        message_id: 'historical', content: 'old', status: 'completed', tool_calls: [],
        started_at: '2026-08-13T20:50:21Z',
      },
    };
    const active: CodingSessionLiveTurnSegment = {
      segment_id: 'active',
      kind: 'assistant_message',
      assistant_message: {
        message_id: 'active', content: 'working', status: 'streaming', tool_calls: [],
      },
    };
    const merged = mergePersistedChatMessages(
      stream([], [historical, active]),
      [persisted(74, 'Did you create the deal?', '2026-08-14T12:07:06Z')],
    );

    expect(merged?.live_turn_segments.map((segment) => segment.segment_id)).toEqual(['active']);
  });

  it('retains the snapshot assistant segments that match durable runtime message ids', () => {
    const durableProgress = {
      ...persisted(2, 'Inspecting the conversation.', '2026-08-15T08:23:04Z'),
      runtime_message_id: 'runtime-progress',
    };
    const durableFinal = {
      ...persisted(4, 'The final answer.', '2026-08-15T08:26:14Z'),
      runtime_message_id: 'runtime-final',
      turn_segments: [{
        segment_id: 'tool-1',
        kind: 'tool_call' as const,
        tool_call: {
          tool_call_id: 'tool-1', tool_name: 'search_documents', args_text: '{}', status: 'completed' as const,
        },
      }],
    };
    const liveSegments: CodingSessionLiveTurnSegment[] = [
      {
        segment_id: 'runtime-progress', kind: 'assistant_message',
        assistant_message: {
          message_id: 'runtime-progress', content: durableProgress.content, status: 'completed', tool_calls: [],
          started_at: '2026-08-15T08:23:02Z',
        },
      },
      {
        segment_id: 'tool-1', kind: 'tool_call',
        tool_call: {
          tool_call_id: 'tool-1', tool_name: 'search_documents', args_text: '{}', status: 'completed',
          started_at: '2026-08-15T08:23:05Z',
        },
      },
      {
        segment_id: 'runtime-final', kind: 'assistant_message',
        assistant_message: {
          message_id: 'runtime-final', content: durableFinal.content, status: 'completed', tool_calls: [],
          started_at: '2026-08-15T08:26:00Z',
        },
      },
    ];

    const merged = mergePersistedChatMessages(stream([], liveSegments), [durableProgress, durableFinal]);

    expect(merged?.live_turn_segments.map((segment) => segment.segment_id)).toEqual([
      'runtime-progress',
      'tool-1',
      'runtime-final',
    ]);
  });
});

describe('hasAuthoritativeDockRuntimeTimeline', () => {
  it('retains a timestamp-less assistant completion through the durable-user handoff', () => {
    const merged = mergePersistedChatMessages(
      stream([], [{
        segment_id: 'final-live',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'final-live',
          content: 'The completed answer.',
          status: 'completed',
          tool_calls: [],
        },
      }]),
      [persisted(1, 'Question', '2026-08-15T08:25:00Z')],
    );

    expect(merged?.live_turn_segments.map((segment) => segment.segment_id)).toEqual(['final-live']);
    expect(merged && hasAuthoritativeDockRuntimeTimeline(merged)).toBe(true);
  });

  it('keeps an assistant-only completion snapshot visible while its durable answer is still catching up', () => {
    const handoff = stream(
      [{ ...transcript(1, 'Question', '2026-08-15T08:25:00Z'), role: 'user' }],
      [{
        segment_id: 'final-live',
        kind: 'assistant_message',
        assistant_message: {
          message_id: 'final-live',
          content: 'The completed answer.',
          status: 'completed',
          tool_calls: [],
          started_at: '2026-08-15T08:25:01Z',
        },
      }],
    );

    expect(hasAuthoritativeDockRuntimeTimeline(handoff)).toBe(true);
  });

  it('accepts a retained assistant/tool timeline that covers the current durable interval', () => {
    const current = stream([
      transcript(1, 'Question', '2026-08-15T08:22:54Z'),
      { ...transcript(2, 'Progress', '2026-08-15T08:23:04Z'), role: 'assistant', message_id: 'progress' },
      {
        ...transcript(4, 'Final', '2026-08-15T08:26:14Z'), role: 'assistant', message_id: 'final',
        turn_segments: [{
          segment_id: 'tool-1', kind: 'tool_call',
          tool_call: { tool_call_id: 'tool-1', tool_name: 'search_documents', args_text: '{}', status: 'completed' },
        }],
      },
    ], [
      {
        segment_id: 'progress', kind: 'assistant_message',
        assistant_message: { message_id: 'progress', content: 'Progress', status: 'completed', tool_calls: [] },
      },
      {
        segment_id: 'tool-1', kind: 'tool_call',
        tool_call: { tool_call_id: 'tool-1', tool_name: 'search_documents', args_text: '{}', status: 'completed' },
      },
      {
        segment_id: 'final', kind: 'assistant_message',
        assistant_message: { message_id: 'final', content: 'Final', status: 'completed', tool_calls: [] },
      },
    ]);

    expect(hasAuthoritativeDockRuntimeTimeline(current)).toBe(true);
  });

  it('rejects incomplete or unmatched retained timelines', () => {
    const durable = stream([
      transcript(1, 'Question', '2026-08-15T08:22:54Z'),
      { ...transcript(2, 'Progress', '2026-08-15T08:23:04Z'), role: 'assistant', message_id: 'progress' },
      {
        ...transcript(4, 'Final', '2026-08-15T08:26:14Z'), role: 'assistant', message_id: 'final',
        turn_segments: [{
          segment_id: 'tool-1', kind: 'tool_call',
          tool_call: { tool_call_id: 'tool-1', tool_name: 'search_documents', args_text: '{}', status: 'completed' },
        }],
      },
    ]);
    const incomplete = {
      ...durable,
      live_turn_segments: [
        {
          segment_id: 'tool-1', kind: 'tool_call' as const,
          tool_call: { tool_call_id: 'tool-1', tool_name: 'search_documents', args_text: '{}', status: 'completed' as const },
        },
        {
          segment_id: 'final', kind: 'assistant_message' as const,
          assistant_message: { message_id: 'final', content: 'Final', status: 'completed' as const, tool_calls: [] },
        },
      ],
    };
    const unmatched = {
      ...durable,
      live_turn_segments: [
        {
          segment_id: 'other-tool', kind: 'tool_call' as const,
          tool_call: { tool_call_id: 'other-tool', tool_name: 'search_documents', args_text: '{}', status: 'completed' as const },
        },
        {
          segment_id: 'other-final', kind: 'assistant_message' as const,
          assistant_message: { message_id: 'other-final', content: 'Other', status: 'completed' as const, tool_calls: [] },
        },
      ],
    };

    expect(hasAuthoritativeDockRuntimeTimeline(incomplete)).toBe(false);
    expect(hasAuthoritativeDockRuntimeTimeline(unmatched)).toBe(false);
  });
});

describe('mergeMessagePages', () => {
  it('replaces the entire approved turn with its summary without resurrecting decisions from older pages', () => {
    const user = persisted(1, 'Check the data.', '2026-09-18T12:00:00Z');
    const progress = { ...persisted(2, 'Searching.', '2026-09-18T12:00:01Z'), message_type: 'assistant_progress' };
    const approval = { ...persisted(3, 'Approved. Continue.', '2026-09-18T12:00:02Z'), message_type: 'approval_request_resolution' };
    const answer = { ...persisted(4, 'The findings.', '2026-09-18T12:00:03Z'), message_type: 'assistant_final' };
    const summary = { ...answer, id: `work:${answer.id}`, message_type: 'status', content: '',
      dock_work_summary: { message_id: answer.id, duration_ms: 3000, activity_count: 2 } };
    expect(mergeMessagePages([user, progress, approval], [summary, answer]).map(message => message.id)).toEqual([user.id, summary.id, answer.id]);
    expect(mergeMessagePages([summary, answer], [user, progress, approval]).map(message => message.id)).toEqual([user.id, summary.id, answer.id]);
  });

  it('replaces streamed progress and late tool metadata with one completed summary without touching other turns', () => {
    const user = persisted(43, 'Add a paragraph.', '2026-09-18T12:59:29Z');
    const progress = { ...persisted(44, 'I will add a paragraph.', '2026-09-18T12:59:33Z'), message_type: 'assistant_progress' };
    const final = { ...persisted(45, 'Added the paragraph.', '2026-09-18T12:59:38Z'), role: 'assistant' as const, message_type: 'assistant_final' };
    const tools = { ...persisted(46, '', '2026-09-18T12:59:37Z'), message_type: 'assistant_progress' };
    const summary = { ...final, id: `work:${final.id}`, content: '', message_type: 'status',
      dock_work_summary: { message_id: tools.id, duration_ms: 9000, activity_count: 2 } };
    const old = persisted(42, 'Previous answer.', '2026-09-18T12:58:00Z');
    const nextUser = persisted(47, 'Another question.', '2026-09-18T13:00:00Z');
    const nextProgress = { ...persisted(48, 'Checking.', '2026-09-18T13:00:01Z'), message_type: 'assistant_progress' };
    const current = [old, user, progress, final, tools, nextUser, nextProgress];
    const expected = [old.id, user.id, summary.id, final.id, nextUser.id, nextProgress.id];
    expect(mergeMessagePages(current, [summary, final]).map(message => message.id)).toEqual(expected);
    // Older pages arriving later must not resurrect the same progress.
    expect(mergeMessagePages([summary, final], [user, progress, tools]).map(message => message.id))
      .toEqual([user.id, summary.id, final.id]);
    // Also reconcile a cache populated before the fix.
    const snapshot = stream([], [{ kind: 'assistant_message', segment_id: progress.id,
      assistant_message: { message_id: progress.id, content: progress.content, message_type: 'assistant_progress',
        status: 'completed', tool_calls: [], started_at: progress.created_at } }]);
    const merged = mergePersistedChatMessages(snapshot, [...current, summary]);
    expect(merged?.transcript_messages.map(message => message.event_id)).toEqual(expected.map(id => `msg:${id}`));
    expect(merged?.live_turn_segments).toEqual([]);
  });

  it('keeps unrelated runs and history outside a compact summary when a page starts mid-turn', () => {
    const old = { ...persisted(2, 'Earlier progress.', '2026-09-18T12:00:00Z'), message_type: 'assistant_progress' };
    const progress = { ...persisted(4, 'Current progress.', '2026-09-18T12:01:00Z'), message_type: 'assistant_progress' };
    const unrelated = { ...progress, id: 'other-run', run_id: 'run-2' };
    const summary = { ...persisted(6, '', '2026-09-18T12:01:09Z'), id: 'work:answer', message_type: 'status',
      dock_work_summary: { message_id: 'answer', duration_ms: 9000, activity_count: 1 } };
    expect(mergeMessagePages([old, progress, unrelated], [summary]).map(message => message.id))
      .toEqual([old.id, unrelated.id, summary.id]);
  });

  it('deduplicates and restores stable sequence order across pagination', () => {
    const merged = mergeMessagePages(
      [persisted(51, 'later', '2026-08-14T00:00:51Z')],
      [
        persisted(1, 'first', '2026-08-14T00:00:01Z'),
        persisted(51, 'later refreshed', '2026-08-14T00:00:51Z'),
      ],
    );
    expect(merged.map((message) => [message.dock_chat_sequence, message.content])).toEqual([
      [1, 'first'],
      [51, 'later refreshed'],
    ]);
  });
});

describe('resolveVisiblePendingEcho', () => {
  const pending = { id: 'client-new', content: 'A new question' };

  it.each([
    { label: 'a first message', existing: [] },
    {
      label: 'an ongoing conversation',
      existing: [{ ...persisted(1, 'An earlier question', '2026-08-15T09:59:00Z'), client_message_id: 'client-old' }],
    },
  ])('hides the optimistic echo as soon as $label has an accepted durable row', ({ existing }) => {
    const accepted = {
      ...persisted(3, pending.content, '2026-08-15T10:00:00Z'),
      id: 'accepted',
      client_message_id: pending.id,
    };

    expect(resolveVisiblePendingEcho(pending, existing)).toEqual(pending);
    expect(resolveVisiblePendingEcho(pending, [...existing, accepted])).toBeNull();
  });
});
