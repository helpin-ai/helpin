'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useReviewNotes } from './ReviewNotes';

// Hero concept: one wire, four stages, one customer. The line draws left to right,
// each stage hangs its artifact off the wire, and work inside the stages ticks off.
// Everything is in the DOM from the first paint; ticks only reveal and flip state.
const TICK_MS = 290;

// Schedule. The dark line departs for a stage, arrives about four ticks later, and the
// stage's work begins once it has arrived. Two lines: a light track showing the whole
// route from the first paint, and a dark line that travels it, accelerating out of each
// stop and braking into the next.
const S = {
  depart1: 1, hear: 5, hearBubble: 6, hearLine: 8,
  depart2: 9, decide: 13, decideCard: 14, d1: 15, d2: 16, d3: 17, taskRow: 17,
  depart3: 18, ship: 22, shipCard: 23, s1: 24, s2: 25, approved: 28, s4: 29, s5: 30,
  depart4: 31, tell: 35, doc: 36, reply: 37, deal: 38,
  exit: 39,
};
const LAST = 41;

type Item = { at: number; label: string; done?: string };

const DECIDE: Item[] = [
  { at: S.d1, label: 'Signal: Timeline identified · High' },
  { at: S.d2, label: 'Next step accepted: confirm Okta before renewal' },
  { at: S.d3, label: 'HLP-142 created from the conversation' },
];

const SHIP: Item[] = [
  { at: S.s1, label: 'Task planner scoped the work' },
  { at: S.s2, label: 'Code builder opened PR #318' },
  { at: S.approved, label: 'Waiting for approval', done: 'Approved by Sam' },
  { at: S.s4, label: 'Review agent: checks passed' },
  { at: S.s5, label: 'Merged by Sam' },
];

// Stepped wire geometry. Stage labels alternate between an upper and a lower level and the
// line moves between them with an S-curve, entering and leaving at the viewport edges.
const GAP = 24;
const LEVEL_Y = [15, 65]; // y of the wire on each level; badges hang 12px beneath it
const LEVEL_TOP = [27, 77]; // padding-top for nodes on each level: line, then a 12px gap, then the badge
const levelOf = (i: number) => i % 2;

// x position of each stage label's left edge, in SVG coordinates (0 = viewport left).
function stageXs(w: number, vw: number): number[] {
  const ox = Math.max(0, (vw - w) / 2);
  const cw = (w - GAP * 3) / 4;
  return [0, 1, 2, 3].map((i) => ox + i * (cw + GAP) - 2);
}

// Path length at which the path reaches a given x. The wire is monotonic in x.
function lengthAtX(path: SVGPathElement, total: number, x: number): number {
  let lo = 0, hi = total;
  for (let k = 0; k < 24; k += 1) {
    const mid = (lo + hi) / 2;
    if (path.getPointAtLength(mid).x < x) lo = mid; else hi = mid;
  }
  return (lo + hi) / 2;
}

