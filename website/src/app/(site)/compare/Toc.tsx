'use client';

import { useEffect, useState } from 'react';

type Item = { id: string; label: string };

// "On this page" navigation that follows the reader. Links work without JavaScript;
// the observer only adds the current-section highlight.
export function Toc({ items }: { items: Item[] }) {
  const [active, setActive] = useState(items[0]?.id);

  useEffect(() => {
    if (typeof IntersectionObserver === 'undefined') return;
    const sections = items.map(item => document.getElementById(item.id)).filter((node): node is HTMLElement => Boolean(node));
    const observer = new IntersectionObserver(entries => {
      const inView = entries.filter(entry => entry.isIntersecting).sort((a, b) => a.boundingClientRect.top - b.boundingClientRect.top);
      if (inView[0]) setActive(inView[0].target.id);
    }, { rootMargin: '-90px 0px -65% 0px' });
    sections.forEach(section => observer.observe(section));
    return () => observer.disconnect();
  }, [items]);

  return (
    <nav className="cmp-toc" aria-label="On this page">
      <span className="cmp-toc-title">On this page</span>
      <ol>
        {items.map((item, index) => (
          <li key={item.id}>
            <a href={`#${item.id}`} aria-current={active === item.id ? 'location' : undefined}>
              <span>{String(index + 1).padStart(2, '0')}</span>{item.label}
            </a>
          </li>
        ))}
      </ol>
    </nav>
  );
}
