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
  const assistant = segments.find((segment) => segment.kind === 'assistant');
  if (assistant?.kind === 'assistant') return `work:${assistant.messageId || assistant.id.replace(/^live:/, '')}`;
  const tool = segments.find((segment) => segment.kind === 'tool');
  if (tool?.kind === 'tool') return `work:${tool.toolCall.tool_call_id || tool.id.replace(/^live:/, '')}`;
  const reasoning = segments.find((segment) => segment.kind === 'reasoning');
  if (reasoning?.kind === 'reasoning') return `work:${reasoning.reasoning.message_id || reasoning.id.replace(/^live:/, '')}`;
  return `work:${segments[0]?.id.replace(/^live:/, '') || 'activity'}`;
}

function groupedActivity(segments: TranscriptSegment[], active: boolean): DockWorkingTimelineEntry[] {
  if (segments.length === 0) return [];
  // The stream projects the one current reasoning message separately and the
  // collector places it before the cumulative turn timeline. It belongs to
  // the newest live activity, so reattach it at the tail before grouping.
  const streamingReasoning = active
    ? segments.filter((segment) => segment.kind === 'reasoning' && segment.reasoning.status === 'streaming')
    : [];
  const orderedSegments = streamingReasoning.length > 0
    ? [
        ...segments.filter((segment) => !(segment.kind === 'reasoning' && segment.reasoning.status === 'streaming')),
        ...streamingReasoning,
      ]
    : segments;
  let finalAssistantIndex = -1;
  if (!active) {
    const lastAssistantIndex = orderedSegments.findLastIndex((segment) => segment.kind === 'assistant');
    const hasWorkAfter = orderedSegments.slice(lastAssistantIndex + 1).some((segment) => (
      segment.kind === 'tool' || segment.kind === 'reasoning'
    ));
    if (lastAssistantIndex >= 0 && !hasWorkAfter) finalAssistantIndex = lastAssistantIndex;
  }

  const groups: TranscriptSegment[][] = [];
  let current: TranscriptSegment[] = [];
  const flush = () => {
    if (current.length > 0) groups.push(current);
    current = [];
  };

  for (let index = 0; index < orderedSegments.length; index += 1) {
    if (index === finalAssistantIndex) continue;
    const segment = orderedSegments[index];
    if (segment.kind === 'assistant') {
      const reasoningOnly = current.length > 0 && current.every((entry) => entry.kind === 'reasoning');
      if (current.length > 0 && !reasoningOnly) flush();
    }
    current.push(segment);
  }
  flush();

  const entries: DockWorkingTimelineEntry[] = groups.map((group, index) => ({
    kind: 'working_group',
    key: workingGroupKey(group),
    segments: group,
    active: active && index === groups.length - 1,
  }));
  if (finalAssistantIndex >= 0) {
    const segment = orderedSegments[finalAssistantIndex];
    entries.push({ kind: 'segment', key: segment.id, segment });
  }
  return entries;
}

function isConversationBoundary(segment: TranscriptSegment): boolean {
  return segment.kind === 'user'
    || segment.kind === 'review_decision'
    || segment.kind === 'status'
    || segment.kind === 'context';
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
    entries.push(...groupedActivity(activity, active && isTrailing));
    activity = [];
  };

  for (const segment of segments) {
    if (!isConversationBoundary(segment)) {
      activity.push(segment);
      continue;
    }
    flush(false);
    entries.push({ kind: 'segment', key: segment.id, segment });
  }
  flush(true);
  return entries;
}
