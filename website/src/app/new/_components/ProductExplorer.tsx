'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowUpRight } from 'lucide-react';

const AREAS = [
  {
    id: 'inbox', label: 'Inbox',
    title: 'Answer with the whole story.',
    description: 'Agents draft replies using customer history and product knowledge. Your team reviews the answer with the context already attached.',
  },
  {
    id: 'meetings', label: 'Meetings',
    title: 'Keep the work moving after the call.',
    description: 'Capture decisions and next steps from customer meetings. Give agents the conversation behind every action item.',
  },
  {
    id: 'projects', label: 'Projects',
    title: 'From customer request to engineering work.',
    description: 'Agents help turn feedback into tasks, plan the next steps, and carry the customer’s requirements into development.',
  },
  {
    id: 'crm', label: 'CRM',
    title: 'Know what matters before you follow up.',
    description: 'Give agents the conversations, deal history, and buyer signals behind each account so they can prepare a relevant next step.',
  },
  {
    id: 'knowledge', label: 'Knowledge',
    title: 'Turn what changed into useful answers.',
    description: 'Agents help draft and update documentation from customer questions and product work, with the source material close at hand.',
  },
] as const;

export function ProductExplorer() {
  const [active, setActive] = useState(0);
  const [horizontal, setHorizontal] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(true);
  const [scrollDriven, setScrollDriven] = useState(false);
  const track = useRef<HTMLDivElement>(null);
  const container = useRef<HTMLDivElement>(null);
  const tabList = useRef<HTMLDivElement>(null);
  const tabs = useRef<Array<HTMLButtonElement | null>>([]);
  const geometry = useRef({ start: 0, step: 1 });

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
    let frame = 0;
    let enabled = false;

    const update = () => {
      frame = 0;
      if (!enabled) return;
      const { start, step } = geometry.current;
      const position = Math.max(0, Math.min(AREAS.length, (window.scrollY - start) / step));
      const index = Math.min(AREAS.length - 1, Math.floor(position));
      const fraction = Math.min(1, position - index);
      // Fade only around scene boundaries. Holding the scroll holds the frame.
      const entering = index === 0 ? 1 : Math.min(1, fraction / 0.16);
      const leaving = index === AREAS.length - 1 ? 1 : Math.min(1, (1 - fraction) / 0.12);
      const opacity = Math.max(0, Math.min(entering, leaving));
      rail.style.setProperty('--px-opacity', String(opacity));
      rail.style.setProperty('--px-shift', `${(1 - opacity) * 12}px`);
      rail.style.setProperty('--px-progress', String(fraction));
      setActive(index);
    };
    const schedule = () => { if (!frame) frame = window.requestAnimationFrame(update); };
    const measure = () => {
      const top = parseFloat(getComputedStyle(scene).top) || 80;
      const sceneHeight = scene.offsetHeight;
      // Short viewports use manual tabs so content never becomes trapped offscreen.
      enabled = !reducedMotion && sceneHeight <= window.innerHeight - top - 12;
      setScrollDriven(enabled);
      if (enabled) {
        const step = window.innerHeight * 0.7;
        rail.style.height = `${sceneHeight + step * AREAS.length}px`;
        geometry.current = { start: rail.getBoundingClientRect().top + window.scrollY - top, step };
        schedule();
      } else {
        rail.style.removeProperty('height');
        rail.style.removeProperty('--px-opacity');
        rail.style.removeProperty('--px-shift');
        rail.style.removeProperty('--px-progress');
      }
    };
    measure();
    const resize = new ResizeObserver(measure);
    resize.observe(scene);
    // Earlier lazy-loaded content can change this section's document offset.
    resize.observe(document.body);
    window.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', measure);
    return () => {
      resize.disconnect();
      window.removeEventListener('scroll', schedule);
      window.removeEventListener('resize', measure);
      window.cancelAnimationFrame(frame);
      rail.style.removeProperty('height');
    };
  }, [reducedMotion]);

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
    setActive(index);
    if (scrollDriven) {
      const { start, step } = geometry.current;
      window.scrollTo({ top: start + (index + 0.35) * step, behavior: 'instant' });
    }
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

  return <div ref={track} className={`px-scroll-track${scrollDriven ? ' is-scroll-driven' : ''}`}>
    <div ref={container} className="product-explorer">
    <div className="px-navigation">
      <div ref={tabList} className="px-tabs" role="tablist" aria-label="Product areas" aria-orientation={horizontal ? 'horizontal' : 'vertical'}>
        {AREAS.map(({ id, label }, index) => <button key={id} ref={node => { tabs.current[index] = node; }} type="button" role="tab" id={`product-tab-${id}`} aria-controls={`product-panel-${id}`} aria-selected={active === index} tabIndex={active === index ? 0 : -1} onClick={() => selectTab(index)} onKeyDown={event => onKeyDown(event, index)}>{label}{active === index && <span className="px-tab-progress" aria-hidden="true" />}</button>)}
      </div>
    </div>
    {AREAS.map((area, index) => <div className="px-panel" role="tabpanel" id={`product-panel-${area.id}`} aria-labelledby={`product-tab-${area.id}`} hidden={active !== index} tabIndex={0} key={area.id}>
      <div className="px-copy"><h3>{area.title}</h3>{' '}<p>{area.description}</p><a href={`/new/product#${area.id}`}>Explore {area.label.toLowerCase()} <ArrowUpRight size={15} aria-hidden="true" /></a></div>
      {/* Keep the approved workspace image intact until individual product screenshots are supplied. */}
      <div className="px-stage">
        <a className="px-product-image" href="/new/product/workspace-product-v1.webp" target="_blank" rel="noopener noreferrer" aria-label="View the Helpin workspace board at full size (opens in a new tab)">
          <img src="/new/product/workspace-product-v1.webp" alt="Helpin Studio SSO task board: Okta mapping and setup docs in progress, Acme’s security requirements in review, and role mapping shipped." width={1727} height={910} loading="lazy" decoding="async" />
        </a>
      </div>
    </div>)}
    </div>
  </div>;
}
