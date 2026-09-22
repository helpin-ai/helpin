'use client';

import { useEffect, useRef, useState } from 'react';
import { Inbox, PanelRight, Sparkles } from 'lucide-react';
import { SupportWorkspace } from './support-workspace';
import './support-inbox-showcase.css';

const DETAILS = [
  { title: 'Find the next conversation.', copy: 'Organize chat and email into shared and team inboxes.', icon: Inbox },
  { title: 'See what happened before.', copy: 'Review the customer’s history and the work already underway.', icon: PanelRight },
  { title: 'Reply with a clear next step.', copy: 'Use the findings to explain what happens next.', icon: Sparkles },
];

export function SupportInboxShowcase() {
  const [selected, setSelected] = useState<number | null>(null);
  const [preview, setPreview] = useState<number | null>(null);
  const active = preview ?? selected;
  const track = useRef<HTMLDivElement>(null);
  const [enhanced, setEnhanced] = useState(false);
  const [assisted, setAssisted] = useState(false);
  useEffect(() => {
    const media = matchMedia('(min-width: 1100px) and (min-height: 650px) and (prefers-reduced-motion: no-preference)');
    let frame = 0;
    const update = () => {
      frame = 0;
      const element = track.current;
      if (!element) return;
      // Fit the complete desktop canvas without switching to a stacked layout
      // merely because browser chrome makes the viewport shorter.
      const scale = innerHeight < 820 ? Math.min(1, (innerHeight - 220) / 600) : 1;
      element.style.setProperty('--inbox-preview-scale', String(scale));
      setEnhanced(media.matches);
      const distance = element.offsetHeight - (element.firstElementChild as HTMLElement).offsetHeight;
      setAssisted(media.matches && 80 - element.getBoundingClientRect().top > distance * 0.45);
    };
    const schedule = () => { if (!frame) frame = requestAnimationFrame(update); };
    const observer = new ResizeObserver(schedule);
    if (track.current) observer.observe(track.current);
    media.addEventListener('change', schedule);
    window.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', schedule);
    update();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      media.removeEventListener('change', schedule);
      window.removeEventListener('scroll', schedule);
      window.removeEventListener('resize', schedule);
    };
  }, []);
  function selectStep(second: boolean) {
    const element = track.current;
    if (!element) return;
    const distance = element.offsetHeight - (element.firstElementChild as HTMLElement).offsetHeight;
    window.scrollTo({ top: scrollY + element.getBoundingClientRect().top - 80 + (second ? distance * 0.8 : 0), behavior: 'instant' });
  }
  return <figure className="support-inbox-showcase" data-enhanced={enhanced} onFocusCapture={event => {
    // Reveal inbox controls when keyboard navigation reaches behind the overlay.
    if (enhanced && assisted && (event.target as HTMLElement).closest('.swi-shell')) selectStep(false);
  }}>
    <div ref={track} className="support-inbox-scroll" data-enhanced={enhanced}>
      <div className="support-inbox-sticky">
        <div className="support-inbox-stage"><SupportWorkspace scrollStory assisted={!enhanced || assisted} highlight={active} /></div>
        <p className="support-inbox-scroll-caption">{enhanced && assisted ? 'Ask Agent brings the findings together. Your teammate reviews the reply.' : 'Start with the conversation. Keep the customer’s history in view.'}</p>
      </div>
    </div>
    <figcaption><div className="support-inbox-callouts">{DETAILS.map(({ title, copy, icon: Icon }, index) => <button key={title} type="button" aria-pressed={selected === index} aria-label={`Highlight area ${index + 1}: ${title}`} data-active={active === index} onMouseEnter={() => setPreview(index)} onMouseLeave={() => setPreview(null)} onFocus={() => setPreview(index)} onBlur={() => setPreview(null)} onClick={() => setSelected(value => value === index ? null : index)}><span className="support-inbox-callout-number">{String(index + 1).padStart(2, '0')}</span><span className="support-inbox-callout-copy"><span className="support-inbox-callout-title"><Icon size={15} aria-hidden="true" />{title}</span><span>{copy}</span></span></button>)}</div></figcaption>
  </figure>;
}
