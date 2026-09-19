'use client';

import { useEffect } from 'react';
import { usePathname } from 'next/navigation';

// Animate content groups, leaving the hero, sticky product tour, and demo timelines
// to their own motion. Content stays visible without JavaScript or observer support.
const GROUPS = [
  '.sec-head', '.record-intro', '.ask-agent-intro', '.final',
  '.record-bento', '.ask-agent-capabilities', '.ctrl',
  '.self-host-features', '.developer-features', '.developer-cli-copy',
  '.pm-items', '.six', '.steps.big', '.footer-nav',
];
const SINGLE_ITEMS = [
  '.ask-agent-screenshot', '.control-screenshot', '.developer-visual',
  '.self-host-intro .links', '.self-host-license', '.ask-agent-links',
  '.section-close', '.rs-close', '.footer-brand', '.footer-community', '.footer-bottom',
];

export function ScrollReveal() {
  const pathname = usePathname();

  useEffect(() => {
    const root = document.querySelector('.hp3');
    if (!root || typeof IntersectionObserver === 'undefined' || typeof Element.prototype.animate !== 'function') return;

    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const targets = new Map<HTMLElement, number>();
    const revealed = new Set<HTMLElement>();
    const running = new Map<HTMLElement, Animation>();
    let observer: IntersectionObserver | undefined;

    for (const selector of GROUPS) {
      root.querySelectorAll(selector).forEach(group => {
        Array.from(group.children).forEach((child, index) => {
          if (child instanceof HTMLElement) targets.set(child, Math.min(index, 3) * 75);
        });
      });
    }
    root.querySelectorAll<HTMLElement>(SINGLE_ITEMS.join(',')).forEach(element => targets.set(element, 0));

    const stop = () => {
      observer?.disconnect();
      running.forEach(animation => animation.cancel());
      running.clear();
    };

    const start = () => {
      stop();
      if (motion.matches) return;
      observer = new IntersectionObserver(entries => {
        for (const entry of entries) {
          if (!entry.isIntersecting) continue;
          const element = entry.target as HTMLElement;
          observer?.unobserve(element);
          if (revealed.has(element)) continue;
          revealed.add(element);
          // A restored scroll position or keyboard jump should never hide focused content.
          if (element.contains(document.activeElement)) continue;
          const animation = element.animate(
            [{ opacity: 0, translate: '0 18px' }, { opacity: 1, translate: '0 0' }],
            { duration: 650, delay: targets.get(element) ?? 0, easing: 'cubic-bezier(.22,1,.36,1)', fill: 'backwards' },
          );
          running.set(element, animation);
          animation.onfinish = () => running.delete(element);
        }
      }, { threshold: 0, rootMargin: '0px 0px 24px 0px' });

      targets.forEach((_, element) => {
        // Don't replay sections already above the viewport on a deep link or restoration.
        if (element.getBoundingClientRect().bottom <= 0) revealed.add(element);
        if (!revealed.has(element)) observer?.observe(element);
      });
    };

    const onFocus = (event: Event) => {
      if (!(event.target instanceof Node)) return;
      for (const [element, animation] of running) {
        if (element.contains(event.target)) {
          animation.cancel();
          running.delete(element);
        }
      }
    };

    start();
    motion.addEventListener('change', start);
    root.addEventListener('focusin', onFocus);
    return () => {
      stop();
      motion.removeEventListener('change', start);
      root.removeEventListener('focusin', onFocus);
    };
  }, [pathname]);

  return null;
}
