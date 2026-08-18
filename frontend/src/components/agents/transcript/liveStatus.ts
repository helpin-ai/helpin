import { describeToolCall } from '@/components/pm/CodingSession/toolCallPresentation';
import { isToolName } from '@/lib/toolNames';
import type { CodingSessionStreamState } from '@/lib/pmTypes';

/**
 * One-line live status for an active run: the currently running tool's label
 * ("Read src/foo.ts", "Search \"refund\""), falling back to reasoning/lifecycle
 * phrasing. Null when there is nothing in flight to report.
 */
export function deriveLiveStatusLabel(
  stream: Pick<CodingSessionStreamState, 'live_turn_segments' | 'live_reasoning_message'> | null,
  runStatus?: string,
): string | null {
  if (stream) {
    for (let index = stream.live_turn_segments.length - 1; index >= 0; index -= 1) {
      const segment = stream.live_turn_segments[index];
      if (segment.kind !== 'tool_call') continue;
      if (isToolName(segment.tool_call.tool_name, 'update_plan')) continue;
      if (segment.tool_call.status !== 'running') break;
      return describeToolCall(segment.tool_call).primaryLabel;
    }
    if (stream.live_reasoning_message?.status === 'streaming') return 'Thinking…';
  }
  if (runStatus === 'queued') return 'Starting agent…';
  if (runStatus === 'running') return 'Thinking…';
  return null;
}
