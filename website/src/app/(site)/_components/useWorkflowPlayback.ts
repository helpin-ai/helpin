'use client';

import { useEffect, useRef, useState } from 'react';
import { useBentoPlayback } from './useBentoPlayback';

// One timeout per story beat. Visibility and pause preserve the current beat;
// changing an example restarts it. Reduced motion shows the completed outcome.
const DEFAULT_BEATS = [0, 1200, 3000, 4800, 6600, 8200];
export function useWorkflowPlayback({ beats = DEFAULT_BEATS, duration = 14000, paused = false, resetKey = '' }: {
  beats?: readonly number[]; duration?: number; paused?: boolean; resetKey?: string | number;
} = {}) {
  const { container, playing, reducedMotion } = useBentoPlayback(0, !paused);
  const [phase, setPhase] = useState(0);
  const [cycle, setCycle] = useState(0);
  const elapsed = useRef(0);
  const schedule = beats.join(',');

  useEffect(() => {
    elapsed.current = 0;
    setPhase(reducedMotion ? schedule.split(',').length - 1 : 0);
    setCycle(value => value + 1);
  }, [resetKey, reducedMotion, schedule]);

  useEffect(() => {
    if (!playing) return;
    const times = schedule.split(',').map(Number);
    let started = performance.now();
    let timer: ReturnType<typeof setTimeout>;
    const advance = () => {
      const current = elapsed.current + performance.now() - started;
      if (current >= duration) {
        elapsed.current = 0;
        started = performance.now();
        setCycle(value => value + 1);
        setPhase(0);
        timer = setTimeout(advance, times[1] ?? duration);
        return;
      }
      setPhase(Math.max(0, times.reduce((last, time, index) => current >= time ? index : last, 0)));
      const next = times.find(time => time > current) ?? duration;
      timer = setTimeout(advance, Math.max(1, next - current));
    };
    advance();
    return () => {
      clearTimeout(timer);
      elapsed.current = Math.min(duration, elapsed.current + performance.now() - started);
    };
  }, [playing, schedule, duration, resetKey, reducedMotion]);

  return { container, phase, cycle, playing, reducedMotion };
}
