'use client';

import { useEffect, useRef } from 'react';
import { useBentoPlayback } from './useBentoPlayback';
import { W, H, type MotionVariant } from './vortex-art';
import { createVortexRenderer, type VortexRenderer } from './vortex-renderer';
import './hero-vortex.css';

// Keep the animated canvas within a predictable pixel budget on large displays.
// Small stages can still use 2× resolution; the decorative lines retain their geometry.
const MAX_CANVAS_PIXELS = 2_000_000;

// Line geometry lives in vortex-art.ts and is drawn off the main thread (vortex.worker.ts).
// The canvas redraws its curves at a bounded resolution while the geometry itself moves:
// bundles twist, curves breathe, and ripples travel along each line.
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
    // The drift box overhangs the clipped stage; only paint the part that can be seen.
    const measure = () => {
      const width = drift.offsetWidth, height = drift.offsetHeight;
      const left = Math.max(0, -drift.offsetLeft), top = Math.max(0, -drift.offsetTop);
      const visibleWidth = Math.max(0, Math.min(width, art.clientWidth - drift.offsetLeft) - left);
      const visibleHeight = Math.max(0, Math.min(height, art.clientHeight - drift.offsetTop) - top);
      Object.assign(canvas.style, { left: `${left}px`, top: `${top}px`, width: `${visibleWidth}px`, height: `${visibleHeight}px` });
      return { width, height, left, top, visibleWidth, visibleHeight };
    };
    // Size the canvas before it enters the page: an unsized canvas that later grows to fill
    // the hero counts as a layout shift (CLS) on every page that uses this art.
    measure();
    drift.appendChild(canvas);
    const current = createVortexRenderer(canvas, variant);
    renderer.current = current;
    const observer = new ResizeObserver(() => {
      const { width, height, left, top, visibleWidth, visibleHeight } = measure();
      // Match an xMidYMid slice crop of the 1400×700 artwork inside the drift box.
      const scale = Math.max(width / W, height / H);
      const pixels = visibleWidth * visibleHeight;
      const budgetRatio = pixels ? Math.sqrt(MAX_CANVAS_PIXELS / pixels) : 2;
      const dpr = Math.max(1, Math.min(window.devicePixelRatio || 1, 2, budgetRatio));
      current.resize({
        width: visibleWidth, height: visibleHeight, dpr, scale,
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
