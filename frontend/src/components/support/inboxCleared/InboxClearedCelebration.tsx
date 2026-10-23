import { useEffect, useRef, type CSSProperties } from 'react';

import type { InboxCelebration } from '@/stores/supportInboxCelebrationStore';
import { launchInboxClearedConfetti } from './inboxClearedConfetti';
import { RARITY_LABEL } from './inboxClearedMessages';

const TICKS = Array.from({ length: 16 }, (_, i) => {
  const angle = (i / 16) * Math.PI * 2 - Math.PI / 2;
  const outer = i % 2 ? 41.5 : 43.5;
  return {
    x1: 48 + Math.cos(angle) * 39,
    y1: 48 + Math.sin(angle) * 39,
    x2: 48 + Math.cos(angle) * outer,
    y2: 48 + Math.sin(angle) * outer,
  };
});

function prefersReducedMotion(): boolean {
  return typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

/** Replaces the empty thread pane after the viewer clears an inbox. */
export function InboxClearedCelebration({ celebration }: { celebration: InboxCelebration }) {
  const rootRef = useRef<HTMLDivElement>(null);
  const sealRef = useRef<HTMLDivElement>(null);
  const { message } = celebration;

  useEffect(() => {
    const root = rootRef.current;
    const seal = sealRef.current;
    if (!root || !seal || prefersReducedMotion()) return;
    const sealBox = seal.getBoundingClientRect();
    return launchInboxClearedConfetti({
      origin: { x: sealBox.left + sealBox.width / 2, y: sealBox.top + sealBox.height / 2 },
      area: root.getBoundingClientRect(),
      size: celebration.clearedCount,
      dark: document.documentElement.classList.contains('dark'),
    });
    // One show per celebration, even if the copy re-renders.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [celebration.id]);

  return (
    <div
      ref={rootRef}
      role="status"
      aria-live="polite"
      data-rarity={message.rarity}
      data-inbox-cleared
      className="inbox-cleared relative flex min-h-[280px] flex-1 flex-col items-center justify-center overflow-hidden px-6 py-12"
    >
      <div key={celebration.id} className="relative flex w-full max-w-[400px] flex-col items-center text-center">
        <div ref={sealRef} className="inbox-cleared-seal mb-3.5 h-24 w-24" aria-hidden="true">
          <svg viewBox="0 0 96 96" className="h-full w-full overflow-visible">
            <circle className="inbox-cleared-orbit" cx="48" cy="48" r="46" />
            {TICKS.map((tick, i) => (
              <line
                key={i}
                className="inbox-cleared-tick"
                x1={tick.x1.toFixed(2)}
                y1={tick.y1.toFixed(2)}
                x2={tick.x2.toFixed(2)}
                y2={tick.y2.toFixed(2)}
                style={{ '--i': i } as CSSProperties}
              />
            ))}
            <circle className="inbox-cleared-ring" cx="48" cy="48" r="34" />
            <circle className="inbox-cleared-disc" cx="48" cy="48" r="27" />
            <path className="inbox-cleared-check" d="M36.5 48.5l7.5 7.5 15.5-16" />
          </svg>
        </div>
        <div className="inbox-cleared-tone flex items-center gap-2 text-[11.5px] font-semibold uppercase tracking-[0.04em]">
          <span>{RARITY_LABEL[message.rarity]}</span>
          <span aria-hidden="true" className="h-[11px] w-px bg-quiet-meta-separator" />
          <span className="text-quiet-text-secondary">{message.name}</span>
        </div>
        <h2 className="inbox-cleared-title mt-3 text-[22px] font-semibold leading-tight tracking-[-0.02em] text-quiet-text-primary [text-wrap:balance]">
          {message.title.split(' ').map((word, i) => (
            <span key={i}>
              {i > 0 && ' '}
              <span className="inbox-cleared-word inline-block" style={{ '--i': i } as CSSProperties}>{word}</span>
            </span>
          ))}
        </h2>
        <p className="inbox-cleared-body mt-2.5 max-w-[360px] text-sm leading-[1.7] text-quiet-text-secondary [text-wrap:pretty]">
          {message.body}
        </p>
      </div>
    </div>
  );
}
