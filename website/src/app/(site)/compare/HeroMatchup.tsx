'use client';

import { useState } from 'react';
import { ArrowDown, Check, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../_components/useBentoPlayback';
import type { Competitor } from './compare-data';

type Props = { name: string; glance: Competitor['glance'] };

// Three cards set the other tool's approach beside Helpin's. Each card fills in turn,
// then the scene settles and replays. Paused, reduced motion, and server HTML all show
// the finished comparison.
export function HeroMatchup({ name, glance }: Props) {
  const { container, playing, cycle } = useBentoPlayback(12000);
  const [paused, setPaused] = useState(false);

  return (
    <div ref={container} className="cmp-hero-scene" data-playing={playing && !paused}>
      <div className="cmp-hero-toolbar">
        <span className="cmp-hero-versus">
          <img className="cmp-mark" src="/brand/helpin-icon-white.svg" width={18} height={18} alt="" />
          <strong>Helpin</strong>
          <em>vs</em>
          <strong>{name}</strong>
        </span>
        <button type="button" aria-label={`${paused ? 'Play' : 'Pause'} comparison animation`} aria-pressed={paused} onClick={() => setPaused(!paused)}>
          {paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}
        </button>
      </div>
      <ol key={cycle} className="cmp-hero-cards" aria-label={`Where Helpin and ${name} differ most`}>
        {glance.map((row, index) => (
          <li key={row.label} className="cmp-hero-card" style={{ '--i': index } as React.CSSProperties}>
            <div className="cmp-hero-card-label"><span>{String(index + 1).padStart(2, '0')}</span>{row.label}</div>
            <div className="cmp-hero-card-body">
              <div className="cmp-hero-side cmp-hero-rival"><span className="cmp-hero-side-name">{name}</span><p>{row.competitor}</p></div>
              <span className="cmp-hero-vs" aria-hidden="true">vs</span>
              <div className="cmp-hero-side cmp-hero-helpin"><span className="cmp-hero-side-name">Helpin</span><p><Check size={13} strokeWidth={2.4} aria-hidden="true" />{row.helpin}</p></div>
            </div>
          </li>
        ))}
      </ol>
      <p className="cmp-hero-foot"><span>Where the two differ most</span><a href="#side-by-side">Full comparison<ArrowDown size={12} aria-hidden="true" /></a></p>
    </div>
  );
}
