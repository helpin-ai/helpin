import { describe, expect, it } from 'vitest';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { buildDockWorkingTimeline } from '../dockWorkingGroups';

function assistant(id: string, content: string, options: { messageId?: string; streaming?: boolean } = {}): TranscriptSegment {
  return { kind: 'assistant', id, messageId: options.messageId, content, streaming: options.streaming };
}

function tool(
  id: string,
  status: 'running' | 'completed' | 'failed' = 'completed',
  toolName = 'repository_search',
): TranscriptSegment {
  return {
    kind: 'tool',
    id,
    toolCall: {
      tool_call_id: id.replace(/^live:/, ''),
      tool_name: toolName,
      args_text: '{"query":"pagination"}',
      status,
    },
  };
}

function user(id: string, content: string): TranscriptSegment {
  return {
    kind: 'user',
    id,
    message: { event_id: id, role: 'user', content, timestamp: '2026-08-14T00:00:00Z', sequence_no: 1 },
  };
}

describe('buildDockWorkingTimeline', () => {
  it('collapses completed standalone work before the final response with its turn duration', () => {
    const timestamps = new Map([
      ['user-1', 1_000],
      ['progress-1', 2_000],
      ['tool-1', 5_000],
      ['final', 10_000],
    ]);
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate it.'),
      assistant('progress-1', 'I will inspect the repository.'),
      tool('tool-1'),
      assistant('final', 'The issue is in the event sorter.'),
    ], false, {
      collapseCompletedWork: true,
      timestampForSegment: (segment) => timestamps.get(segment.id) ?? null,
    });

    expect(timeline).toHaveLength(3);
    expect(timeline[1]).toMatchObject({
      kind: 'working_group',
      durationMs: 9_000,
      segments: [
        { kind: 'assistant', id: 'progress-1' },
        { kind: 'tool', id: 'tool-1' },
      ],
    });
    expect(timeline[2]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'final' } });
  });

  it('leaves a completed direct answer flat', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Answer directly.'),
      assistant('answer', 'Here is the answer.'),
    ], false, { collapseCompletedWork: true });

    expect(timeline).toHaveLength(2);
    expect(timeline.some((entry) => entry.kind === 'working_group')).toBe(false);
  });

  it('collapses completed work independently for each user turn', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate the first issue.'),
      assistant('progress-1', 'Checking the first issue.'),
      tool('tool-1'),
      assistant('final-1', 'The first issue is fixed.'),
      user('user-2', 'Now inspect the second issue.'),
      assistant('progress-2', 'Checking the second issue.'),
      tool('tool-2'),
      assistant('final-2', 'The second issue is fixed.'),
    ], false, { collapseCompletedWork: true });

    expect(timeline.map((entry) => entry.kind)).toEqual([
      'segment',
      'working_group',
      'segment',
      'segment',
      'working_group',
      'segment',
    ]);
    expect(timeline.filter((entry) => entry.kind === 'working_group')).toHaveLength(2);
  });

  it('keeps every assistant message flat and groups only the tool phases between them', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate it.'),
      assistant('progress-1', 'I will inspect the conversation.', { messageId: 'message-1' }),
      tool('tool-1'),
      assistant('progress-2', 'Now I will inspect the repository.', { messageId: 'message-2' }),
      tool('tool-2'),
      assistant('final', 'The pagination state is not advancing.', { messageId: 'message-3' }),
    ], false);

    expect(timeline.map((entry) => entry.kind)).toEqual([
      'segment',
      'segment',
      'segment',
      'segment',
      'segment',
      'segment',
    ]);
    expect(timeline[1]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'progress-1' } });
    expect(timeline[2]).toMatchObject({ kind: 'segment', key: 'tool-1', segment: { kind: 'tool' } });
    expect(timeline[3]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'progress-2' } });
    expect(timeline[4]).toMatchObject({ kind: 'segment', key: 'tool-2', segment: { kind: 'tool' } });
    expect(timeline[5]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'final' } });
  });

  it('keeps the newest trailing tool group active without grouping a streaming assistant', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate it.'),
      assistant('live:progress', 'Checking the repository now.', { messageId: 'message-live', streaming: true }),
      tool('live:tool-live', 'running'),
    ], true);

    expect(timeline).toHaveLength(3);
    expect(timeline[1]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'live:progress' } });
    expect(timeline[2]).toMatchObject({ kind: 'segment', key: 'live:tool-live', segment: { kind: 'tool' } });
  });

  it('uses the tool call ID as a stable key across live and persisted snapshots', () => {
    const live = buildDockWorkingTimeline([
      assistant('live:segment-1', 'Checking.', { messageId: 'assistant-message-1', streaming: true }),
      tool('live:tool-1', 'running'),
    ], true);
    const persisted = buildDockWorkingTimeline([
      assistant('segment-1', 'Checking.', { messageId: 'assistant-message-1' }),
      tool('tool-1'),
      assistant('final', 'Done.', { messageId: 'assistant-message-2' }),
    ], false);

    expect(live[1]).toMatchObject({ kind: 'segment', key: 'live:tool-1', segment: { kind: 'tool' } });
    expect(persisted[1]).toMatchObject({ kind: 'segment', key: 'tool-1', segment: { kind: 'tool' } });
  });

  it('keeps a completed assistant-only response outside a working group', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Answer directly.'),
      assistant('answer', 'Here is the answer.', { messageId: 'answer-message' }),
    ], false);

    expect(timeline).toHaveLength(2);
    expect(timeline[1]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'answer' } });
  });

  it('keeps separately projected reasoning flat and outside every tool group', () => {
    const reasoning: TranscriptSegment = {
      kind: 'reasoning',
      id: 'live-reasoning:reasoning-1',
      reasoning: { message_id: 'reasoning-1', content: 'Comparing the latest result.', status: 'streaming' },
    };
    const timeline = buildDockWorkingTimeline([
      reasoning,
      assistant('progress-1', 'Checking the conversation.', { messageId: 'message-1' }),
      tool('tool-1'),
      assistant('progress-2', 'Checking the repository.', { messageId: 'message-2' }),
      tool('tool-2', 'running'),
    ], true);

    expect(timeline[0]).toMatchObject({
      kind: 'segment',
      segment: { kind: 'reasoning', id: 'live-reasoning:reasoning-1' },
    });
    const tools = timeline.filter((entry) => entry.kind === 'segment' && entry.segment.kind === 'tool');
    expect(tools).toHaveLength(2);
  });

  it('leaves a final streaming assistant message flat and completes the preceding tool group', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate it.'),
      assistant('progress-1', 'Checking the repository.', { messageId: 'progress-message' }),
      tool('tool-1'),
      assistant('live:final', 'Here is what I found.', { messageId: 'final-message', streaming: true }),
    ], true);

    expect(timeline).toHaveLength(4);
    expect(timeline[2]).toMatchObject({ kind: 'segment', key: 'tool-1', segment: { kind: 'tool' } });
    expect(timeline[3]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'live:final' } });
  });

  it('keeps every contiguous tool sequence in one chronological phase', () => {
    const timeline = buildDockWorkingTimeline([
      tool('search-1'),
      tool('read-1', 'completed', 'read_files'),
      tool('create-1', 'completed', 'create_document'),
      tool('search-2'),
    ], false);

    const groups = timeline.filter((entry) => entry.kind === 'working_group');
    expect(groups).toHaveLength(1);
    expect(groups[0]?.segments.map((segment) => segment.id)).toEqual([
      'search-1',
      'read-1',
      'create-1',
      'search-2',
    ]);
  });
});
