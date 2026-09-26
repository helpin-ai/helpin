import { useEffect, useState } from 'react';
import { TranscriptSegmentView, type TranscriptSegment } from '@/components/agents/transcript';
import type { RenderSegmentOptions } from '@/components/agents/transcript/segmentRenderers';

export function DockAnswerSegment({ segment, options, animate }: {
  segment: TranscriptSegment; options: RenderSegmentOptions; animate: boolean;
}) {
  const content = segment.kind === 'assistant' ? segment.content : '';
  const [motionAllowed, setMotionAllowed] = useState(() =>
    typeof window.matchMedia === 'function' && !window.matchMedia('(prefers-reduced-motion: reduce)').matches);
  const [visible, setVisible] = useState(() => animate && motionAllowed ? '' : content);
  useEffect(() => {
    const media = window.matchMedia?.('(prefers-reduced-motion: reduce)');
    if (!media) return;
    const update = () => setMotionAllowed(!media.matches);
    media.addEventListener?.('change', update);
    return () => media.removeEventListener?.('change', update);
  }, []);
  if ((!animate || !motionAllowed || !content.startsWith(visible)) && visible !== content) setVisible(content);
  // Revisions are authoritative; only append-only text is progressively revealed.
  const revealing = animate && motionAllowed && content.startsWith(visible) && visible.length < content.length;
  useEffect(() => {
    if (!revealing) return;
    const timer = window.setTimeout(() => {
      let end = Math.min(content.length, visible.length + Math.max(5, Math.ceil(content.length / 60)));
      // Never cut a UTF-16 surrogate pair in half.
      if (end < content.length && /[\uD800-\uDBFF]/.test(content[end - 1])) end++;
      setVisible(content.slice(0, end));
    }, 32);
    return () => window.clearTimeout(timer);
  }, [content, revealing, visible]);
  if (segment.kind !== 'assistant' || !animate) return <TranscriptSegmentView segment={segment} options={options} />;
  return <div data-dock-answer-revealing={revealing || undefined} aria-busy={revealing}>
    <TranscriptSegmentView segment={{ ...segment, content: revealing ? visible : content, streaming: revealing || segment.streaming }} options={options} />
  </div>;
}
