'use client';

import { useEffect, useRef, useState, type KeyboardEvent, type ReactNode } from 'react';
import { ArrowUpRight } from 'lucide-react';
import { ProductPreview, type ProductPreviewName } from './product-previews';

const AREAS = [
  {
    id: 'inbox', label: 'Support', href: '/products/customer-support',
    title: 'Answer customer questions and keep track of every conversation.',
    description: <>Manage chat and email in <span className="home-feature">shared team inboxes</span>. The Echo agent can use <span className="home-feature">your docs</span> and check <span className="home-feature">connected tools</span> for <span className="home-feature">code, logs, and customer account details</span> before replying. It can follow up to see if the customer still needs help and bring your team in when needed.</>,
  },
  {
    id: 'meetings', label: 'Meetings', href: '/products/meetings',
    title: 'Keep the notes and action items from your meetings.',
    description: <>Record internal meetings and customer calls, then get <span className="home-feature">AI summaries and action items</span>. Use Helpin AI to <span className="home-feature">create follow-up tasks</span> so the decisions made in a meeting turn into work your team can track.</>,
  },
  {
    id: 'projects', label: 'Projects', href: '/products/projects',
    title: 'AI agents plan and build. Your team reviews.',
    description: <>Organize <span className="home-feature">tasks, stories, epics, and sprints</span>, and track progress against your <span className="home-feature">roadmap and objectives</span>. Scribe can plan a change, Forge can write the code, and Lens can review it.</>,
  },
  {
    id: 'crm', label: 'CRM', href: '/products/crm',
    title: 'See which leads need a follow-up and what to say.',
    description: <>Keep contacts, deals, emails, and meeting notes together. The Beacon agent helps you <span className="home-feature">spot buying interest</span>, understand what is holding up a deal, and <span className="home-feature">prepare a follow-up</span> based on the conversation.</>,
  },
  {
    id: 'knowledge', label: 'Knowledge', href: '/products/knowledge',
    title: 'Keep your help docs and internal guides up to date as your product changes.',
    description: <>Use <span className="home-feature">internal docs spaces</span> for team guides and processes. The Quill agent can prepare <span className="home-feature">article updates</span> from new releases and unanswered customer questions, including fresh <span className="home-feature">screenshots</span> and <span className="home-feature">browser recordings</span>. Your team reviews and publishes the changes so customers and teammates have current instructions.</>,
  },
] as const satisfies ReadonlyArray<{ id: ProductPreviewName; label: string; href: string; title: string; description: ReactNode }>;

const TAB_DURATIONS_MS = [14000, 13000, 28500, 14500, 14000];

