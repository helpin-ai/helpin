'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowUpRight } from 'lucide-react';

const PROJECTS_IMAGE = {
  src: '/new/product/workspace-projects-4k-v3.webp',
  alt: 'OrbitDesk engineering board with customer-reported bugs, feature requests, documentation, and follow-ups across Planned, In Progress, In Review, and Shipped.',
  width: 3840,
  height: 2160,
};

const INBOX_IMAGE = {
  src: '/new/product/workspace-inbox-4k-v4.webp',
  alt: 'OrbitDesk workspace inbox with Northstar Labs’ Maya Chen asking about Okta SSO, an internal note, a Helpin AI reply draft, and the linked SSO project.',
  width: 3840,
  height: 2160,
};

const MEETINGS_IMAGE = {
  src: '/new/meetings/recording-preview-1672-v4.webp',
  alt: 'OrbitDesk workspace meeting review for Northstar Labs, showing an Okta SSO rollout summary, a three-person call preview, decisions, assigned action items, and linked customer records.',
  width: 1672,
  height: 941,
};

const CRM_IMAGE = {
  src: '/new/product/workspace-crm-4k-v3.webp',
  alt: 'OrbitDesk CRM with Northstar Labs contact Maya Chen, an SSO rollout summary, a reviewed buying signal, customer activity, and linked company, deal, and project records.',
  width: 3840,
  height: 2160,
};

const KNOWLEDGE_IMAGE = {
  src: '/new/product/workspace-knowledge-4k-v3.webp',
  alt: 'OrbitDesk workspace and Help Center, showing eight documentation collections and guides for Okta SSO, role mapping, SCIM provisioning, agent approvals, and workspace permissions.',
  width: 3840,
  height: 2160,
};

const AREAS = [
  {
    id: 'inbox', label: 'Inbox',
    image: INBOX_IMAGE,
    title: 'Answer customers with agents that know their history.',
    description: 'Draft replies from past conversations and product knowledge. Review the answer, add a note, or turn the request into a task.',
  },
  {
    id: 'meetings', label: 'Meetings',
    image: MEETINGS_IMAGE,
    title: 'Turn meeting decisions into next steps.',
    description: 'Record and transcribe customer calls. Work with agents to summarize decisions and turn action items into tasks.',
  },
  {
    id: 'projects', label: 'Projects',
    image: PROJECTS_IMAGE,
    title: 'Turn customer requests into planned work.',
    description: 'Use agents to break requests into tasks and plan the next steps. Set priorities and owners, then connect the work to engineering.',
  },
  {
    id: 'crm', label: 'CRM',
    image: CRM_IMAGE,
    title: 'See what’s holding up the deal.',
    description: 'See the support issue, feature request, or meeting objection behind a deal. Ask agents to review the account and prepare your follow-up.',
  },
  {
    id: 'knowledge', label: 'Knowledge',
    image: KNOWLEDGE_IMAGE,
    title: 'Give customers and agents answers they can use.',
    description: 'Create help articles and internal docs alongside the work. Use agents to draft answers from repeated questions and update docs as the product changes.',
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
      <div className="px-copy"><h3>{area.title}</h3>{' '}<p>{area.description}</p><a href={area.id === 'crm' ? '/new/products/crm' : `/new/product#${area.id}`}>Explore {area.label.toLowerCase()} <ArrowUpRight size={15} aria-hidden="true" /></a></div>
      <div className="px-stage">
        <a className="px-product-image" href={area.image.src} target="_blank" rel="noopener noreferrer" aria-label={`View the ${area.label.toLowerCase()} demo image at full size (opens in a new tab)`}>
          <img
            src={area.image.src.replace('-4k-', '-1920-')}
            srcSet={area.id === 'meetings' ? '/new/meetings/recording-preview-960-v4.webp 960w, /new/meetings/recording-preview-1672-v4.webp 1672w' : `${area.image.src.replace('-4k-', '-960-')} 960w, ${area.image.src.replace('-4k-', '-1920-')} 1920w, ${area.image.src} 3840w`}
            sizes="(max-width: 760px) calc(100vw - 80px), (max-width: 960px) calc(100vw - 256px), (max-width: 1280px) calc(100vw - 316px), 964px"
            alt={area.image.alt} width={area.image.width} height={area.image.height} loading="lazy" decoding="async"
          />
        </a>
      </div>
    </div>)}
    </div>
  </div>;
}
