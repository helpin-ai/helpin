import type { TranscriptSegment } from '@/components/agents/transcript';
import { finalAssistantIndex } from './agentTurnState';
import type { DockWorkingTimelineEntry } from './dockWorkingGroups';

/** Group public progress into a rail without moving answers or interaction boundaries. */
export function buildDockActivityTimeline(
  segments: TranscriptSegment[],
  active: boolean,
  timestampForSegment: (segment: TranscriptSegment) => number | null,
  breakBefore: ReadonlySet<string> = new Set(),
): DockWorkingTimelineEntry[] {
  const entries: DockWorkingTimelineEntry[] = [];
  let interval: TranscriptSegment[] = [];
  const flush = (trailing: boolean) => {
    const finalIndex = finalAssistantIndex(interval, !trailing || !active);
    let work: TranscriptSegment[] = [];
    const flushWork = (running: boolean, completedAt?: number | null, completed = false) => {
      if (!work.length) return;
      const first = work[0];
      const id = first.kind === 'tool' ? first.toolCall.tool_call_id
        : first.kind === 'assistant' ? first.messageId ?? first.id
        : first.kind === 'reasoning' ? first.reasoning.message_id : first.id;
      const startedAt = timestampForSegment(first);
      entries.push({
        kind: 'working_group', key: `activity:${id.replace(/^live:/, '')}`,
        segments: work, active: running, completed,
        ...(startedAt !== null ? { startedAt } : {}),
        ...(startedAt !== null && completedAt != null
          ? { durationMs: Math.max(0, completedAt - startedAt) } : {}),
      });
      work = [];
    };
    interval.forEach((segment, index) => {
      // Unknown live assistant text may be the answer. Keep it visible until
      // a following activity or an explicit progress marker establishes its role.
      const answer = segment.kind === 'assistant' && (segment.final || index === finalIndex
        || (trailing && active && index === interval.length - 1 && !segment.progress));
      if (answer) {
        const completed = (segment.final && !segment.streaming) || index === finalIndex;
        // Provisional prose and a still-streaming final are not completion events.
        // Keep earlier tool rows open while the answer is being delivered.
        flushWork(active && trailing && !completed, completed ? timestampForSegment(segment) : undefined, !!completed);
        entries.push({ kind: 'segment', key: segment.id, segment });
      } else {
        if (breakBefore.has(segment.id)) flushWork(false);
        work.push(segment);
      }
    });
    flushWork(active && trailing);
    interval = [];
  };
  for (const segment of segments) {
    if (segment.kind === 'assistant' || segment.kind === 'tool' || segment.kind === 'reasoning') {
      interval.push(segment);
    } else {
      flush(false);
      entries.push({ kind: 'segment', key: segment.id, segment });
    }
  }
  flush(true);
  return entries;
}