function wirePath(w: number, vw: number): string {
  if (!w || !vw) return '';
  const ox = Math.max(0, (vw - w) / 2);
  const cw = (w - GAP * 3) / 4;
  const y = (i: number) => LEVEL_Y[levelOf(i)];
  const r = 12; // corner radius of each step
  let d = `M ${-ox} ${y(0)}`;
  for (let i = 0; i < 4; i += 1) {
    const next = i + 1;
    if (next < 4) {
      // Step down or up just before the next stage: two small rounded corners and a short vertical.
      const xStep = ox + next * (cw + GAP) - 28;
      const y1 = y(i), y2 = y(next);
      const down = y2 > y1;
      const s1 = down ? 1 : 0; // sweep flags for a right-turning then left-turning corner
      const s2 = down ? 0 : 1;
      d += ` L ${xStep - r} ${y1}`;
      d += ` A ${r} ${r} 0 0 ${s1} ${xStep} ${y1 + (down ? r : -r)}`;
      d += ` L ${xStep} ${y2 - (down ? r : -r)}`;
      d += ` A ${r} ${r} 0 0 ${s2} ${xStep + r} ${y2}`;
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
  const [marks, setMarks] = useState<number[]>([]);
  const [reduced, setReduced] = useState(false);
  const wrapRef = useRef<HTMLDivElement>(null);
  const pathRef = useRef<SVGPathElement>(null);
  const prevTarget = useRef(0);
  const settle = useRef(true); // true right after a re-measure: jump, don't animate
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
    const path = pathRef.current;
    if (!path || !d) return;
    const total = path.getTotalLength();
    setLen(total);
    setMarks(stageXs(size.w, size.vw).map((x) => lengthAtX(path, total, x)));
    settle.current = true;
  }, [d, size.w, size.vw]);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setReduced(true); setT(LAST); return; }
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

  // Stage badges are always visible. A badge fills as the dark line closes in on it
  // (two ticks after departure, so the fill lands with the arrival), stays filled while
  // the stage runs, and settles to a done state when the line leaves for the next stage.
  const departs = [S.depart1, S.depart2, S.depart3, S.depart4, S.exit];
  const badge = (i: number) => {
    if (t >= departs[i + 1]) return 'wlabel done';
    if (t >= departs[i] + 2) return 'wlabel running';
    return 'wlabel';
  };

  // Where the dark line's head should be for the current tick.
  let target = 0;
  if (marks.length === 4 && len) {
    if (t >= S.exit) target = len;
    else if (t >= S.depart4) target = marks[3];
    else if (t >= S.depart3) target = marks[2];
    else if (t >= S.depart2) target = marks[1];
    else if (t >= S.depart1) target = marks[0];
  }
  // Constant-acceleration feel: ease-in-out, with duration growing with distance.
  const dist = Math.abs(target - prevTarget.current);
  const dur = settle.current || reduced ? 0 : Math.min(1.6, Math.max(0.6, 0.45 + dist / 480));
  const headStyle = len
    ? { strokeDasharray: len, strokeDashoffset: len - target, transition: dur ? `stroke-dashoffset ${dur}s cubic-bezier(.45,0,.55,1)` : 'none', opacity: 1 }
    : { opacity: 0 };
  useEffect(() => { prevTarget.current = target; settle.current = false; }, [target]);

  return (
    <div className="wire" ref={wrapRef} role="img" aria-label="One customer question moving through four stages: heard in chat and a meeting, decided into a task, shipped through an agent-opened pull request that a person approves and merges, and told back as a published article and a reply">
      <svg className="wire-svg" width={size.vw || 0} height={96} aria-hidden="true" style={{ left: -Math.max(0, (size.vw - size.w) / 2) }}>
        <path ref={pathRef} className="track" d={d} fill="none" strokeWidth={1.5} />
        <path className="head" d={d} fill="none" strokeWidth={1.5} strokeLinecap="round" style={headStyle} />
      </svg>
      <div className="wire-nodes" aria-hidden="true">
        {/* 01 Hear: chat bubbles and a transcript line */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(0)] }}>
          <div className={badge(0)}><i /><span>01 Hear</span></div>
          <div className="wart hear">
            <div className={`bub ${on(S.hearBubble) ?? ''}`}>
              <span className="bav">M</span>
              <div>
                <p>We're moving to Okta next month. Does SSO work with it?</p>
                <small>Maya R. · Acme Corp · website chat</small>
              </div>
            </div>
            <div className={`tline ${on(S.hearLine) ?? ''}`}>
              <span className="mono tt">14:02</span>
              <div><b>Maya:</b> “…security wants the Okta mapping steps before we sign.”</div>
              <span className="wchip am">Timeline identified</span>
            </div>
          </div>
        </div>

        {/* 02 Decide: bare checklist of pills, then the task row */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(1)] }}>
          <div className={badge(1)}><i /><span>02 Decide</span></div>
          <div className={`wart decide ${on(S.decideCard) ?? ''}`}>
            <Checklist items={DECIDE} t={t} cardAt={S.decideCard} />
            <div className={`trow ${on(S.taskRow) ?? ''}`}>
              <span className="tkey">HLP-142</span>
              <span className="tname">Verify and document Okta SAML mapping</span>
              <span className="tstate"><i />Todo</span>
            </div>
          </div>
        </div>

        {/* 03 Ship: a dark run log */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(2)] }}>
          <div className={badge(2)}><i /><span>03 Ship</span></div>
          <div className={`wart log ${on(S.shipCard) ?? ''}`}>
            <div className="lhead"><span>agent run · HLP-142</span><span className="mono">HLP-142-okta-saml-mapping</span></div>
            <Checklist items={SHIP} t={t} cardAt={S.shipCard} />
          </div>
        </div>

        {/* 04 Tell: document preview, a reply bubble, the deal */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(3)] }}>
          <div className={badge(3)}><i /><span>04 Tell</span></div>
          <div className="wart tell">
            <div className={`docp ${on(S.doc) ?? ''}`}>
              <div className="dtitle">Set up SSO with Okta</div>
              <div className="dline" style={{ width: '92%' }} /><div className="dline" style={{ width: '76%' }} /><div className="dline" style={{ width: '84%' }} />
              <div className="dfoot"><span>Help center · Security</span><span className="wchip em">Published</span></div>
            </div>
            <div className={`bub reply ${on(S.reply) ?? ''}`}>
              <div>
                <p>Okta is verified and documented. Here are the exact steps.</p>
                <small>Sam · source: Set up SSO with Okta</small>
              </div>
              <span className="bav">S</span>
            </div>
            <div className={`deal ${on(S.deal) ?? ''}`}><i /><b>Growth renewal · $48k</b><span>Renewal signed</span></div>
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
