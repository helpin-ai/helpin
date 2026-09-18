import type { TranscriptSegment } from '@/components/agents/transcript';
import type { CodingSessionTurnState } from '@/lib/pmTypes';
import { finalAssistantIndex } from './agentTurnState';

export interface DockWorkingGroupEntry {
  kind: 'working_group';
  key: string;
  segments: TranscriptSegment[];
  active: boolean;
  durationMs?: number;
  completed?: boolean;
  startedAt?: number;
}

export interface DockWorkingSegmentEntry {
  kind: 'segment';
  key: string;
  segment: TranscriptSegment;
}

export type DockWorkingTimelineEntry = DockWorkingGroupEntry | DockWorkingSegmentEntry;

interface DockWorkingTimelineOptions {
  turnState?: CodingSessionTurnState;
  collapseCompletedWork?: boolean;
  timestampForSegment?: (segment: TranscriptSegment) => number | null;
}

function workingGroupKey(segments: TranscriptSegment[]): string {
  const tool = segments.find((segment) => segment.kind === 'tool');
  if (tool?.kind === 'tool') return `work:${tool.toolCall.tool_call_id || tool.id.replace(/^live:/, '')}`;
  const reasoning = segments.find((segment) => segment.kind === 'reasoning');
  if (reasoning?.kind === 'reasoning') return `work:${reasoning.reasoning.message_id || reasoning.id.replace(/^live:/, '')}`;
  return `work:${segments[0]?.id.replace(/^live:/, '') || 'activity'}`;
}

function activityEntries(
  segments: TranscriptSegment[],
  active: boolean,
): DockWorkingTimelineEntry[] {
  if (segments.length === 0) return [];
  const toolCount = segments.filter((segment) => segment.kind === 'tool').length;
  if (toolCount < 2) {
    return segments.map((segment) => ({ kind: 'segment', key: segment.id, segment }));
  }
  return [{
    kind: 'working_group',
    key: workingGroupKey(segments),
    segments,
    active,
  }];
}

function completedRunTimeline(
  segments: TranscriptSegment[],
  active: boolean,
  timestampForSegment?: (segment: TranscriptSegment) => number | null,
  turnState?: CodingSessionTurnState,
): DockWorkingTimelineEntry[] {
  const entries: DockWorkingTimelineEntry[] = [];
  let interval: TranscriptSegment[] = [];
  let boundaryStartedAt: number | null = null;

  const flush = (trailing: boolean) => {
    if (interval.length === 0) return;
    const finalIndex = finalAssistantIndex(interval, !trailing || !active);
    const work = finalIndex > 0 ? interval.slice(0, finalIndex) : [];
    const canCollapse = work.length > 0 && work.every((segment) => (
      segment.kind === 'assistant' || segment.kind === 'tool' || segment.kind === 'reasoning'
    ));

    if (canCollapse) {
      const finalResponse = interval[finalIndex];
      const currentAnswer = turnState?.answer_message_id && finalResponse.id.endsWith(turnState.answer_message_id);
      const startedAt = currentAnswer ? Date.parse(turnState.started_at) : boundaryStartedAt ?? timestampForSegment?.(work[0]) ?? null;
      const completedAt = currentAnswer && turnState.completed_at ? Date.parse(turnState.completed_at) : timestampForSegment?.(finalResponse) ?? null;
      const durationMs = startedAt !== null && completedAt !== null
        ? Math.max(0, completedAt - startedAt)
        : 0;
      entries.push({
        kind: 'working_group',
        key: workingGroupKey(work),
        segments: work,
        active: false,
        durationMs,
      });
      for (const segment of interval.slice(finalIndex)) {
        entries.push({ kind: 'segment', key: segment.id, segment });
      }
    } else {
      entries.push(...buildDockWorkingTimeline(interval, active && trailing, { collapseCompletedWork: false }));
    }
    interval = [];
  };

  for (const segment of segments) {
    if (segment.kind === 'user' || segment.kind === 'review_decision') {
      flush(false);
      entries.push({ kind: 'segment', key: segment.id, segment });
      boundaryStartedAt = timestampForSegment?.(segment) ?? null;
      continue;
    }
    interval.push(segment);
  }
  flush(true);
  return entries;
}

/**
 * Converts the root Ask transcript into normal conversation rows plus complete,
 * sequential working groups. No source segment is discarded or summarized.
 */
export function buildDockWorkingTimeline(
  segments: TranscriptSegment[],
  active: boolean,
  options: DockWorkingTimelineOptions = {},
): DockWorkingTimelineEntry[] {
  if ((options.collapseCompletedWork && !active) || (options.collapseCompletedWork !== false && segments.some((s) => s.kind === 'assistant' && s.final))) {
    return completedRunTimeline(segments, active, options.timestampForSegment, options.turnState);
  }
  const entries: DockWorkingTimelineEntry[] = [];
  let activity: TranscriptSegment[] = [];
  const flush = (isTrailing: boolean) => {
    entries.push(...activityEntries(activity, active && isTrailing));
    activity = [];
  };

  for (let index = 0; index < segments.length; index += 1) {
    const segment = segments[index];
    if (segment.kind === 'tool') {
      activity.push(segment);
      continue;
    }
    flush(false);
    entries.push({ kind: 'segment', key: segment.id, segment });
  }
  flush(true);
  return entries;
}
