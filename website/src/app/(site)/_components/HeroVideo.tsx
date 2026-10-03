'use client';

import { useEffect, useRef, useState } from 'react';
import { Play } from 'lucide-react';
import './hero-video.css';

export function HeroVideo({ src, poster }: { src: string; poster: string }) {
  const container = useRef<HTMLElement>(null);
  const video = useRef<HTMLVideoElement>(null);
  const [playing, setPlaying] = useState(false);
  const [started, setStarted] = useState(false);
  const [previewActive, setPreviewActive] = useState(false);

  useEffect(() => {
    const element = container.current;
    const connection = (navigator as Navigator & { connection?: { saveData?: boolean } }).connection;
    if (!element || connection?.saveData || started) return;
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    let visible = false;
    const update = () => setPreviewActive(visible && !document.hidden && !motion.matches);
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting && entry.intersectionRatio >= 0.15;
      update();
    }, { threshold: [0, 0.15] });
    observer.observe(element);
    motion.addEventListener('change', update);
    document.addEventListener('visibilitychange', update);
    return () => {
      observer.disconnect();
      motion.removeEventListener('change', update);
      document.removeEventListener('visibilitychange', update);
    };
  }, [started]);

  async function play() {
    try {
      await video.current?.play();
    } catch {
      // Leave the native player available if playback cannot start.
      video.current?.focus();
    }
  }

  return <figure ref={container} className="hero-workflow hero-film" id="intro-video">
    <video ref={video} controls playsInline preload="none" width={1920} height={1080} poster={poster} aria-label="Introducing Helpin" onPlay={() => { setStarted(true); setPlaying(true); }} onPause={() => setPlaying(false)} onEnded={() => setPlaying(false)}>
      <source src={src} type="video/mp4" />
      <a href={src}>Watch the Helpin introduction</a>
    </video>
    {previewActive && !started && <video className="hero-video-preview" src="/new/home/helpin-launch-preview-v1.mp4" autoPlay muted loop playsInline preload="none" width={960} height={540} aria-hidden="true" tabIndex={-1} onPlaying={event => { event.currentTarget.dataset.ready = 'true'; }} />}
    {!playing && <button type="button" className="hero-video-play" aria-label="Play Helpin introduction" onClick={play}><Play size={30} fill="currentColor" aria-hidden="true" /></button>}
  </figure>;
}
