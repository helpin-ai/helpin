import type { TranscriptSegment } from '@/components/agents/transcript';

export interface DockWorkingGroupEntry {
  kind: 'working_group';
  key: string;
  segments: TranscriptSegment[];
  active: boolean;
}

export interface DockWorkingSegmentEntry {
  kind: 'segment';
  key: string;
  segment: TranscriptSegment;
}

export type DockWorkingTimelineEntry = DockWorkingGroupEntry | DockWorkingSegmentEntry;

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
  if (!segments.some((segment) => segment.kind === 'tool')) {
    return segments.map((segment) => ({ kind: 'segment', key: segment.id, segment }));
  }
  return [{
    kind: 'working_group',
    key: workingGroupKey(segments),
    segments,
    active,
  }];
}

/**
 * Converts the root Ask transcript into normal conversation rows plus complete,
 * sequential working groups. No source segment is discarded or summarized.
 */
export function buildDockWorkingTimeline(
  segments: TranscriptSegment[],
  active: boolean,
): DockWorkingTimelineEntry[] {
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
