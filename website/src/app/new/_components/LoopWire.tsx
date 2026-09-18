'use client';

import { useCallback, useEffect, useState } from 'react';
import { useReviewNotes } from './ReviewNotes';

// Hero concept: one wire, four stages, one customer. The line draws left to right,
// each stage hangs its artifact off the wire, and work inside the stages ticks off.
// Everything is in the DOM from the first paint; ticks only reveal and flip state.
const TICK_MS = 350;
const LAST = 30;

type Item = { at: number; label: string; done?: string };

const DECIDE: Item[] = [
  { at: 8, label: 'Signal: Timeline identified · High' },
  { at: 10, label: 'Next step accepted: confirm Okta before renewal' },
  { at: 12, label: 'HLP-142 created from the conversation' },
];

const SHIP: Item[] = [
  { at: 14, label: 'Task planner scoped the work' },
  { at: 16, label: 'Code builder opened PR #318' },
  { at: 20, label: 'Waiting for approval', done: 'Approved by Sam' },
  { at: 22, label: 'Review agent: checks passed' },
  { at: 24, label: 'Merged by Sam' },
];

function Check({ state }: { state: 'todo' | 'active' | 'done' }) {
  if (state === 'done') return <i className="ck done" aria-hidden="true">✓</i>;
  if (state === 'active') return <i className="ck active" aria-hidden="true" />;
  return <i className="ck" aria-hidden="true" />;
}

function Checklist({ items, t, cardAt }: { items: Item[]; t: number; cardAt: number }) {
  const shown = t >= cardAt;
  let activeAssigned = false;
  return (
    <ul className="wl">
      {items.map((it) => {
        const done = t >= it.at;
        let state: 'todo' | 'active' | 'done' = done ? 'done' : 'todo';
        if (!done && shown && !activeAssigned) { state = 'active'; activeAssigned = true; }
        const isApproval = it.done !== undefined;
        return (
          <li key={it.label} className={state}>
            <Check state={state} />
            <span>{done && it.done ? it.done : it.label}</span>
            {isApproval && state === 'active' ? <em className="approve">Approve</em> : null}
          </li>
        );
      })}
    </ul>
  );
}

export function LoopWire() {
  const [t, setT] = useState(0);
  const [run, setRun] = useState(0);
  const { shown } = useReviewNotes();

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setT(LAST); return; }
    let cur = 0;
    setT(0);
    const id = window.setInterval(() => {
      cur += 1;
      setT(cur);
      if (cur >= LAST) window.clearInterval(id);
    }, TICK_MS);
    return () => window.clearInterval(id);
  }, [run]);

  const replay = useCallback(() => setRun((r) => r + 1), []);
  const on = (at: number) => (t >= at ? 'on' : undefined);

  return (
    <div className="wire" aria-label="One customer question moving through Hear, Decide, Ship, and Tell">
      <div className={`wire-line ${t >= 1 ? 'drawn' : ''}`} aria-hidden="true" />
      <div className="wire-nodes">
        {/* 01 Hear */}
        <div className="wnode">
          <div className={`wlabel ${on(2) ?? ''}`}><i /><span>01 Hear</span></div>
          <div className={`wcard ${on(3) ?? ''}`}>
            <div className={`witem ${on(3) ?? ''}`}>
              <div className="wq">“We're moving to Okta next month. Does SSO work with it?”</div>
              <div className="wmeta">Maya R. · Acme Corp · website chat</div>
            </div>
            <div className={`witem ${on(5) ?? ''}`}>
              <div className="wq"><b>Renewal call</b> · next step: send Okta SAML mapping steps</div>
              <div className="wmeta"><span className="wchip am">Timeline identified</span> from the transcript</div>
            </div>
          </div>
        </div>

        {/* 02 Decide */}
        <div className="wnode">
          <div className={`wlabel ${on(3) ?? ''}`}><i /><span>02 Decide</span></div>
          <div className={`wcard ${on(7) ?? ''}`}>
            <Checklist items={DECIDE} t={t} cardAt={7} />
            <div className={`wmeta witem ${on(12) ?? ''}`}>Task linked to the chat, the meeting, and the deal</div>
          </div>
        </div>

        {/* 03 Ship */}
        <div className="wnode">
          <div className={`wlabel ${on(4) ?? ''}`}><i /><span>03 Ship</span></div>
          <div className={`wcard ${on(13) ?? ''}`}>
            <div className="wmeta" style={{ marginBottom: 6 }}>HLP-142 · Run agent · <span className="mono">HLP-142-okta-saml-mapping</span></div>
            <Checklist items={SHIP} t={t} cardAt={13} />
          </div>
        </div>

        {/* 04 Tell */}
        <div className="wnode">
          <div className={`wlabel ${on(5) ?? ''}`}><i /><span>04 Tell</span></div>
          <div className={`wcard ${on(25) ?? ''}`}>
            <div className={`witem wdoc ${on(26) ?? ''}`}>
              <b>Set up SSO with Okta</b>
              <div className="wmeta">Help center · drafted from the coverage gap · <span className="wchip em">Published</span></div>
            </div>
            <div className={`witem ${on(28) ?? ''}`}>
              <div className="wq">“Okta is verified and documented. Here are the exact steps.”</div>
              <div className="wmeta">Reply to Maya · source: Set up SSO with Okta</div>
            </div>
            <div className={`witem ${on(30) ?? ''}`}>
              <span className="wchip em">Renewal signed · $48k</span>
            </div>
          </div>
        </div>
      </div>
      {shown ? (
        <div className="replay-row">
          <button type="button" className="replay" onClick={replay}>Replay animation</button>
          <span>tick {t} of {LAST}</span>
        </div>
      ) : null}
    </div>
  );
}
