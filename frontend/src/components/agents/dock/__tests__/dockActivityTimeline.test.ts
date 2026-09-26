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
      { kind: 'working_group', active: true, completed: false, segments: [progress, tool] },
      { kind: 'segment', segment: answer },
    ]);
  });
  it('keeps work running through provisional narration until a final is confirmed', () => {
    const narration: TranscriptSegment = { kind: 'assistant', id: 'narration', content: 'Checking the next item.', streaming: true };
    const first = buildDockActivityTimeline([progress, tool, narration], true, times);
    expect(first[0]).toMatchObject({ active: true, completed: false });
    expect(first[0]).not.toHaveProperty('durationMs');
    const nextTool = { ...tool, id: 'second', toolCall: { ...tool.toolCall, tool_call_id: 'second' } };
    const next = buildDockActivityTimeline([progress, tool, narration, nextTool], true, times);
    expect(next[0]).toMatchObject({ key: first[0].key, active: true });
    const finished = buildDockActivityTimeline([progress, tool, narration, nextTool, final], false, times);
    expect(finished[0]).toMatchObject({ key: first[0].key, active: false, completed: true, durationMs: 4000 });
  });
  it('does not hide unknown streaming answer text or turn explicit progress into a final', () => {
    const answer: TranscriptSegment = { kind: 'assistant', id: 'answer', content: 'A direct answer', streaming: true };
    expect(buildDockActivityTimeline([answer], true, times)).toMatchObject([{ kind: 'segment', segment: answer }]);
    expect(buildDockActivityTimeline([progress], false, times)).toMatchObject([{ kind: 'working_group', completed: false }]);
  });
  it('keeps review decisions in the same activity and starts fresh for an actual user message', () => {
    const user: TranscriptSegment = { kind: 'user', id: 'user', message: { event_id: 'user', role: 'user', content: 'Continue?', timestamp: '', sequence_no: 1 } };
    const review: TranscriptSegment = { ...user, kind: 'review_decision', id: 'review' };
    const result = buildDockActivityTimeline([progress, review, tool, user, final], false, times);
    expect(result.map(entry => entry.kind === 'segment' ? entry.segment.kind : entry.kind)).toEqual(['working_group', 'user', 'assistant']);
    expect(result[0]).toMatchObject({ segments: [progress, review, tool] });
  });
  it('keeps the full turn in one activity without dropping any events', () => {
    const result = buildDockActivityTimeline([progress, tool, final], false, times);
    expect(result.map(entry => entry.kind)).toEqual(['working_group', 'segment']);
    expect(result.flatMap(entry => entry.kind === 'working_group' ? entry.segments : [entry.segment])).toEqual([progress, tool, final]);
  });
  it('keeps late tool metadata in the activity before the final answer', () => {
    expect(buildDockActivityTimeline([progress, final, tool], false, times)).toMatchObject([
      { kind: 'working_group', completed: true, segments: [progress, tool] },
      { kind: 'segment', segment: final },
    ]);
  });
  it('does not promote legacy narration to a final when followed by an approval', () => {
    const decision: TranscriptSegment = { kind: 'review_decision', id: 'decision', message: { role: 'user', event_id: 'decision', content: 'Approved.', timestamp: '', sequence_no: 2 } };
    const narration = { ...progress, progress: undefined };
    expect(buildDockActivityTimeline([narration, decision], false, times)).toMatchObject([
      { kind: 'working_group', completed: false, segments: [narration, decision] },
    ]);
  });
  it('keeps the summary identity when live progress is persisted', () => {
    const live = { ...progress, id: 'live:progress', messageId: 'message-1' } as TranscriptSegment;
    const saved = { ...progress, messageId: 'message-1' } as TranscriptSegment;
    expect(buildDockActivityTimeline([live, tool], true, times)[0].key).toBe(buildDockActivityTimeline([saved, tool, final], false, times)[0].key);
  });
});
