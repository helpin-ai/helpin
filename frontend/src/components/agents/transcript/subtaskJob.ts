import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';
import { canonicalToolName } from '@/lib/toolNames';
import type { SubtaskJob } from '@/lib/services/agentService';
import type { TranscriptSegment } from './segments';

const TERMINAL_STATES = new Set(['completed', 'failed', 'cancelled']);

/** The job id a spawn_subtasks call returned, read from its result text. */
export function subtaskJobIdFromToolCall(toolCall: CodingSessionLiveToolCall): string | null {
  const text = `${toolCall.result?.output_summary ?? ''} ${toolCall.result?.content ?? ''}`;
  return text.match(/workjob_[A-Za-z0-9]+/)?.[0] ?? null;
}

/** Job ids started by the spawn_subtasks calls in a run of segments, in order, without repeats. */
export function subtaskJobIdsFromSegments(segments: TranscriptSegment[]): string[] {
  const ids: string[] = [];
  for (const segment of segments) {
    if (segment.kind !== 'tool' || canonicalToolName(segment.toolCall.tool_name) !== 'spawn_subtasks') continue;
    const id = subtaskJobIdFromToolCall(segment.toolCall);
    if (id && !ids.includes(id)) ids.push(id);
  }
  return ids;
}

export function isSubtaskJobFinished(job: Pick<SubtaskJob, 'state'>): boolean {
  return TERMINAL_STATES.has(job.state);
}

export interface SubtaskJobProgress {
  /** Items that reached a final outcome, whether or not they succeeded. */
  settled: number;
  total: number;
  percent: number;
  /** Counts worth showing beside the bar, in display order; zero counts are left out. */
  detail: string[];
}

export function subtaskJobProgress(job: Pick<SubtaskJob, 'counts'>): SubtaskJobProgress {
  const { total, completed, failed, cancelled, running, pending, retry_wait: retryWait, blocked } = job.counts;
  const settled = completed + failed + cancelled;
  const detail = [
    [running, 'running'],
    [pending + retryWait, 'waiting'],
    [blocked, 'blocked'],
    [failed, 'failed'],
    [cancelled, 'cancelled'],
  ]
    .filter(([count]) => (count as number) > 0)
    .map(([count, label]) => `${count} ${label}`);
  return { settled, total, percent: total > 0 ? Math.round((settled / total) * 100) : 0, detail };
}
