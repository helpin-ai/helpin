'use client';

import { useEffect, useRef, useState } from 'react';
import { Maximize2, Pause, Play, RotateCcw, Volume2, VolumeX } from 'lucide-react';
import type { compareVideo } from './compare-data';

type Props = { id: string; name: string; video: ReturnType<typeof compareVideo> };

// The comparison video under the hero. Only the poster loads with the page; the file
// buffers as the video nears the viewport, plays muted while half of it is on screen,
// and pauses when it leaves. Reduced motion never autoplays. The last frame is the end
// card, so it stops there instead of looping. Without JavaScript it keeps native controls.
export function CompareFilm({ id, name, video }: Props) {
  const frame = useRef<HTMLDivElement>(null);
  const player = useRef<HTMLVideoElement>(null);
  const userPaused = useRef(false);
  const [enhanced, setEnhanced] = useState(false);
  const [preload, setPreload] = useState<'none' | 'auto'>('none');
  const [playing, setPlaying] = useState(false);
  const [muted, setMuted] = useState(true);
  const [ended, setEnded] = useState(false);
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
      if (!reduced && !userPaused.current && !media.ended) media.play().catch(() => {});
    }, { threshold: 0.5 });
    near.observe(element);
    visible.observe(element);
    return () => { near.disconnect(); visible.disconnect(); };
  }, []);

  function toggle() {
    const media = player.current;
    if (!media) return;
    if (media.ended) { media.currentTime = 0; }
    if (media.paused) { userPaused.current = false; media.play().catch(() => {}); }
    else { userPaused.current = true; media.pause(); }
  }

  function toggleSound() {
    const media = player.current;
    if (!media) return;
    const next = !media.muted;
    media.muted = next;
    setMuted(next);
    if (!next && media.paused) {
      if (media.ended) media.currentTime = 0;
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

  const PlayIcon = ended ? RotateCcw : playing ? Pause : Play;
  const playLabel = ended ? 'Replay the video' : playing ? 'Pause the video' : 'Play the video';

  return (
    <figure id={id} className="cmp-film" aria-label={video.title}>
      <div ref={frame} className="cmp-film-frame" data-playing={playing} data-enhanced={enhanced}>
        <video
          ref={player}
          src={video.src}
          poster={video.poster}
          preload={preload}
          muted={muted}
          playsInline
          controls={!enhanced}
          aria-describedby={`${id}-summary`}
          onClick={enhanced ? toggle : undefined}
          onPlay={() => { setPlaying(true); setEnded(false); }}
          onPause={() => setPlaying(false)}
          onEnded={() => { setPlaying(false); setEnded(true); }}
          onTimeUpdate={event => setProgress(event.currentTarget.currentTime / (event.currentTarget.duration || video.seconds))}
        />
        <span className="cmp-film-progress" aria-hidden="true"><span style={{ transform: `scaleX(${Math.min(1, progress)})` }} /></span>
      </div>
      <figcaption>
        {enhanced ? (
          <div className="cmp-film-controls">
            <button type="button" onClick={toggle} aria-label={playLabel}><PlayIcon size={15} aria-hidden="true" /></button>
            <button type="button" onClick={toggleSound} aria-label={muted ? 'Turn sound on' : 'Turn sound off'}>
              {muted ? <VolumeX size={15} aria-hidden="true" /> : <Volume2 size={15} aria-hidden="true" />}
              <span aria-hidden="true">{muted ? 'Sound on' : 'Sound off'}</span>
            </button>
            <button type="button" onClick={fullScreen} aria-label="Watch full screen"><Maximize2 size={15} aria-hidden="true" /></button>
          </div>
        ) : null}
        <div className="cmp-film-text">
          <strong>Helpin vs {name} in {video.seconds} seconds</strong>
          <span id={`${id}-summary`}>{video.summary}</span>
        </div>
      </figcaption>
    </figure>
  );
}
