'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useReviewNotes } from './ReviewNotes';

// Hero concept: one wire, four stages, one customer. The line draws left to right,
// each stage hangs its artifact off the wire, and work inside the stages ticks off.
// Everything is in the DOM from the first paint; ticks only reveal and flip state.
const TICK_MS = 320;
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

// Stepped wire geometry. Stage labels alternate between an upper and a lower level and the
// line moves between them with an S-curve, entering and leaving at the viewport edges.
const GAP = 24;
const LEVEL_Y = [15, 65]; // centre line of the label pill on each level
const LEVEL_TOP = [0, 50]; // padding-top for nodes on each level
const levelOf = (i: number) => i % 2;

function wirePath(w: number, vw: number): string {
  if (!w || !vw) return '';
  const ox = Math.max(0, (vw - w) / 2);
  const cw = (w - GAP * 3) / 4;
  const y = (i: number) => LEVEL_Y[levelOf(i)];
  let d = `M ${-ox} ${y(0)} L ${ox} ${y(0)}`;
  for (let i = 0; i < 4; i += 1) {
    const colL = ox + i * (cw + GAP);
    const bendStart = colL + cw * 0.62;
    const next = i + 1;
    if (next < 4) {
      const bendEnd = ox + next * (cw + GAP) - 6;
      const mid = (bendStart + bendEnd) / 2;
      d += ` L ${bendStart} ${y(i)} C ${mid} ${y(i)} ${mid} ${y(next)} ${bendEnd} ${y(next)}`;
    } else {
      d += ` L ${vw + ox} ${y(i)}`;
    }
  }
  return d;
}

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
  const [size, setSize] = useState({ w: 0, vw: 0 });
  const [len, setLen] = useState(0);
  const wrapRef = useRef<HTMLDivElement>(null);
  const pathRef = useRef<SVGPathElement>(null);
  const { shown } = useReviewNotes();

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;
    const measure = () => setSize({ w: el.clientWidth, vw: window.innerWidth });
    measure();
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    window.addEventListener('resize', measure);
    return () => { ro.disconnect(); window.removeEventListener('resize', measure); };
  }, []);

  const d = wirePath(size.w, size.vw);
  useEffect(() => {
    if (pathRef.current && d) setLen(pathRef.current.getTotalLength());
  }, [d]);

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
    <div className="wire" ref={wrapRef} role="img" aria-label="One customer question moving through four stages: heard in chat and a meeting, decided into a task, shipped through an agent-opened pull request that a person approves and merges, and told back as a published article and a reply">
      <svg className="wire-svg" width={size.vw || 0} height={96} aria-hidden="true" style={{ left: -Math.max(0, (size.vw - size.w) / 2) }}>
        <path
          ref={pathRef}
          d={d}
          fill="none"
          stroke="currentColor"
          strokeWidth={1.5}
          style={len ? { strokeDasharray: len, strokeDashoffset: t >= 1 ? 0 : len, opacity: 1 } : { opacity: 0 }}
        />
      </svg>
      <div className="wire-nodes" aria-hidden="true">
        {/* 01 Hear: chat bubbles and a transcript line */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(0)] }}>
          <div className={`wlabel ${on(3) ?? ''}`}><i /><span>01 Hear</span></div>
          <div className="wart hear">
            <div className={`bub ${on(4) ?? ''}`}>
              <span className="bav">M</span>
              <div>
                <p>We're moving to Okta next month. Does SSO work with it?</p>
                <small>Maya R. · Acme Corp · website chat</small>
              </div>
            </div>
            <div className={`tline ${on(6) ?? ''}`}>
              <span className="mono tt">14:02</span>
              <div><b>Maya:</b> “…security wants the Okta mapping steps before we sign.”</div>
              <span className="wchip am">Timeline identified</span>
            </div>
          </div>
        </div>

        {/* 02 Decide: bare checklist of pills, then the task row */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(1)] }}>
          <div className={`wlabel ${on(4) ?? ''}`}><i /><span>02 Decide</span></div>
          <div className={`wart decide ${on(7) ?? ''}`}>
            <Checklist items={DECIDE} t={t} cardAt={7} />
            <div className={`trow ${on(12) ?? ''}`}>
              <span className="tkey">HLP-142</span>
              <span className="tname">Verify and document Okta SAML mapping</span>
              <span className="tstate"><i />Todo</span>
            </div>
          </div>
        </div>

        {/* 03 Ship: a dark run log */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(2)] }}>
          <div className={`wlabel ${on(5) ?? ''}`}><i /><span>03 Ship</span></div>
          <div className={`wart log ${on(13) ?? ''}`}>
            <div className="lhead"><span>agent run · HLP-142</span><span className="mono">HLP-142-okta-saml-mapping</span></div>
            <Checklist items={SHIP} t={t} cardAt={13} />
          </div>
        </div>

        {/* 04 Tell: document preview, a reply bubble, the deal */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(3)] }}>
          <div className={`wlabel ${on(6) ?? ''}`}><i /><span>04 Tell</span></div>
          <div className="wart tell">
            <div className={`docp ${on(26) ?? ''}`}>
              <div className="dtitle">Set up SSO with Okta</div>
              <div className="dline" style={{ width: '92%' }} /><div className="dline" style={{ width: '76%' }} /><div className="dline" style={{ width: '84%' }} />
              <div className="dfoot"><span>Help center · Security</span><span className="wchip em">Published</span></div>
            </div>
            <div className={`bub reply ${on(28) ?? ''}`}>
              <div>
                <p>Okta is verified and documented. Here are the exact steps.</p>
                <small>Sam · source: Set up SSO with Okta</small>
              </div>
              <span className="bav">S</span>
            </div>
            <div className={`deal ${on(30) ?? ''}`}><i /><b>Growth renewal · $48k</b><span>Renewal signed</span></div>
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
