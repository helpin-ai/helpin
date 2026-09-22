import { useEffect, useId, useRef, useState, type CSSProperties } from 'react';
import { PauseIcon, PlayIcon } from '@/lib/icons';
import './public-page-vortex.css';

// Adapted from website/src/app/new/_components/HeroVortex.tsx.
// Keep the website's line geometry and small compositor-animated light layers.
const CX = 700, CY = 350, LENGTH = 1100, SPREAD = 190, GAP = 185;
const BUNDLES = [
  { axis: 'v', offset: -GAP }, { axis: 'v', offset: GAP },
  { axis: 'h', offset: -GAP }, { axis: 'h', offset: GAP },
].map(({ axis, offset }) => ({
  transform: `rotate(${axis === 'v' ? 9 : -9} ${CX} ${CY})`,
  lines: Array.from({ length: 14 }, (_, index) => {
    const t = index / 13 - 0.5;
    const position = offset + t * SPREAD;
    const bow = t * 130;
    return {
      d: axis === 'v'
        ? `M ${CX + position - bow * 0.4} ${CY - LENGTH} Q ${CX + position + bow} ${CY} ${CX + position - bow * 0.4} ${CY + LENGTH}`
        : `M ${CX - LENGTH} ${CY + position - bow * 0.4} Q ${CX} ${CY + position + bow} ${CX + LENGTH} ${CY + position - bow * 0.4}`,
      opacity: Math.max(0.07, 0.55 - Math.abs(t) * 0.7),
    };
  }),
}));


export function PublicPageVortex() {
  const id = useId().replace(/:/g, '');
  const container = useRef<HTMLDivElement>(null);
  const lights = useRef<HTMLDivElement>(null);
  const [paused, setPaused] = useState(false);
  const [playing, setPlaying] = useState(false);

  // Same visibility/reduced-motion lifecycle as the website's useBentoPlayback.
  useEffect(() => {
    const element = container.current;
    if (!element) return;
    const media = window.matchMedia('(prefers-reduced-motion: reduce)');
    let visible = false;
    const update = () => setPlaying(visible && !document.hidden && !media.matches && !paused);
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting;
      update();
    }, { threshold: 0, rootMargin: '120px 0px' });
    update();
    observer.observe(element);
    media.addEventListener('change', update);
    document.addEventListener('visibilitychange', update);
    return () => {
      observer.disconnect();
      media.removeEventListener('change', update);
      document.removeEventListener('visibilitychange', update);
    };
  }, [paused]);

  useEffect(() => {
    const layer = lights.current;
    const artwork = layer?.parentElement;
    if (!layer || !artwork) return;
    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      layer.style.transform = `translate(-50%, -50%) scale(${Math.max(width / 1400, height / 700)})`;
      layer.dataset.ready = 'true';
    });
    observer.observe(artwork);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={container} className="public-vortex" data-playing={playing}>
      <div className="public-vortex-art" aria-hidden="true">
        <div className="public-vortex-drift">
          <svg viewBox="0 0 1400 700" preserveAspectRatio="xMidYMid slice" focusable="false">
            <defs>
              <linearGradient id={`${id}-base`} x1="0" y1="0" x2="1" y2="1">
                <stop stopColor="#0B7A4E" /><stop offset="50%" stopColor="#0F9D63" /><stop offset="100%" stopColor="#3FC48F" />
              </linearGradient>
              <linearGradient id={`${id}-sheen`} x1="0" y1="0" x2="1" y2="0.3">
                <stop stopColor="#2AE79A" stopOpacity="0" /><stop offset="42%" stopColor="#2AE79A" stopOpacity="0" />
                <stop offset="50%" stopColor="#74FFC4" /><stop offset="58%" stopColor="#2AE79A" stopOpacity="0" /><stop offset="100%" stopColor="#2AE79A" stopOpacity="0" />
              </linearGradient>
            </defs>
            {BUNDLES.map((bundle, bi) => (
              <g key={bi} transform={bundle.transform} fill="none" strokeLinecap="round">
                <g stroke={`url(#${id}-base)`} strokeWidth="4" opacity="0.22">
                  {bundle.lines.map((line, j) => <path key={j} d={line.d} opacity={line.opacity} />)}
                </g>
                <g stroke={`url(#${id}-base)`} strokeWidth="1">
                  {bundle.lines.map((line, j) => <path key={j} d={line.d} opacity={line.opacity} />)}
                </g>
                <g className="public-vortex-sheen" stroke={`url(#${id}-sheen)`} strokeWidth="1.5">
                  {bundle.lines.map((line, j) => <path key={j} d={line.d} opacity={line.opacity * 1.9} />)}
                </g>
              </g>
            ))}
          </svg>
          <div ref={lights} className="public-vortex-lights">
            {BUNDLES.map((bundle, bi) => {
              const [angle, x, y] = bundle.transform.match(/-?[\d.]+/g)!.map(Number);
              return <div key={bi} className="public-vortex-bundle" style={{ transform: `translate(${x}px, ${y}px) rotate(${angle}deg) translate(${-x}px, ${-y}px)` }}>
                {bundle.lines.filter((_, j) => j % 3 === 1).map((line, j) => (
                  <span key={j} className="public-vortex-light" style={{
                    offsetPath: `path("${line.d}")`,
                    '--light-duration': `${6 + (j % 3) * 1.2}s`,
                    '--light-delay': `${-(j * 1.7 + bi * 2.1)}s`,
                  } as CSSProperties} />
                ))}
              </div>;
            })}
          </div>
        </div>
      </div>
      <button type="button" className="public-vortex-toggle" onClick={() => setPaused(value => !value)} aria-label={paused ? 'Play background animation' : 'Pause background animation'}>
        {paused ? <PlayIcon className="size-4" aria-hidden="true" /> : <PauseIcon className="size-4" aria-hidden="true" />}
      </button>
    </div>
  );
}
