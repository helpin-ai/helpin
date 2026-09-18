'use client';

import { useEffect, useRef, useState } from 'react';

// Section two visual: one customer's week on one record. Entries appear in order
// once the panel scrolls into view; reduced motion shows the finished timeline.
const ENTRIES = [
  { when: '09:14', title: 'Support conversation', sub: '“Export jobs are timing out.”', kind: 'support' },
  { when: '09:22', title: 'AI identified issue', sub: 'Possible regression · 4 similar reports', kind: 'ai' },
  { when: '10:05', title: 'Task created', sub: 'Fix CSV export timeout', kind: 'task' },
  { when: 'Wed', title: 'Product meeting', sub: 'Mentioned in roadmap review · Priority increased', kind: 'meeting' },
  { when: 'Thu', title: 'PR #482 merged', sub: 'CSV export timeout fixed', kind: 'pr' },
  { when: 'Fri', title: 'Customer follow-up', sub: '“Your export issue has been resolved.”', kind: 'reply' },
];

const STEP_MS = 650;

export function RecordStory() {
  const ref = useRef<HTMLDivElement>(null);
  const [n, setN] = useState(0);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setN(ENTRIES.length); return; }
    let timer: number | undefined;
    const start = () => {
      let cur = 0;
      timer = window.setInterval(() => {
        cur += 1;
        setN(cur);
        if (cur >= ENTRIES.length) window.clearInterval(timer);
      }, STEP_MS);
    };
    if (typeof IntersectionObserver === 'undefined') { start(); return; }
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) { start(); io.disconnect(); }
    }, { threshold: 0.4 });
    io.observe(el);
    return () => { io.disconnect(); if (timer) window.clearInterval(timer); };
  }, []);

  return (
    <div className="rs-panel" ref={ref} role="img" aria-label="Acme Inc. customer record: a support conversation on Tuesday morning becomes a task, is raised in Wednesday's product meeting, is fixed by a merged pull request on Thursday, and the customer is told on Friday">
      <div className="rs-head">
        <div className="rs-av">A</div>
        <div>
          <b>Acme Inc.</b>
          <span>Enterprise · $42k ARR · Renewal in 63 days</span>
        </div>
      </div>
      <ol className="rs-tl" aria-hidden="true">
        {ENTRIES.map((e, i) => (
          <li key={e.title} className={`${e.kind} ${i < n ? 'on' : ''}`}>
            <span className="rs-when">{e.when}</span>
            <span className="rs-dot" />
            <span className="rs-body"><b>{e.title}</b><span>{e.sub}</span></span>
          </li>
        ))}
      </ol>
    </div>
  );
}
