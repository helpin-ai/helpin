'use client';

import { createElement, useEffect, useRef, useState, type ReactNode } from 'react';

type MotionState = 'static' | 'waiting' | 'playing';

// Plays a CSS entrance sequence once, the first time the element scrolls into view.
// Server HTML and reduced motion keep the complete, static state. Content that is
// already on screen when the page loads stays still, so it never blinks.
export function InViewOnce({ as = 'div', className, children }: { as?: 'div' | 'article' | 'ol'; className?: string; children: ReactNode }) {
  const ref = useRef<HTMLElement>(null);
  const [state, setState] = useState<MotionState>('static');

  useEffect(() => {
    const element = ref.current;
    if (!element || typeof IntersectionObserver === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) return;
    if (element.getBoundingClientRect().top < window.innerHeight) return;
    setState('waiting');
    const observer = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) return;
      setState('playing');
      observer.disconnect();
    }, { rootMargin: '0px 0px -12% 0px' });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

  return createElement(as, { ref, className, 'data-motion': state }, children);
}
