'use client';

import { useId, useState, type CSSProperties } from 'react';
import { Pause, Play } from 'lucide-react';
import { useBentoPlayback } from './useBentoPlayback';
import './hero-vortex.css';

// Adapted from the teammate's VortexLines.jsx handoff (September 2026).
// Source: https://storage.googleapis.com/vm-dev-screenshots/Helpin.ai%20Open%20Source%20Alternative.zip
// Keep its bowed # geometry and staggered trails; wide translucent strokes
// replace animated blur to avoid rerasterizing a large filtered SVG each frame.
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

export function HeroVortex() {
  const id = useId().replace(/:/g, '');
  const [paused, setPaused] = useState(false);
  const { container, playing, reducedMotion } = useBentoPlayback(16_000, !paused);

  return (
    <div ref={container} className="hero-vortex" data-playing={playing}>
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
            {BUNDLES.map((bundle, bi) => (
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
                <g stroke="#3FC48F" strokeWidth="1.6" strokeDasharray="80 680">
                  {bundle.lines.map((line, j) => (
                    <path key={j} className="hero-vortex-trail" d={line.d} opacity={Math.min(0.95, line.opacity * 2.2)} style={{
                      '--vortex-duration': `${5.5 + (j % 4) * 0.9}s`,
                      '--vortex-delay': `${-(j * 0.7 + bi * 1.6)}s`,
                    } as CSSProperties} />
                  ))}
                </g>
              </g>
            ))}
          </svg>
        </div>
      </div>
      <button className="hero-vortex-toggle" type="button" hidden={reducedMotion}
        onClick={() => setPaused(value => !value)}
        aria-label={paused ? 'Play background animation' : 'Pause background animation'}
        title={paused ? 'Play background animation' : 'Pause background animation'}>
        {paused ? <Play size={14} aria-hidden="true" /> : <Pause size={14} aria-hidden="true" />}
      </button>
    </div>
  );
}
