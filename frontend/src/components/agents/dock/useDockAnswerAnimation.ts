import { useState } from 'react';
import type { TranscriptSegment } from '@/components/agents/transcript';
import { findLastMatchingIndex } from './findLastMatchingIndex';

const answerId = (segment: TranscriptSegment) => segment.kind === 'assistant'
  ? segment.messageId ?? segment.id.replace(/^live:/, '') : segment.id;

/** Animate only answers arriving after we observed this conversation working. */
export function useDockAnswerAnimation(segments: TranscriptSegment[], active: boolean) {
  const ids = segments.filter(segment => segment.kind === 'assistant' && !segment.progress).map(answerId);
  const currentTurnStart = findLastMatchingIndex(segments, segment => segment.kind === 'user' || segment.kind === 'review_decision');
  const currentTurnIds = new Set(segments.slice(currentTurnStart + 1).map(answerId));
  const signature = JSON.stringify(ids);
  const [previous, setPrevious] = useState(() => ({
    signature, active, armed: active, initialized: segments.length > 0,
    known: new Set(ids), animated: new Set<string>(),
  }));
  let current = previous;
  if (signature !== previous.signature || active !== previous.active || (!previous.initialized && segments.length > 0)) {
    const animated = new Set(previous.animated);
    if (previous.initialized && (previous.armed || active)) {
      ids.forEach(id => { if (!previous.known.has(id) && currentTurnIds.has(id)) animated.add(id); });
    }
    current = { signature, active, armed: previous.armed || active,
      initialized: previous.initialized || segments.length > 0,
      known: new Set([...previous.known, ...ids]), animated };
    setPrevious(current);
  }
  return (segment: TranscriptSegment) => current.animated.has(answerId(segment));
}
