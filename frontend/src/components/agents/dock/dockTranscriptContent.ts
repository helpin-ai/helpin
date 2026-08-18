import { DOCK_SEGMENT_KINDS, hasRenderableSegments } from '@/components/agents/transcript';
import type { CodingSessionStreamState } from '@/lib/pmTypes';

/** True when the stream has at least one renderable assistant/tool segment. */
export function dockTranscriptHasContent(
  stream: CodingSessionStreamState | null,
  active: boolean,
): boolean {
  return hasRenderableSegments(stream, { includeLive: active, include: DOCK_SEGMENT_KINDS });
}
