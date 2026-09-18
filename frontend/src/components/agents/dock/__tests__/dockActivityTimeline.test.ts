import { describe, expect, it } from 'vitest';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { buildDockActivityTimeline } from '../dockActivityTimeline';

const progress: TranscriptSegment = { kind: 'assistant', id: 'progress', content: 'Checking the work.', progress: true };
const tool: TranscriptSegment = { kind: 'tool', id: 'search', toolCall: { tool_call_id: 'search', tool_name: 'search', args_text: '{}', status: 'running' } };
const final: TranscriptSegment = { kind: 'assistant', id: 'answer', content: 'Here are the results.', final: true };
const times = (segment: TranscriptSegment) => ({ progress: 1000, search: 2000, answer: 5000 })[segment.id as 'progress'] ?? null;

describe('activity timeline', () => {
  it('groups live narration and tools in their original order', () => {
    expect(buildDockActivityTimeline([progress, tool], true, times)).toMatchObject([
      { kind: 'working_group', active: true, segments: [progress, tool] },
    ]);
  });
  it('keeps a streaming final answer outside the activity summary', () => {
    const answer = { ...final, streaming: true } as TranscriptSegment;
    expect(buildDockActivityTimeline([progress, tool, answer], true, times)).toMatchObject([
      { kind: 'working_group', active: false, completed: true, durationMs: 4000, segments: [progress, tool] },
      { kind: 'segment', segment: answer },
    ]);
  });
  it('does not hide unknown streaming answer text or turn explicit progress into a final', () => {
    const answer: TranscriptSegment = { kind: 'assistant', id: 'answer', content: 'A direct answer', streaming: true };
    expect(buildDockActivityTimeline([answer], true, times)).toMatchObject([{ kind: 'segment', segment: answer }]);
    expect(buildDockActivityTimeline([progress], false, times)).toMatchObject([{ kind: 'working_group', completed: false }]);
  });
  it('keeps user questions and review decisions outside activity groups', () => {
    const user: TranscriptSegment = { kind: 'user', id: 'user', message: { event_id: 'user', role: 'user', content: 'Continue?', timestamp: '', sequence_no: 1 } };
    const review: TranscriptSegment = { ...user, kind: 'review_decision', id: 'review' };
    const result = buildDockActivityTimeline([progress, review, tool, user, final], false, times);
    expect(result.map(entry => entry.kind === 'segment' ? entry.segment.kind : entry.kind)).toEqual(['working_group', 'review_decision', 'working_group', 'user', 'assistant']);
  });
  it('splits activity at a delegated-run boundary without dropping any events', () => {
    const result = buildDockActivityTimeline([progress, tool, final], false, times, new Set(['search']));
    expect(result.map(entry => entry.kind)).toEqual(['working_group', 'working_group', 'segment']);
    expect(result.flatMap(entry => entry.kind === 'working_group' ? entry.segments : [entry.segment])).toEqual([progress, tool, final]);
  });
  it('keeps the summary identity when live progress is persisted', () => {
    const live = { ...progress, id: 'live:progress', messageId: 'message-1' } as TranscriptSegment;
    const saved = { ...progress, messageId: 'message-1' } as TranscriptSegment;
    expect(buildDockActivityTimeline([live, tool], true, times)[0].key).toBe(buildDockActivityTimeline([saved, tool, final], false, times)[0].key);
  });
});
