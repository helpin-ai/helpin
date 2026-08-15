import type { TranscriptSegment } from '@/components/agents/transcript';
import { canonicalToolName } from '@/lib/toolNames';

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

function isConsequentialTool(segment: TranscriptSegment): boolean {
  if (segment.kind !== 'tool') return false;
  const name = canonicalToolName(segment.toolCall.tool_name).toLowerCase();
  return /^(apply|approve|assign|cancel|checkout|commit|create|delete|edit|generate|kill|merge|move|publish|reject|remove|reply|resolve|send|update|upload|write)(_|$)/.test(name);
}

/**
 * Converts the root Ask transcript into normal conversation rows plus complete,
 * sequential working groups. No source segment is discarded or summarized.
 */
export function buildDockWorkingTimeline(
  segments: TranscriptSegment[],
  active: boolean,
): DockWorkingTimelineEntry[] {
  // Live reasoning is projected separately before the cumulative turn. Move
  // it to the newest execution phase, but never behind a streaming assistant
  // reply: assistant prose is always a hard group boundary.
  const streamingReasoning = active
    ? segments.filter((segment) => segment.kind === 'reasoning' && segment.reasoning.status === 'streaming')
    : [];
  const withoutStreamingReasoning = streamingReasoning.length > 0
    ? segments.filter((segment) => !(segment.kind === 'reasoning' && segment.reasoning.status === 'streaming'))
    : segments;
  const lastSegment = withoutStreamingReasoning.at(-1);
  const orderedSegments = streamingReasoning.length === 0
    ? withoutStreamingReasoning
    : lastSegment?.kind === 'assistant' && lastSegment.streaming
      ? [...withoutStreamingReasoning.slice(0, -1), ...streamingReasoning, lastSegment]
      : [...withoutStreamingReasoning, ...streamingReasoning];

  const entries: DockWorkingTimelineEntry[] = [];
  let activity: TranscriptSegment[] = [];
  const flush = (isTrailing: boolean) => {
    entries.push(...activityEntries(activity, active && isTrailing));
    activity = [];
  };

  for (let index = 0; index < orderedSegments.length; index += 1) {
    const segment = orderedSegments[index];
    if (isConsequentialTool(segment)) {
      flush(false);
      activity.push(segment);
      flush(active && index === orderedSegments.length - 1);
      continue;
    }
    if (segment.kind === 'tool' || segment.kind === 'reasoning') {
      activity.push(segment);
      continue;
    }
    flush(false);
    entries.push({ kind: 'segment', key: segment.id, segment });
  }
  flush(true);
  return entries;
}
