import { cn } from '@/lib/utils';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import {
  collectSegments,
  hasRenderableSegments,
  DOCK_SEGMENT_KINDS,
  TranscriptSegmentView,
} from '@/components/agents/transcript';
import { DockUserMessage } from './DockUserMessage';

const DOCK_CHAT_SEGMENT_KINDS = new Set([...DOCK_SEGMENT_KINDS, 'user'] as const);

/** True when the stream has at least one renderable assistant/tool segment. */
export function dockTranscriptHasContent(
  stream: CodingSessionStreamState | null,
  active: boolean,
): boolean {
  return hasRenderableSegments(stream, { includeLive: active, include: DOCK_SEGMENT_KINDS });
}

/**
 * Renders an agent run's output inline inside the Ask Agents dock — assistant
 * messages as markdown plus a compact one-line row per tool call — so a
 * one-shot agent's result is readable in the bar without opening the full
 * session sheet. The main chat also shows user turns; embedded execution
 * strips remain assistant/tool-only. All shared segments stay flat and
 * non-expandable.
 */
export function DockTranscript({
  stream,
  active,
  showUserMessages = true,
  className,
}: {
  stream: CodingSessionStreamState | null;
  /** True while the run is still executing — controls live-turn inclusion. */
  active: boolean;
  /** Main chat shows user turns; embedded execution strips stay agent-only. */
  showUserMessages?: boolean;
  className?: string;
}) {
  if (!stream) return null;
  const segments = collectSegments(stream, {
    includeLive: active,
    include: showUserMessages ? DOCK_CHAT_SEGMENT_KINDS : DOCK_SEGMENT_KINDS,
  });
  if (segments.length === 0) return null;
  const latestAssistantSegmentId = [...segments].reverse().find((segment) => segment.kind === 'assistant')?.id;

  return (
    <div className={cn('space-y-1.5', className)}>
      {segments.map((segment) => (
        <TranscriptSegmentView
          key={segment.id}
          segment={segment}
          options={{
            expandable: true,
            collapseLongAssistantContent: segment.kind !== 'assistant' || segment.id !== latestAssistantSegmentId,
            fallbackUserLabel: 'You',
          }}
        />
      ))}
    </div>
  );
}
