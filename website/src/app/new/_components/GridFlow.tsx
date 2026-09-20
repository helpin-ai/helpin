'use client';

import { useEffect, useRef, useState, type CSSProperties } from 'react';

/** Decorative routes share the background grid's spacing and centered origin. */
export function GridFlow() {
  const container = useRef<HTMLDivElement>(null);
  const [size, setSize] = useState({ width: 0, height: 0, step: 64 });
  const [playing, setPlaying] = useState(false);

  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    let visible = false;
    const updatePlayback = () => setPlaying(visible && !motion.matches && !document.hidden);
    const measure = () => {
      const { width, height } = element.getBoundingClientRect();
      const step = parseFloat(getComputedStyle(element).getPropertyValue('--grid-step')) || 64;
      setSize(previous => previous.width === width && previous.height === height && previous.step === step
        ? previous : { width, height, step });
    };
    const resize = new ResizeObserver(measure);
    const intersection = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      updatePlayback();
    });
    measure();
    resize.observe(element);
    intersection.observe(element);
    motion.addEventListener('change', updatePlayback);
    document.addEventListener('visibilitychange', updatePlayback);
    return () => {
      resize.disconnect();
      intersection.disconnect();
      motion.removeEventListener('change', updatePlayback);
      document.removeEventListener('visibilitychange', updatePlayback);
    };
  }, []);

  const { width, height, step } = size;
  // CSS background-position: center top centers one grid tile, not a grid line.
  const origin = ((width / 2 - step / 2) % step + step) % step + .5;
  const columns = Math.floor((width - origin) / step);
  const bands = Math.max(1, Math.ceil(height / (step * 6)));
  const routes = width > 0 && columns >= 3 ? Array.from({ length: bands * 2 }, (_, index) => {
    const band = Math.floor(index / 2);
    const right = index % 2 === 1;
    const direction = right ? -1 : 1;
    const x = origin + (right ? columns : 0) * step;
    const row = Math.min(Math.floor(height / step) - 3, 1 + band * 6 + (right ? 1 : 0));
    const y = Math.max(1, row) * step + .5;
    const endX = x + direction * step * 2;
    const endY = y + step * 2;
    return {
      path: `M ${x} ${y} h ${direction * step} v ${step * 2} h ${direction * step}`,
      endX, endY,
      duration: `${10 + index % 3 * 1.5}s`,
      delay: `${-((index * 3.7 + 1) % 11)}s`,
    };
  }) : [];

  return (
    <div ref={container} className="grid-flow" data-playing={playing} aria-hidden="true">
      <svg width="100%" height="100%" focusable="false">
        {routes.map((route, index) => (
          <g key={`${index}-${width}-${height}-${step}`} style={{ '--flow-duration': route.duration, '--flow-delay': route.delay } as CSSProperties}>
            <path className="grid-flow-trail" d={route.path} pathLength="100" />
            <g className="grid-flow-point" style={{ offsetPath: `path('${route.path}')` }}>
              <circle r="7" className="grid-flow-glow" />
              <circle r="2" />
            </g>
            <g transform={`translate(${route.endX} ${route.endY})`}>
              <circle className="grid-flow-complete" r="8" />
            </g>
          </g>
        ))}
      </svg>
    </div>
  );
}
