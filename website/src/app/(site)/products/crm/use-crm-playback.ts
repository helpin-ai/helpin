"use client";
import { useEffect, useState } from "react";
import { useBentoPlayback } from "../../_components/useBentoPlayback";
export function useCRMPlayback(enabled = true) {
  const { container, playing, cycle } = useBentoPlayback(20000);
  const [paused, setPaused] = useState(false);
  const [frame, setFrame] = useState(3);
  const active = enabled && playing && !paused;
  useEffect(() => {
    if (!active) return;
    setFrame(0);
    const timers = [2600, 5600, 9600].map((delay, index) =>
      setTimeout(() => setFrame(index + 1), delay),
    );
    return () => timers.forEach(clearTimeout);
  }, [active, cycle]);
  return { container, active, phase: active ? frame : 3, paused, setPaused };
}
