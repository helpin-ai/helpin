'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowDown, ArrowRight, Check, CheckCheck, Inbox, ListFilter, Mail, Pause, Play, Route, Tag, Users } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-inbox-features.css';

const FEATURES = [
  {
    id: 'email', label: 'Email forwarding',
    title: 'Keep your support address. Give your team a shared inbox.',
    copy: 'Forward email from your existing address into Helpin. Questions arrive as conversations your team can assign, annotate, and answer alongside live chat.',
    points: ['Forward into the shared inbox or a specific team inbox.', 'Test your forwarding setup and see when it is verified.', 'Configure a verified sender address for replies from your domain.'],
    description: 'Email sent to support@orbitdesk.example is forwarded into the shared inbox. The forwarding setup is verified, and Maya’s email arrives as a conversation.',
  },
  {
    id: 'routing', label: 'Routing & assignment',
    title: 'Get each question to the team that can answer it.',
    copy: 'Give billing, technical support, and other teams their own inboxes. Use routing rules or AI triage to suggest a destination, and enable automatic moves when you’re ready.',
    points: ['Route by message content or sender details.', 'Keep team inbox access limited to its members.', 'Assign work manually or use round-robin assignment.'],
    description: 'A customer asks for an invoice update. A rule matches the word invoice and routes the conversation to Billing. Round-robin assignment selects Sam Rivera.',
  },
  {
    id: 'organize', label: 'Tags & saved views',
    title: 'Turn a busy inbox into a clear view of the work.',
    copy: 'Tag conversations by topic, request, or product area. Combine filters into saved views so your team can return to the conversations that need attention.',
    points: ['Create and apply the tags that fit your workflow.', 'Filter conversations by tags, owner, and status.', 'Save personal views or share them with your team.'],
    description: 'Maya’s CSV export issue is tagged Bug and Exports. A saved team view filters open conversations tagged Exports and shows that issue alongside another export question.',
  },
] as const;

function Illustration({ variant }: { variant: string }) {
  if (variant === 'email') return <>
    <div className="inbox-proof-heading"><Mail size={16} /><span>Forwarding setup</span><span className="inbox-proof-status"><Check size={12} /> Verified</span></div>
    <div className="inbox-proof-step inbox-proof-address"><span>YOUR EXISTING ADDRESS</span><strong>support@orbitdesk.example</strong></div>
    <div className="inbox-proof-connector inbox-proof-step"><ArrowDown size={18} /><span>Email forwarding</span></div>
    <div className="inbox-proof-step inbox-proof-destination"><Inbox size={20} /><div><strong>Shared inbox</strong><span>OrbitDesk</span></div><CheckCheck size={16} /></div>
    <div className="inbox-proof-step inbox-proof-email"><div className="inbox-proof-person"><img src="/new/avatars/maya.webp" width={28} height={28} alt="" /><div><strong>Maya Chen</strong><span>Northstar Labs · Email</span></div><small>New</small></div><strong>Can you help with our CSV export?</strong><p>The export stops before all our rows are included.</p><div className="inbox-proof-foot"><Mail size={13} /> The email is now a shared conversation.</div></div>
  </>;
  if (variant === 'routing') return <>
    <div className="inbox-proof-heading"><Route size={16} /><span>Routing & assignment</span><span className="inbox-proof-status"><Check size={12} /> Enabled</span></div>
    <div className="inbox-proof-step inbox-proof-question"><span>NEW CONVERSATION</span><p>“Can you update the company name on our invoice?”</p></div>
    <div className="inbox-proof-step inbox-proof-rule"><span>WHEN THE MESSAGE CONTAINS</span><strong>invoice</strong><ArrowRight size={16} /><div><span>MOVE TO</span><strong>Billing</strong></div></div>
    <div className="inbox-proof-connector inbox-proof-step"><ArrowDown size={18} /><span>Round-robin assignment</span></div>
    <div className="inbox-proof-step inbox-proof-owner"><img src="/new/avatars/sam.webp" width={38} height={38} alt="" /><div><strong>Sam Rivera</strong><span>Billing inbox · Assigned</span></div><CheckCheck size={17} /></div>
    <div className="inbox-proof-step inbox-proof-result"><Check size={14} /> The right inbox. A clear owner.</div>
  </>;
  return <>
    <div className="inbox-proof-heading"><ListFilter size={16} /><span>Organize conversations</span><span className="inbox-proof-status"><Users size={12} /> Team view</span></div>
    <div className="inbox-proof-step inbox-proof-question"><span>MAYA CHEN · NORTHSTAR LABS</span><p>CSV export stops before all rows are included</p><div className="inbox-proof-tags"><span><Tag size={11} /> Bug</span><span><Tag size={11} /> Exports</span></div></div>
    <div className="inbox-proof-step inbox-proof-view"><div><ListFilter size={16} /><strong>Open export issues</strong></div><div className="inbox-proof-filters"><span>Status: Open</span><span>Tag: Exports</span></div></div>
    <div className="inbox-proof-step inbox-proof-results"><div><span className="support-live-dot" /><strong>CSV export stops early</strong><small>Maya</small></div><div><span className="support-live-dot" /><strong>Missing date column</strong><small>Alex</small></div></div>
    <div className="inbox-proof-step inbox-proof-result"><Check size={14} /> A saved view your team can come back to.</div>
  </>;
}

