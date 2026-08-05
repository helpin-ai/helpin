import { cn } from '@/lib/utils';
import type { CodingSessionStreamState } from '@/lib/pmTypes';
import {
  collectSegments,
  hasRenderableSegments,
  DOCK_SEGMENT_KINDS,
  TranscriptSegmentView,
} from '@/components/agents/transcript';

/** True when the stream has at least one renderable chat/tool segment. */
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
 * session sheet. User messages stay interleaved chronologically with assistant
 * and tool segments; implementation-only context/status rows remain hidden.
 */
export function DockTranscript({
  stream,
  active,
  className,
}: {
  stream: CodingSessionStreamState | null;
  /** True while the run is still executing — controls live-turn inclusion. */
  active: boolean;
  className?: string;
}) {
  if (!stream) return null;
  const segments = collectSegments(stream, { includeLive: active, include: DOCK_SEGMENT_KINDS });
  if (segments.length === 0) return null;

  return (
    <div className={cn('space-y-1.5', className)}>
      {segments.map((segment) => (
        <TranscriptSegmentView
          key={segment.id}
          segment={segment}
          options={{ expandable: true, fallbackUserLabel: 'You' }}
        />
      ))}
    </div>
  );
}
