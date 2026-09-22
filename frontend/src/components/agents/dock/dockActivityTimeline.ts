import type { TranscriptSegment } from '@/components/agents/transcript';
import { finalAssistantIndex } from './agentTurnState';
import type { DockWorkingTimelineEntry } from './dockWorkingGroups';
import { findLastMatchingIndex } from './findLastMatchingIndex';

/** Decisions resume the same activity; only conversation boundaries end a turn. */
export function buildDockActivityTimeline(
  segments: TranscriptSegment[],
  active: boolean,
  timestampForSegment: (segment: TranscriptSegment) => number | null,
): DockWorkingTimelineEntry[] {
  const entries: DockWorkingTimelineEntry[] = [];
  let interval: TranscriptSegment[] = [];
  const flush = (trailing: boolean) => {
    // A legacy narration followed by an approval/tool is not a final answer.
    const finalIndex = finalAssistantIndex(interval, (!trailing || !active) && interval.at(-1)?.kind === 'assistant');
    let answerIndex = finalIndex;
    if (answerIndex < 0) answerIndex = findLastMatchingIndex(interval, segment => segment.kind === 'assistant' && !!segment.final);
    const last = interval.at(-1);
    // Keep unclassified streaming prose visible until subsequent activity
    // establishes it as narration; explicit progress always stays in the rail.
    if (answerIndex < 0 && trailing && active && last?.kind === 'assistant' && !last.progress) answerIndex = interval.length - 1;
    const answer = interval[answerIndex];
    const completed = answer?.kind === 'assistant' && !answer.streaming && (answer.final || answerIndex === finalIndex);
    // Late tool metadata belongs to the same activity, before the final answer.
    const work = interval.filter((_, index) => index !== answerIndex).map(segment =>
      segment.kind === 'assistant' && segment.final ? { ...segment, final: false, progress: true } : segment);
    if (work.length) {
      const first = work[0];
      const id = first.kind === 'tool' ? first.toolCall.tool_call_id
        : first.kind === 'assistant' ? first.messageId ?? first.id
        : first.kind === 'reasoning' ? first.reasoning.message_id : first.id;
      const startedAt = timestampForSegment(first);
      const completedAt = completed ? timestampForSegment(answer) : null;
      entries.push({
        kind: 'working_group', key: `activity:${id.replace(/^live:/, '')}`,
        segments: work, active: active && trailing && !completed, completed: !!completed,
        ...(startedAt !== null ? { startedAt } : {}),
        ...(startedAt !== null && completedAt != null
          ? { durationMs: Math.max(0, completedAt - startedAt) } : {}),
      });
    }
    if (answer) entries.push({ kind: 'segment', key: answer.id, segment: answer });
    interval = [];
  };
  for (const segment of segments) {
    if (segment.kind === 'assistant' || segment.kind === 'tool' || segment.kind === 'reasoning' || segment.kind === 'review_decision') {
      interval.push(segment);
    } else {
      flush(false);
      entries.push({ kind: 'segment', key: segment.id, segment });
    }
  }
  flush(true);
  return entries;
}
