import { memo, type CSSProperties } from 'react';
import './streaming-text.css';

/** A local preview stream: full text reserves its layout while glyphs fade in.
 * Playback stays with the scene, so pause/offscreen/reduced motion reveal the
 * complete answer without per-character React renders or live announcements.
 */
export const StreamingText = memo(function StreamingText({ text, active, pending = false, duration = 2200, delay = 0 }: {
  text: string;
  active: boolean;
  pending?: boolean;
  duration?: number;
  delay?: number;
}) {
  const characters = Array.from(text);
  const step = Math.max(0, duration - 160) / Math.max(1, characters.length - 1);
  return <span className="preview-stream" data-streaming={active} data-pending={pending}>
    <span className="preview-stream-accessible">{text}</span>
    <span aria-hidden="true">{characters.map((character, index) => <span key={index} className="preview-stream-character" style={{ '--stream-delay': `${delay + index * step}ms` } as CSSProperties}>{character}</span>)}</span>
  </span>;
});
