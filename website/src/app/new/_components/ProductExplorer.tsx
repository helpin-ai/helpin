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
  const [active, setActive] = useState(2);
  const [horizontal, setHorizontal] = useState(false);
  const tabs = useRef<Array<HTMLButtonElement | null>>([]);

  useEffect(() => {
    const media = window.matchMedia('(max-width: 760px)');
    const update = () => setHorizontal(media.matches);
    update();
    media.addEventListener('change', update);
    return () => media.removeEventListener('change', update);
  }, []);

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
    setActive(next);
    tabs.current[next]?.focus({ preventScroll: true });
    tabs.current[next]?.scrollIntoView({ block: 'nearest', inline: 'nearest', behavior: 'instant' });
  }

  return <div className="product-explorer">
    <div className="px-navigation">
      <div className="px-tabs" role="tablist" aria-label="Product areas" aria-orientation={horizontal ? 'horizontal' : 'vertical'}>
        {AREAS.map(({ id, label }, index) => <button key={id} ref={node => { tabs.current[index] = node; }} type="button" role="tab" id={`product-tab-${id}`} aria-controls={`product-panel-${id}`} aria-selected={active === index} tabIndex={active === index ? 0 : -1} onClick={() => setActive(index)} onKeyDown={event => onKeyDown(event, index)}>{label}</button>)}
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
  </div>;
}