function InboxProof({ feature, paused, onToggle }: { feature: typeof FEATURES[number]; paused: boolean; onToggle: () => void }) {
  const { container, playing, cycle } = useBentoPlayback(8500);
  return <div className="inbox-feature-proof" ref={container} data-playing={playing && !paused}>
    <div className="inbox-proof-toolbar"><span>OrbitDesk</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} inbox feature animation`} aria-pressed={paused} onClick={onToggle}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <div key={cycle} className={`inbox-proof-scene inbox-proof-scene-${feature.id}`} role="img" aria-label={feature.description}><div aria-hidden="true"><Illustration variant={feature.id} /></div></div>
  </div>;
}

export function SupportInboxFeatures() {
  const [selected, setSelected] = useState(0);
  const [paused, setPaused] = useState(false);
  const [horizontal, setHorizontal] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(true);
  const [scrollDriven, setScrollDriven] = useState(false);
  const track = useRef<HTMLDivElement>(null);
  const scene = useRef<HTMLDivElement>(null);
  const geometry = useRef({ start: 0, step: 1 });
  const tabList = useRef<HTMLDivElement>(null);
  const tabs = useRef<(HTMLButtonElement | null)[]>([]);
  const feature = FEATURES[selected];

  useEffect(() => {
    const narrow = window.matchMedia('(max-width: 760px)');
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => { setHorizontal(narrow.matches); setReducedMotion(motion.matches); };
    update();
    narrow.addEventListener('change', update);
    motion.addEventListener('change', update);
    return () => { narrow.removeEventListener('change', update); motion.removeEventListener('change', update); };
  }, []);

  useEffect(() => {
    const rail = track.current;
    const element = scene.current;
    if (!rail || !element) return;
    let frame = 0;
    let enabled = false;
    const update = () => {
      frame = 0;
      if (!enabled) return;
      const { start, step } = geometry.current;
      const position = Math.max(0, Math.min(FEATURES.length, (window.scrollY - start) / step));
      const index = Math.min(FEATURES.length - 1, Math.floor(position));
      const fraction = Math.min(1, position - index);
      const entering = index === 0 ? 1 : Math.min(1, fraction / .16);
      const leaving = index === FEATURES.length - 1 ? 1 : Math.min(1, (1 - fraction) / .12);
      const opacity = Math.min(entering, leaving);
      rail.style.setProperty('--inbox-opacity', String(opacity));
      rail.style.setProperty('--inbox-shift', `${(1 - opacity) * 12}px`);
      rail.style.setProperty('--inbox-progress', String(fraction));
      setSelected(index);
    };
    const schedule = () => { if (!frame) frame = requestAnimationFrame(update); };
    const measure = () => {
      const top = parseFloat(getComputedStyle(element).top) || 80;
      const height = element.offsetHeight;
      // Match the homepage: only pin a scene if the entire panel fits.
      enabled = !reducedMotion && height <= window.innerHeight - top - 12;
      setScrollDriven(enabled);
      if (enabled) {
        const step = window.innerHeight * .7;
        rail.style.height = `${height + step * FEATURES.length}px`;
        geometry.current = { start: rail.getBoundingClientRect().top + window.scrollY - top, step };
        schedule();
      } else {
        rail.style.removeProperty('height');
        rail.style.removeProperty('--inbox-opacity');
        rail.style.removeProperty('--inbox-shift');
        rail.style.removeProperty('--inbox-progress');
      }
    };
    measure();
    const resize = new ResizeObserver(measure);
    resize.observe(element);
    resize.observe(document.body);
    window.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', measure);
    return () => {
      resize.disconnect();
      window.removeEventListener('scroll', schedule);
      window.removeEventListener('resize', measure);
      cancelAnimationFrame(frame);
      rail.style.removeProperty('height');
    };
  }, [reducedMotion]);

  function selectFeature(index: number) {
    setSelected(index);
    if (scrollDriven) {
      const { start, step } = geometry.current;
      window.scrollTo({ top: start + (index + .35) * step, behavior: 'instant' });
    }
  }

  useEffect(() => {
    if (!horizontal) return;
    const list = tabList.current;
    const tab = tabs.current[selected];
    if (!list || !tab) return;
    const listRect = list.getBoundingClientRect();
    const tabRect = tab.getBoundingClientRect();
    list.scrollTo({ left: list.scrollLeft + tabRect.left - listRect.left - (list.clientWidth - tabRect.width) / 2, behavior: 'instant' });
  }, [selected, horizontal]);

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    let next = index;
    if (event.key === (horizontal ? 'ArrowRight' : 'ArrowDown')) next = (index + 1) % FEATURES.length;
    else if (event.key === (horizontal ? 'ArrowLeft' : 'ArrowUp')) next = (index + FEATURES.length - 1) % FEATURES.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = FEATURES.length - 1;
    else return;
    event.preventDefault();
    selectFeature(next);
    tabs.current[next]?.focus({ preventScroll: true });
  }

  return <div ref={track} className={`inbox-feature-track${scrollDriven ? ' is-scroll-driven' : ''}`}><div ref={scene} className="inbox-features">
    <div className="inbox-feature-navigation">
    <div ref={tabList} className="inbox-feature-tabs" role="tablist" aria-label="Explore inbox features" aria-orientation={horizontal ? 'horizontal' : 'vertical'}>{FEATURES.map(({ id, label }, index) => <button key={id} ref={element => { tabs.current[index] = element; }} type="button" role="tab" id={`inbox-tab-${id}`} aria-controls={`inbox-panel-${id}`} aria-selected={selected === index} tabIndex={selected === index ? 0 : -1} onClick={() => selectFeature(index)} onKeyDown={event => onKeyDown(event, index)}>{label}<span className="inbox-feature-progress" aria-hidden="true" /></button>)}</div>
      <ul className="inbox-feature-points">{feature.points.map(point => <li key={point}><Check size={14} aria-hidden="true" /><span>{point}</span></li>)}</ul>
      {feature.id === 'routing' && <p className="inbox-feature-note">Round-robin assignment is available on eligible plans.</p>}
    </div>
    {FEATURES.map(({ id, title, copy }, index) => <div key={id} role="tabpanel" id={`inbox-panel-${id}`} aria-labelledby={`inbox-tab-${id}`} hidden={selected !== index} tabIndex={0}>
      {selected === index && <div className="inbox-feature-panel">
        <div className="inbox-feature-copy"><h3>{title}</h3><p>{copy}</p></div>
        <InboxProof key={id} feature={feature} paused={paused} onToggle={() => setPaused(value => !value)} />
      </div>}
    </div>)}
  </div></div>;
}
