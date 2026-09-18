'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useReviewNotes } from './ReviewNotes';

// Section two visual. At rest, three tool fragments hold three versions of the same
// customer: a support thread, a roadmap row, a CRM card. When the section enters view
// they travel into the record and the week draws down the timeline. One authored moment,
// transforms and opacity only, plays once. Reduced motion shows the finished record.
const ENTRIES = [
  { when: 'Mon', title: 'Renewal call', sub: '“Security needs the Okta steps before we sign.”', kind: 'meeting' },
  { when: '10:42', title: 'Support conversation', sub: '“Does SSO work with Okta? We move next month.”', kind: 'support' },
  { when: '10:45', title: 'Signal identified', sub: 'Timeline identified · High · Okta before renewal', kind: 'ai' },
  { when: '11:05', title: 'Task created', sub: 'HLP-142 · Okta SAML mapping', kind: 'task' },
  { when: 'Wed', title: 'Product meeting', sub: 'Raised in roadmap review · priority increased', kind: 'meeting' },
  { when: 'Thu', title: 'PR #318 merged', sub: 'Okta SAML attribute mapping', kind: 'pr' },
  { when: 'Fri', title: 'Customer follow-up', sub: '“Okta is verified and documented.” · Renewal signed', kind: 'reply' },
];

const MERGE_MS = 800;
const STEP_MS = 520;

export function RecordStory() {
  const ref = useRef<HTMLDivElement>(null);
  const [phase, setPhase] = useState<'rest' | 'merge' | 'play'>('rest');
  const [n, setN] = useState(0);
  const [run, setRun] = useState(0);
  const { shown } = useReviewNotes();

  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setPhase('play'); setN(ENTRIES.length); return; }
    setPhase('rest'); setN(0);
    const timers: number[] = [];
    const start = () => {
      setPhase('merge');
      timers.push(window.setTimeout(() => {
        setPhase('play');
        let cur = 0;
        const id = window.setInterval(() => {
          cur += 1; setN(cur);
          if (cur >= ENTRIES.length) window.clearInterval(id);
        }, STEP_MS);
        timers.push(id);
      }, MERGE_MS));
    };
    if (typeof IntersectionObserver === 'undefined') { start(); return; }
    const io = new IntersectionObserver((entries) => {
      if (entries.some((e) => e.isIntersecting)) { start(); io.disconnect(); }
    }, { threshold: 0.45 });
    io.observe(el);
    return () => { io.disconnect(); timers.forEach((t) => { window.clearTimeout(t); window.clearInterval(t); }); };
  }, [run]);

  const replay = useCallback(() => setRun((r) => r + 1), []);
  const progress = n / ENTRIES.length;

  return (
    <div className={`rs-stage ${phase}`} ref={ref}>
      {/* Three versions of the same story, before Helpin. */}
      <div className="rs-frag rs-frag-a" aria-hidden="true">
        <span className="rs-frag-l">Support inbox</span>
        <b>Acme · Does SSO work with Okta?</b>
        <span>Open · assigned to Sam</span>
      </div>
      <div className="rs-frag rs-frag-b" aria-hidden="true">
        <span className="rs-frag-l">Roadmap</span>
        <b>Okta SAML mapping</b>
        <span>Backlog · no customer linked</span>
      </div>
      <div className="rs-frag rs-frag-c" aria-hidden="true">
        <span className="rs-frag-l">CRM</span>
        <b>Acme Corp · $48k ARR</b>
        <span>Renewal in 22 days · last touch 3 weeks ago</span>
      </div>

      <div className="rs-panel" role="img" aria-label="Acme Corp customer record: a renewal call on Monday, the Okta question on Tuesday morning that becomes a signal and task HLP-142, the Wednesday product meeting, PR #318 merged on Thursday, and the customer told on Friday with the renewal signed">
        <div className="rs-head">
          <div className="rs-av">A</div>
          <div>
            <b>Acme Corp</b>
            <span>Growth · $48k ARR · Renewal in 22 days</span>
          </div>
          <span className="rs-live">One record</span>
        </div>
        <ol className="rs-tl" aria-hidden="true" style={{ ['--rs-p' as string]: progress }}>
          {ENTRIES.map((e, i) => (
            <li key={e.title} className={`${e.kind} ${i < n ? 'on' : ''}`}>
              <span className="rs-when">{e.when}</span>
              <span className="rs-dot" />
              <span className="rs-body"><b>{e.title}</b><span>{e.sub}</span></span>
            </li>
          ))}
        </ol>
      </div>
      {shown ? (
        <div className="replay-row rs-replay">
          <button type="button" className="replay" onClick={replay}>Replay this section</button>
        </div>
      ) : null}
    </div>
  );
}
