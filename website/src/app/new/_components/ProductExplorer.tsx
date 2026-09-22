'use client';

import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { ArrowUpRight } from 'lucide-react';
import { ProductPreview, type ProductPreviewName } from './product-previews';

const AREAS = [
  {
    id: 'inbox', label: 'Support', href: '/new/products/customer-support',
    title: 'Answer the question without losing the story.',
    description: 'Handle chat and email in one inbox. Let agents use past conversations and product knowledge to answer, investigate, or hand off—with the findings and customer request attached.',
  },
  {
    id: 'meetings', label: 'Meetings', href: '/new/products/meetings',
    title: 'Keep the commitments, not just the recording.',
    description: 'Capture calls, review summaries, and turn decisions into tasks and follow-ups. Keep the transcript linked so the next step stays grounded in what was actually agreed.',
  },
  {
    id: 'projects', label: 'Projects', href: '/new/products/projects',
    title: 'Plan roadmaps. Run sprints. Track delivery.',
    description: 'Manage epics, tasks, dependencies, and objectives in one place. Let agents help break down the work while your team sets priorities, assigns owners, and keeps relevant customer requests attached.',
  },
  {
    id: 'crm', label: 'CRM', href: '/new/products/crm',
    title: 'See the relationship behind the deal.',
    description: 'Manage contacts, companies, and pipelines alongside their conversations and open work. Understand renewal concerns, spot buying signals, and prepare the next step with the supporting history in view.',
  },
  {
    id: 'knowledge', label: 'Knowledge', href: '/new/products/knowledge',
    title: 'Turn recurring questions into useful answers.',
    description: 'Publish help articles and internal docs. Use agents to draft guidance from recurring questions and prepare updates as your product changes—ready for your team to review.',
  },
] as const satisfies ReadonlyArray<{ id: ProductPreviewName; label: string; href: string; title: string; description: string }>;

export function ProductExplorer() {
  const [active, setActive] = useState(0);
  const [preloadPreviews, setPreloadPreviews] = useState(false);
  const [horizontal, setHorizontal] = useState(false);
  const [reducedMotion, setReducedMotion] = useState(true);
  const [scrollDriven, setScrollDriven] = useState(false);
  const track = useRef<HTMLDivElement>(null);
  const container = useRef<HTMLDivElement>(null);
  const tabList = useRef<HTMLDivElement>(null);
  const tabs = useRef<Array<HTMLButtonElement | null>>([]);
  const geometry = useRef({ start: 0, step: 1 });

  useEffect(() => {
    const element = track.current;
    if (!element) return;
    // Prepare all five scenes before the walkthrough enters view. Hidden demos
    // pause through useBentoPlayback and retain their state between selections.
    const observer = new IntersectionObserver(([entry]) => {
      if (!entry.isIntersecting) return;
      setPreloadPreviews(true);
      observer.disconnect();
    }, { rootMargin: '800px 0px' });
    observer.observe(element);
    return () => observer.disconnect();
  }, []);

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
      const preview = scene.querySelector<HTMLElement>('.px-panel:not([hidden]) .product-preview');
      // Fit the embedded demo beneath the copy before deciding whether the scene
      // can pin. Fixed-height demos otherwise disable the walkthrough on laptops.
      if (!reducedMotion && !horizontal && preview) {
        const chromeHeight = scene.offsetHeight - preview.offsetHeight;
        const available = window.innerHeight - top - 16 - chromeHeight;
        rail.style.setProperty('--px-preview-height', `${Math.max(320, Math.min(640, available))}px`);
      } else {
        rail.style.removeProperty('--px-preview-height');
      }
      const sceneHeight = scene.offsetHeight;
      // Keep manual tabs where a readable demo cannot fit, and for reduced motion.
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
    // The navigation hides while scrolling down. Reclaim its reserved space,
    // then restore the clearance when scrolling back up brings it into view.
    const navigation = document.querySelector('.hp3 .pnav');
    const navigationObserver = new MutationObserver(measure);
    if (navigation) navigationObserver.observe(navigation, { attributes: true, attributeFilter: ['data-hidden'] });
    window.addEventListener('scroll', schedule, { passive: true });
    window.addEventListener('resize', measure);
    return () => {
      resize.disconnect();
      navigationObserver.disconnect();
      window.removeEventListener('scroll', schedule);
      window.removeEventListener('resize', measure);
      window.cancelAnimationFrame(frame);
      rail.style.removeProperty('height');
      rail.style.removeProperty('--px-preview-height');
    };
  }, [horizontal, reducedMotion]);

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
      <div className="px-copy"><h3>{area.title}</h3>{' '}<p>{area.description}</p><a href={area.href}>Explore {area.id === 'inbox' ? 'customer support' : area.id === 'crm' ? 'CRM' : area.label.toLowerCase()} <ArrowUpRight size={15} aria-hidden="true" /></a></div>
      <div className="px-stage">
        {(active === index || preloadPreviews) && <ProductPreview product={area.id} theme="light" />}
      </div>
    </div>)}
    </div>
  </div>;
}
