'use client';

import { useEffect, useRef } from 'react';
import { useBentoPlayback } from './useBentoPlayback';
import { W, H, type MotionVariant } from './vortex-art';
import { createVortexRenderer, type VortexRenderer } from './vortex-renderer';
import './hero-vortex.css';

// Line geometry lives in vortex-art.ts and is drawn off the main thread (vortex.worker.ts).
// The canvas redraws its curves at device resolution, so strokes stay sharp while the
// geometry itself moves: bundles twist, curves breathe, ripples travel along each line.
export function HeroVortex({ variant = 'vortex', tone = 'light' }: { variant?: MotionVariant; tone?: 'light' | 'dark' }) {
  const { container, playing } = useBentoPlayback(0);
  const driftRef = useRef<HTMLDivElement>(null);
  const renderer = useRef<VortexRenderer | null>(null);
  const playingRef = useRef(playing);
  playingRef.current = playing;

  useEffect(() => {
    const drift = driftRef.current, art = drift?.parentElement;
    if (!drift || !art) return;
    // A fresh canvas per mount: control of a canvas can be handed to the worker only once.
    const canvas = document.createElement('canvas');
    drift.appendChild(canvas);
    const current = createVortexRenderer(canvas, variant);
    renderer.current = current;
    // The drift box overhangs the clipped stage; only paint the part that can be seen.
    const observer = new ResizeObserver(() => {
      const width = drift.offsetWidth, height = drift.offsetHeight;
      const left = Math.max(0, -drift.offsetLeft), top = Math.max(0, -drift.offsetTop);
      const visibleWidth = Math.max(0, Math.min(width, art.clientWidth - drift.offsetLeft) - left);
      const visibleHeight = Math.max(0, Math.min(height, art.clientHeight - drift.offsetTop) - top);
      Object.assign(canvas.style, { left: `${left}px`, top: `${top}px`, width: `${visibleWidth}px`, height: `${visibleHeight}px` });
      // Match an xMidYMid slice crop of the 1400×700 artwork inside the drift box.
      const scale = Math.max(width / W, height / H);
      current.resize({
        width: visibleWidth, height: visibleHeight, dpr: Math.min(window.devicePixelRatio || 1, 2), scale,
        originX: (width - W * scale) / 2 - left, originY: (height - H * scale) / 2 - top,
      });
    });
    observer.observe(drift);
    observer.observe(art);
    current.play(playingRef.current);
    return () => {
      observer.disconnect();
      current.dispose();
      canvas.remove();
      renderer.current = null;
    };
  }, [variant]);

  useEffect(() => renderer.current?.play(playing), [playing]);

  return (
    <div ref={container} className="hero-vortex" data-playing={playing} data-variant={variant} data-tone={tone}>
      <div className="hero-vortex-art" aria-hidden="true">
        <div ref={driftRef} className="hero-vortex-drift" />
      </div>
    </div>
  );
}
