'use client';

import { useEffect, useRef, useState } from 'react';

// Both bento sections share the same visibility, motion, and loop behavior.
export function useBentoPlayback(duration = 6000) {
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
    if (reducedMotion) {
      setPlaying(false);
      return;
    }
    const element = container.current;
    if (!element) return;
    let visible = false;
    const update = () => setPlaying(visible && !document.hidden);
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.intersectionRatio >= .5;
      update();
    }, { threshold: .5 });
    observer.observe(element);
    document.addEventListener('visibilitychange', update);
    return () => {
      observer.disconnect();
      document.removeEventListener('visibilitychange', update);
    };
  }, [reducedMotion]);

  useEffect(() => {
    if (!playing) return;
    // Let the completed scene settle briefly before replaying the sequence.
    const timer = setInterval(() => setCycle(value => value + 1), duration);
    return () => clearInterval(timer);
  }, [playing, duration]);

  return { container, playing: playing && !reducedMotion, cycle };
}
