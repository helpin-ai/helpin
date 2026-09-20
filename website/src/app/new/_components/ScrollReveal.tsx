'use client';

import { useEffect } from 'react';
import { usePathname } from 'next/navigation';

// Animate content groups, leaving the hero, sticky product tour, and demo timelines
// to their own motion. Content stays visible without JavaScript or observer support.
const GROUPS = [
  '.sec-head', '.record-intro', '.ask-agent-intro', '.final',
  '.record-bento-copy', '.ask-agent-capabilities > article', '.ctrl-copy',
  '.self-host-features', '.developer-features', '.developer-cli-copy',
  '.pm-items', '.six', '.steps.big', '.footer-nav', '.support-features',
];
const SINGLE_ITEMS = [
  '.ask-agent-screenshot', '.control-screenshot', '.developer-visual',
  '.self-host-intro .links', '.self-host-license', '.ask-agent-links',
  '.section-close', '.rs-close', '.footer-brand', '.footer-community', '.footer-bottom',
  '.support-context-card', '.support-work-card',
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
          if (!(child instanceof HTMLElement)) return;
          // Keep the illustration's viewport geometry stable while its own scene runs.
          if (child.matches('[data-playing]') || child.querySelector('[data-playing]')) return;
          targets.set(child, Math.min(index, 3) * 50);
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
          const animation = running.get(element);
          if (!animation) continue;
          if (element.contains(document.activeElement)) {
            animation.cancel();
            running.delete(element);
          } else {
            animation.play();
          }
        }
      }, { threshold: 0, rootMargin: '0px 0px 120px 0px' });

      // Read geometry together, before applying any animation styles. Content that
      // is already visible (including restored scroll positions) must never blink.
      const positions = Array.from(targets, ([element, delay]) => ({
        element, delay, top: element.getBoundingClientRect().top,
      }));
      for (const { element, delay, top } of positions) {
        if (top < window.innerHeight || element.contains(document.activeElement)) revealed.add(element);
        if (revealed.has(element)) continue;
        // Prepare the hidden start state offscreen, rather than hiding a card once
        // it has already appeared. Large product images only fade; text moves gently.
        const translate = element.querySelector('img, svg') ? '0 0' : '0 12px';
        const animation = element.animate(
          [{ opacity: 0, translate }, { opacity: 1, translate: '0 0' }],
          { duration: 500, delay, easing: 'cubic-bezier(.22,1,.36,1)', fill: 'both' },
        );
        animation.pause();
        running.set(element, animation);
        animation.onfinish = () => {
          animation.cancel();
          running.delete(element);
        };
        observer.observe(element);
      }
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
