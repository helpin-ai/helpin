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
    // Elements waiting offscreen hold their start state with plain inline styles. A paused
    // animation would do the same visually, but keeps a compositor layer alive per element.
    const waiting = new Map<HTMLElement, { delay: number; translate: string }>();
    const running = new Map<HTMLElement, Animation>();
    let observer: IntersectionObserver | undefined;

    const release = (element: HTMLElement) => {
      element.style.removeProperty('opacity');
      element.style.removeProperty('translate');
      waiting.delete(element);
    };

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
      Array.from(waiting.keys()).forEach(release);
    };

    const reveal = (element: HTMLElement) => {
      const pending = waiting.get(element);
      if (!pending) return;
      // Start the animation before releasing the inline start state; the backwards fill holds
      // the same start keyframe through the delay, so there is no frame without either.
      const animation = element.animate(
        [{ opacity: 0, translate: pending.translate }, { opacity: 1, translate: '0 0' }],
        { duration: 500, delay: pending.delay, easing: 'cubic-bezier(.22,1,.36,1)', fill: 'backwards' },
      );
      release(element);
      running.set(element, animation);
      animation.onfinish = () => running.delete(element);
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
          if (element.contains(document.activeElement)) release(element);
          else reveal(element);
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
        element.style.opacity = '0';
        element.style.translate = translate;
        waiting.set(element, { delay, translate });
        observer.observe(element);
      }
    };

    const onFocus = (event: Event) => {
      if (!(event.target instanceof Node)) return;
      for (const element of Array.from(waiting.keys())) {
        if (element.contains(event.target)) release(element);
      }
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
