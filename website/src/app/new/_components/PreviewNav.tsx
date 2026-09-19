'use client';

import { useEffect, useRef, useState } from 'react';
import Link from 'next/link';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, SIGNUP_URL, GithubIcon } from './ui';

const LINKS = [
  { label: 'Product', href: '/new/product' },
  { label: 'Developers', href: '/new#developers' },
  { label: 'Open Source', href: '/new#open-source' },
  { label: 'Docs', href: 'https://github.com/helpin-ai/helpin/blob/develop/docs/README.md' },
];

export function PreviewNav() {
  const [hidden, setHidden] = useState(false);
  const nav = useRef<HTMLElement>(null);

  useEffect(() => {
    let previousY = Math.max(0, window.scrollY);
    let travel = 0;
    let frame = 0;
    const update = () => {
      frame = 0;
      const y = Math.max(0, window.scrollY);
      const delta = y - previousY;
      previousY = y;
      if (y <= 60 || nav.current?.querySelector(':focus-visible')) {
        travel = 0;
        setHidden(false);
        return;
      }
      if (!delta) return;
      if (Math.sign(delta) !== Math.sign(travel)) travel = 0;
      travel += delta;
      // Ignore small scroll jitter, but reveal promptly when direction changes.
      if (travel > 20) setHidden(true);
      else if (travel < -10) setHidden(false);
    };
    const onScroll = () => { if (!frame) frame = window.requestAnimationFrame(update); };
    window.addEventListener('scroll', onScroll, { passive: true });
    return () => {
      window.removeEventListener('scroll', onScroll);
      window.cancelAnimationFrame(frame);
    };
  }, []);

  return (
    <nav ref={nav} className="pnav" aria-label="Main navigation" data-hidden={hidden} onFocusCapture={event => { if (event.target.matches(':focus-visible')) setHidden(false); }}>
      <div className="wrap">
        <Link href="/new" className="logo" aria-label="Helpin"><HelpinBrand /></Link>
        <div className="navlinks">
          {LINKS.map((l) => (
            <span key={l.label}><a href={l.href}>{l.label}</a></span>
          ))}
        </div>
        <div className="navright">
          <a className="gh" href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
            <GithubIcon />
            GitHub
          </a>
          <a href="https://app.helpin.ai">Sign in</a>
          <Link className="btn btn-primary" href={SIGNUP_URL}>Start free →</Link>
        </div>
      </div>
    </nav>
  );
}
