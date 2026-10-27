import { TranscriptSegmentView, type TranscriptSegment } from '@/components/agents/transcript';
import type { RenderSegmentOptions } from '@/components/agents/transcript/segmentRenderers';
import { useRevealText } from '@/hooks/useRevealText';

export function DockAnswerSegment({ segment, options, animate }: {
  segment: TranscriptSegment; options: RenderSegmentOptions; animate: boolean;
}) {
  const content = segment.kind === 'assistant' ? segment.content : '';
  // Revisions are authoritative; only append-only text is progressively revealed.
  const { visible, revealing } = useRevealText(content, animate);
  if (segment.kind !== 'assistant' || !animate) return <TranscriptSegmentView segment={segment} options={options} />;
  return <div data-dock-answer-revealing={revealing || undefined} aria-busy={revealing}>
    <TranscriptSegmentView segment={{ ...segment, content: visible, streaming: revealing || segment.streaming }} options={options} />
  </div>;
}
