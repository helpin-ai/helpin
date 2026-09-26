'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { Maximize2, Volume2, VolumeX } from 'lucide-react';
import type { compareVideo } from './compare-data';

type Props = { id: string; name: string; video: ReturnType<typeof compareVideo> };

// The comparison video in the hero. It plays muted and on a loop as soon as the page is
// interactive and a quarter of it is on screen, and pauses while it's scrolled away.
// Sound and full screen sit inside the frame; clicking the video, or Space/Enter while it
// has focus, pauses it. Reduced motion never autoplays. Without JavaScript it keeps
// native controls.
export function CompareFilm({ id, name, video }: Props) {
  const frame = useRef<HTMLDivElement>(null);
  const player = useRef<HTMLVideoElement>(null);
  const userPaused = useRef(false);
  const [enhanced, setEnhanced] = useState(false);
  const [preload, setPreload] = useState<'none' | 'auto'>('none');
  const [muted, setMuted] = useState(true);
  const [progress, setProgress] = useState(0);

  useEffect(() => {
    setEnhanced(true);
    const element = frame.current;
    const media = player.current;
    if (!element || !media || typeof IntersectionObserver === 'undefined') return;
    const reduced = window.matchMedia('(prefers-reduced-motion: reduce)').matches;
    const near = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) return;
      setPreload('auto');
      near.disconnect();
    }, { rootMargin: '600px 0px' });
    const visible = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) { media.pause(); return; }
      if (!reduced && !userPaused.current) media.play().catch(() => {});
    }, { threshold: 0.25 });
    near.observe(element);
    visible.observe(element);
    return () => { near.disconnect(); visible.disconnect(); };
  }, []);

  function toggle() {
    const media = player.current;
    if (!media) return;
    if (media.paused) { userPaused.current = false; media.play().catch(() => {}); }
    else { userPaused.current = true; media.pause(); }
  }

  function onKey(event: KeyboardEvent<HTMLVideoElement>) {
    if (event.key !== ' ' && event.key !== 'Enter') return;
    event.preventDefault();
    toggle();
  }

  function toggleSound() {
    const media = player.current;
    if (!media) return;
    const next = !media.muted;
    media.muted = next;
    setMuted(next);
    if (!next && media.paused) {
      userPaused.current = false;
      media.play().catch(() => {});
    }
  }

  function fullScreen() {
    const media = player.current as (HTMLVideoElement & { webkitEnterFullscreen?: () => void }) | null;
    if (!media) return;
    // iOS Safari only supports full screen on the video element itself.
    if (media.requestFullscreen) media.requestFullscreen().catch(() => {});
    else media.webkitEnterFullscreen?.();
  }

  return (
    <figure id={id} className="cmp-film" aria-label={video.title}>
      <div ref={frame} className="cmp-film-frame" data-enhanced={enhanced}>
        <video
          ref={player}
          src={video.src}
          poster={video.poster}
          preload={preload}
          muted={muted}
          loop
          playsInline
          controls={!enhanced}
          tabIndex={enhanced ? 0 : undefined}
          aria-label={video.title}
          aria-describedby={`${id}-summary`}
          aria-keyshortcuts={enhanced ? 'Space Enter' : undefined}
          onClick={enhanced ? toggle : undefined}
          onKeyDown={enhanced ? onKey : undefined}
          onTimeUpdate={event => setProgress(event.currentTarget.currentTime / (event.currentTarget.duration || video.seconds))}
        />
        {enhanced ? (
          <div className="cmp-film-controls">
            <button type="button" onClick={toggleSound} aria-label={muted ? 'Turn sound on' : 'Turn sound off'}>
              {muted ? <VolumeX size={16} aria-hidden="true" /> : <Volume2 size={16} aria-hidden="true" />}
            </button>
            <button type="button" onClick={fullScreen} aria-label="Watch full screen"><Maximize2 size={15} aria-hidden="true" /></button>
          </div>
        ) : null}
        <span className="cmp-film-progress" aria-hidden="true"><span style={{ transform: `scaleX(${Math.min(1, progress)})` }} /></span>
      </div>
      <figcaption>
        <strong>Helpin vs {name} in {video.seconds} seconds</strong>
        <span id={`${id}-summary`} className="sr-only">{video.summary}</span>
      </figcaption>
    </figure>
  );
}
