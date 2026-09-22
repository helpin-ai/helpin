'use client';

import { useEffect, useId, useRef, type CSSProperties } from 'react';
import { useBentoPlayback } from './useBentoPlayback';
import './hero-vortex.css';

// Adapted from the teammate's VortexLines.jsx handoff (September 2026).
// Source: https://storage.googleapis.com/vm-dev-screenshots/Helpin.ai%20Open%20Source%20Alternative.zip
// Keep the large SVG static; small light streaks follow its paths on separate
// layers so flowing highlights do not repaint the whole line illustration.
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

type MotionVariant = 'vortex' | 'flow' | 'connections' | 'orbit' | 'converge';

// Related line families share the same playback and accessibility contract.
const FLOW = [0, 1].map(group => ({
  transform: `rotate(${group ? -7 : 7} ${CX} ${CY})`,
  lines: Array.from({ length: 12 }, (_, i) => ({
    d: `M -120 ${130 + group * 250 + i * 13} C 330 ${-80 + group * 370 + i * 17}, 810 ${530 - group * 220 + i * 9}, 1520 ${150 + group * 260 + i * 12}`,
    opacity: 0.22 + (1 - Math.abs(i - 5.5) / 6) * 0.3,
  })),
}));
const CONNECTIONS = [-1, 1].map(side => ({
  transform: side < 0 ? 'translate(1400 0) scale(-1 1)' : '',
  lines: Array.from({ length: 12 }, (_, i) => ({
    d: `M -100 ${70 + i * 23} H ${230 + i * 20} Q ${270 + i * 20} ${70 + i * 23} ${270 + i * 20} ${110 + i * 23} V ${440 + i * 11} Q ${270 + i * 20} ${480 + i * 11} ${310 + i * 20} ${480 + i * 11} H 1500`,
    opacity: 0.24 + (i % 4) * 0.06,
  })),
}));
const ORBITS = [{
  transform: `rotate(-14 ${CX} ${CY})`,
  lines: Array.from({ length: 18 }, (_, i) => ({
    d: `M ${CX - 340 - i * 18} ${CY} a ${340 + i * 18} ${150 + i * 11} 0 1 0 ${2 * (340 + i * 18)} 0 a ${340 + i * 18} ${150 + i * 11} 0 1 0 ${-2 * (340 + i * 18)} 0`,
    opacity: 0.18 + (1 - Math.abs(i - 8.5) / 9) * 0.3,
  })),
}];
const CONVERGING = [0, 1].map(side => ({
  transform: side ? 'translate(1400 0) scale(-1 1)' : '',
  lines: Array.from({ length: 10 }, (_, i) => ({
    d: `M -120 ${40 + i * 44} C 200 ${40 + i * 44}, 350 ${470 + i * 5}, 700 560`,
    opacity: 0.28 + (i % 3) * 0.07,
  })),
}));
const ART = { vortex: BUNDLES, flow: FLOW, connections: CONNECTIONS, orbit: ORBITS, converge: CONVERGING };

function bundleTransform(transform: string) {
  if (!transform) return undefined;
  if (transform.startsWith('rotate')) {
    const [angle, x, y] = transform.match(/-?[\d.]+/g)!.map(Number);
    return `translate(${x}px, ${y}px) rotate(${angle}deg) translate(${-x}px, ${-y}px)`;
  }
  return 'translateX(1400px) scaleX(-1)';
}

export function HeroVortex({ variant = 'vortex', tone = 'light' }: { variant?: MotionVariant; tone?: 'light' | 'dark' }) {
  const id = useId().replace(/:/g, '');
  const { container, playing } = useBentoPlayback(0);
  const lights = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const layer = lights.current;
    const artwork = layer?.parentElement;
    if (!layer || !artwork) return;
    // Match the SVG's xMidYMid slice crop at every responsive stage size.
    const observer = new ResizeObserver(([entry]) => {
      const { width, height } = entry.contentRect;
      layer.style.transform = `translate(-50%, -50%) scale(${Math.max(width / 1400, height / 700)})`;
      layer.dataset.ready = 'true';
    });
    observer.observe(artwork);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={container} className="hero-vortex" data-playing={playing} data-variant={variant} data-tone={tone}>
      <div className="hero-vortex-art" aria-hidden="true">
        <div className="hero-vortex-drift">
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
            {ART[variant].map((bundle, bi) => (
              <g key={bi} transform={bundle.transform} fill="none" strokeLinecap="round">
                <g stroke={`url(#${id}-base)`} strokeWidth="4" opacity="0.22">
                  {bundle.lines.map((line, j) => <path key={j} d={line.d} opacity={line.opacity} />)}
                </g>
                <g stroke={`url(#${id}-base)`} strokeWidth="1">
                  {bundle.lines.map((line, j) => <path key={j} d={line.d} opacity={line.opacity} />)}
                </g>
                <g className="hero-vortex-sheen" stroke={`url(#${id}-sheen)`} strokeWidth="1.5">
                  {bundle.lines.map((line, j) => <path key={j} d={line.d} opacity={line.opacity * 1.9} />)}
                </g>
              </g>
            ))}
          </svg>
          <div ref={lights} className="hero-vortex-lights">
            {ART[variant].map((bundle, bi) => (
              <div key={bi} className="hero-vortex-light-bundle" style={{ transform: bundleTransform(bundle.transform) }}>
                {bundle.lines.filter((_, j) => j % 3 === 1).map((line, j) => (
                  <span key={j} className="hero-vortex-light" style={{
                    offsetPath: `path("${line.d}")`,
                    '--light-duration': `${6 + (j % 3) * 1.2}s`,
                    '--light-delay': `${-(j * 1.7 + bi * 2.1)}s`,
                  } as CSSProperties} />
                ))}
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
