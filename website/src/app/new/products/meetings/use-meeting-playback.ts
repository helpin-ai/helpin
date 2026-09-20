'use client';
import { useEffect, useState } from 'react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';

export function useMeetingPlayback() {
  const { container, playing, cycle } = useBentoPlayback(18000);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(3);
  const active = playing && !paused;
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [2200, 4800, 8500].map((delay, i) => setTimeout(() => setFrame(i + 1), delay));
    return () => timers.forEach(clearTimeout);
  }, [active, cycle]);
  return { container, active, phase: active ? frame : 3, paused, setPaused };
}
