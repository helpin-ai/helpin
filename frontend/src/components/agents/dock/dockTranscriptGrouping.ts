import type { TranscriptSegment } from '@/components/agents/transcript';
import type { CodingSessionLiveToolCall } from '@/lib/pmTypes';
import { canonicalToolName } from '@/lib/toolNames';
import { repositorySelectorForToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import type { TranscriptToolGroupPresentation } from '@/components/agents/transcript/segmentRenderers';

export interface DockTranscriptEntry {
  key: string;
  segment: TranscriptSegment;
  toolGroup?: TranscriptToolGroupPresentation;
}

function toolGroupKey(segment: Extract<TranscriptSegment, { kind: 'tool' }>): string {
  const toolName = canonicalToolName(segment.toolCall.tool_name).toLowerCase();
  const repository = repositorySelectorForToolCall(segment.toolCall)?.toLowerCase() ?? '';
  return `${toolName}\u0000${repository}`;
}

function combinedStatus(
  current: CodingSessionLiveToolCall['status'],
  next: CodingSessionLiveToolCall['status'],
): CodingSessionLiveToolCall['status'] {
  if (current === 'failed' || next === 'failed') return 'failed';
  if (current === 'running' || next === 'running') return 'running';
  return next;
}

function isFailedEntry(entry: DockTranscriptEntry): boolean {
  return entry.segment.kind === 'tool'
    && (entry.toolGroup?.status ?? entry.segment.toolCall.status) === 'failed';
}

/** Collapses only uninterrupted runs of the same canonical tool. */
export function groupAdjacentDockTools(segments: TranscriptSegment[]): DockTranscriptEntry[] {
  const entries: DockTranscriptEntry[] = [];

  for (const segment of segments) {
    const previous = entries[entries.length - 1];
    if (
      segment.kind === 'tool'
      && previous?.segment.kind === 'tool'
      && toolGroupKey(previous.segment) === toolGroupKey(segment)
      && isFailedEntry(previous) === (segment.toolCall.status === 'failed')
    ) {
      const previousDuration = previous.toolGroup?.totalDurationMs
        ?? previous.segment.toolCall.duration_ms;
      const nextDuration = segment.toolCall.duration_ms;
      const hasDuration = (previousDuration ?? 0) > 0 || (nextDuration ?? 0) > 0;
      previous.toolGroup = {
        count: (previous.toolGroup?.count ?? 1) + 1,
        totalDurationMs: hasDuration ? (previousDuration ?? 0) + (nextDuration ?? 0) : undefined,
        status: combinedStatus(previous.toolGroup?.status ?? previous.segment.toolCall.status, segment.toolCall.status),
      };
      // Keep the latest call as the representative so a live group retains its
      // newest result/status while the stable display label comes from the tool name.
      previous.segment = segment;
      continue;
    }

    entries.push({ key: segment.id, segment });
  }

  return entries;
}
