import { describe, expect, it } from 'vitest';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { buildDockWorkingTimeline } from '../dockWorkingGroups';

function assistant(id: string, content: string, options: { messageId?: string; streaming?: boolean } = {}): TranscriptSegment {
  return { kind: 'assistant', id, messageId: options.messageId, content, streaming: options.streaming };
}

function tool(id: string, status: 'running' | 'completed' | 'failed' = 'completed'): TranscriptSegment {
  return {
    kind: 'tool',
    id,
    toolCall: {
      tool_call_id: id.replace(/^live:/, ''),
      tool_name: 'repository_search',
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
  it('preserves sequential progress and tools as completed groups while leaving the final answer flat', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate it.'),
      assistant('progress-1', 'I will inspect the conversation.', { messageId: 'message-1' }),
      tool('tool-1'),
      assistant('progress-2', 'Now I will inspect the repository.', { messageId: 'message-2' }),
      tool('tool-2'),
      assistant('final', 'The pagination state is not advancing.', { messageId: 'message-3' }),
    ], false);

    expect(timeline.map((entry) => entry.kind)).toEqual(['segment', 'working_group', 'working_group', 'segment']);
    expect(timeline[1]).toMatchObject({ kind: 'working_group', key: 'work:message-1', active: false });
    expect(timeline[2]).toMatchObject({ kind: 'working_group', key: 'work:message-2', active: false });
    expect(timeline[1]?.kind === 'working_group' && timeline[1].segments.map((segment) => segment.id)).toEqual(['progress-1', 'tool-1']);
    expect(timeline[2]?.kind === 'working_group' && timeline[2].segments.map((segment) => segment.id)).toEqual(['progress-2', 'tool-2']);
    expect(timeline[3]?.kind === 'segment' && timeline[3].segment).toMatchObject({ kind: 'assistant', id: 'final' });
  });

  it('keeps the newest in-flight group active and includes an assistant that is still streaming', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Investigate it.'),
      assistant('live:progress', 'Checking the repository now.', { messageId: 'message-live', streaming: true }),
      tool('live:tool-live', 'running'),
    ], true);

    expect(timeline).toHaveLength(2);
    expect(timeline[1]).toMatchObject({ kind: 'working_group', key: 'work:message-live', active: true });
  });

  it('uses the assistant message ID as a stable key across live and persisted snapshots', () => {
    const live = buildDockWorkingTimeline([
      assistant('live:segment-1', 'Checking.', { messageId: 'assistant-message-1', streaming: true }),
      tool('live:tool-1', 'running'),
    ], true);
    const persisted = buildDockWorkingTimeline([
      assistant('segment-1', 'Checking.', { messageId: 'assistant-message-1' }),
      tool('tool-1'),
      assistant('final', 'Done.', { messageId: 'assistant-message-2' }),
    ], false);

    expect(live[0]).toMatchObject({ kind: 'working_group', key: 'work:assistant-message-1' });
    expect(persisted[0]).toMatchObject({ kind: 'working_group', key: 'work:assistant-message-1' });
  });

  it('keeps a completed assistant-only response outside a working group', () => {
    const timeline = buildDockWorkingTimeline([
      user('user-1', 'Answer directly.'),
      assistant('answer', 'Here is the answer.', { messageId: 'answer-message' }),
    ], false);

    expect(timeline).toHaveLength(2);
    expect(timeline[1]).toMatchObject({ kind: 'segment', segment: { kind: 'assistant', id: 'answer' } });
  });

  it('attaches separately projected streaming reasoning to the newest active group', () => {
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

    const groups = timeline.filter((entry) => entry.kind === 'working_group');
    expect(groups[0]?.segments.map((segment) => segment.id)).not.toContain('live-reasoning:reasoning-1');
    expect(groups[1]?.segments.map((segment) => segment.id)).toContain('live-reasoning:reasoning-1');
  });
});
