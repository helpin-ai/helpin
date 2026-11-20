'use client';

import { useEffect, useState } from 'react';
import { useBentoPlayback } from './useBentoPlayback';
import './hero-headline.css';

const phrases = [
  'answer your customers.',
  'keep help docs up to date.',
  'follow up with customers.',
  'hand over with the findings.',
  'turn bug reports into fixes.',
  'tell customers when it ships.',
];

export function HeroHeadline() {
  const { container, playing } = useBentoPlayback(0);
  const [index, setIndex] = useState(0);

  useEffect(() => {
    if (!playing) return;
    setIndex(0);
    let interval: ReturnType<typeof setInterval> | undefined;
    // Hold the first phrase for 3s, then allow 300ms + 3s for each next phrase.
    const timeout = setTimeout(() => {
      setIndex(1);
      interval = setInterval(() => setIndex(value => (value + 1) % phrases.length), 3300);
    }, 3000);
    return () => {
      clearTimeout(timeout);
      clearInterval(interval);
    };
  }, [playing]);

  const active = playing ? index : 0;
  return (
    <div ref={container} className="hero-headline" data-playing={playing}>
      <h1 aria-label={`AI support agents that ${phrases[0]}`}>
        <span aria-hidden="true">AI support agents that</span>
        <span className="hero-headline-phrases" aria-hidden="true">
          {phrases.map((phrase, i) => (
            <span
              key={phrase}
              className="hero-headline-phrase"
              data-state={i === active ? 'active' : i === (active + phrases.length - 1) % phrases.length ? 'outgoing' : 'waiting'}
            >
              {phrase}
            </span>
          ))}
        </span>
      </h1>
    </div>
  );
}