export function ProductExplorer() {
  const [active, setActive] = useState(0);
  const [focused, setFocused] = useState(false);
  const elapsed = useRef(0);
  const [horizontal, setHorizontal] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(true);
  const track = useRef<HTMLDivElement>(null);
  const container = useRef<HTMLDivElement>(null);
  const tabList = useRef<HTMLDivElement>(null);
  const tabs = useRef<Array<HTMLButtonElement | null>>([]);

  useEffect(() => {
    const narrow = window.matchMedia('(max-width: 760px)');
    const motion = window.matchMedia('(prefers-reduced-motion: reduce)');
    const update = () => {
      setHorizontal(narrow.matches);
      setReducedMotion(motion.matches);
    };
    update();
    narrow.addEventListener('change', update);
    motion.addEventListener('change', update);
    return () => {
      narrow.removeEventListener('change', update);
      motion.removeEventListener('change', update);
    };
  }, []);

  useEffect(() => {
    const rail = track.current;
    const scene = container.current;
    if (!rail || !scene) return;
    const duration = TAB_DURATIONS_MS[active];
    rail.style.setProperty('--px-progress', String(reducedMotion ? 1 : elapsed.current / duration));
    if (focused || reducedMotion) return;

    let frame = 0;
    let previous: number | null = null;
    let visible = false;
    const tick = (now: number) => {
      if (previous !== null) elapsed.current += now - previous;
      previous = now;
      rail.style.setProperty('--px-progress', String(Math.min(1, elapsed.current / duration)));
      if (elapsed.current >= duration) {
        elapsed.current = 0;
        setActive(index => (index + 1) % AREAS.length);
        return;
      }
      frame = window.requestAnimationFrame(tick);
    };
    const updatePlayback = () => {
      window.cancelAnimationFrame(frame);
      previous = null;
      if (visible && !document.hidden) frame = window.requestAnimationFrame(tick);
    };
    const observer = new IntersectionObserver(([entry]) => {
      visible = entry.isIntersecting && entry.intersectionRatio >= 0.15;
      updatePlayback();
    }, { threshold: [0, 0.15] });
    observer.observe(scene);
    document.addEventListener('visibilitychange', updatePlayback);
    return () => {
      observer.disconnect();
      document.removeEventListener('visibilitychange', updatePlayback);
      window.cancelAnimationFrame(frame);
    };
  }, [active, focused, reducedMotion]);

  useEffect(() => {
    if (!horizontal) return;
    const list = tabList.current;
    const tab = tabs.current[active];
    if (!list || !tab) return;
    const listRect = list.getBoundingClientRect();
    const tabRect = tab.getBoundingClientRect();
    list.scrollTo({ left: list.scrollLeft + tabRect.left - listRect.left - (list.clientWidth - tabRect.width) / 2, behavior: 'instant' });
  }, [active, horizontal]);

  function selectTab(index: number) {
    elapsed.current = 0;
    track.current?.style.setProperty('--px-progress', reducedMotion ? '1' : '0');
    setActive(index);
  }

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>, index: number) {
    const nextKey = horizontal ? 'ArrowRight' : 'ArrowDown';
    const previousKey = horizontal ? 'ArrowLeft' : 'ArrowUp';
    let next = index;
    if (event.key === nextKey) next = (index + 1) % AREAS.length;
    else if (event.key === previousKey) next = (index - 1 + AREAS.length) % AREAS.length;
    else if (event.key === 'Home') next = 0;
    else if (event.key === 'End') next = AREAS.length - 1;
    else return;
    event.preventDefault();
    selectTab(next);
    tabs.current[next]?.focus({ preventScroll: true });
  }

  return <div ref={track} className="px-scroll-track">
    <div ref={container} className="product-explorer" onFocusCapture={() => setFocused(true)} onBlurCapture={event => { if (!event.currentTarget.contains(event.relatedTarget)) setFocused(false); }}>
    <div className="px-navigation">
      <div ref={tabList} className="px-tabs" role="tablist" aria-label="Product areas" aria-orientation={horizontal ? 'horizontal' : 'vertical'}>
        {AREAS.map(({ id, label }, index) => <button key={id} ref={node => { tabs.current[index] = node; }} type="button" role="tab" id={`product-tab-${id}`} aria-controls={`product-panel-${id}`} aria-selected={active === index} tabIndex={active === index ? 0 : -1} onClick={() => selectTab(index)} onKeyDown={event => onKeyDown(event, index)}>{label}{active === index && <span className="px-tab-progress" aria-hidden="true" />}</button>)}
      </div>
    </div>
    {AREAS.map((area, index) => <div className="px-panel" role="tabpanel" id={`product-panel-${area.id}`} aria-labelledby={`product-tab-${area.id}`} hidden={active !== index} tabIndex={0} key={area.id}>
      <div className="px-copy"><h3>{area.title}</h3>{' '}<p>{area.description}</p><a href={area.href}>Explore {area.id === 'inbox' ? 'customer support' : area.id === 'crm' ? 'CRM' : area.label.toLowerCase()} <ArrowUpRight size={15} aria-hidden="true" /></a></div>
      <div className="px-stage">
        {(active === index) && <ProductPreview product={area.id} theme="light" workflow />}
      </div>
    </div>)}
    </div>
  </div>;
}
