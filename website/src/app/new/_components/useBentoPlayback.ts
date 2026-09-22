'use client';

import { useEffect, useRef, useState } from 'react';

// Start just before a scene enters view and keep it running until it fully leaves.
// A single 50% cutoff repeatedly restarted scenes during small scroll movements.
export function useBentoPlayback(duration = 6000, enabled = true) {
  const container = useRef<HTMLDivElement>(null);
  const [reducedMotion, setReducedMotion] = useState(true);
  const [playing, setPlaying] = useState(false);
  const [cycle, setCycle] = useState(0);

  useEffect(() => {
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => setReducedMotion(media.matches);
    update();
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);

  useEffect(() => {
    if (reducedMotion || !enabled) {
      setPlaying(false);
      return;
    }
    const element = container.current;
    if (!element) return;
    let visible = false;
    const update = () => setPlaying(visible && !document.hidden);
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      update();
    }, { threshold: 0, rootMargin: '120px 0px' });
    observer.observe(element);
    document.addEventListener('visibilitychange', update);
    return () => {
      observer.disconnect();
      document.removeEventListener('visibilitychange', update);
    };
  }, [enabled, reducedMotion]);

  useEffect(() => {
    // CSS-only scenes pass zero: visibility control without a React cycle timer.
    if (!playing || !enabled || duration <= 0) return;
    // Let the completed scene settle briefly before replaying the sequence.
    const timer = setInterval(() => setCycle(value => value + 1), duration);
    return () => clearInterval(timer);
  }, [playing, duration, enabled]);

  return { container, reducedMotion, playing: playing && !reducedMotion && enabled, cycle };
}
