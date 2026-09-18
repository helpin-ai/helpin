'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useReviewNotes } from './ReviewNotes';

// Hero concept: one wire, four stages, one customer. The line draws left to right,
// each stage hangs its artifact off the wire, and work inside the stages ticks off.
// Everything is in the DOM from the first paint; ticks only reveal and flip state.
const TICK_MS = 215;

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

type Item = { at: number; label: string; done?: string; avatar?: string };

const AVATAR: Record<string, string> = { M: 'maya', D: 'dev', L: 'lin', A: 'aisha', S: 'sam', P: 'priya' };
function Avatar({ initial, size = 24 }: { initial: string; size?: number }) {
  const file = AVATAR[initial];
  return file
    ? <img className="bav" src={`/new/avatars/${file}.webp`} width={size} height={size} alt="" loading="lazy" decoding="async" />
    : <span className="bav">{initial}</span>;
}

type Scenario = {
  caption: string;
  hear: { initial: string; text: string; meta: string; time: string; speaker: string; quote: string; chip: string };
  decide: { items: [string, string, string]; key: string; title: string };
  ship: { branch: string; items: [string, string, string, string]; approver: string };
  tell: { doc: string; section: string; reply: string; replyMeta: string; initial: string; deal: string; outcome: string };
};

// Four kinds of request, one loop each. Names and companies are examples for the demo workspace.
const SCENARIOS: Scenario[] = [
  {
    caption: 'Acme Inc. · SSO migration',
    hear: { initial: 'M', text: 'Does SSO work with Okta? We move next month.', meta: 'Maya R. · Acme Inc. · chat',
      time: '14:02', speaker: 'Maya', quote: 'Security needs the Okta steps first.', chip: 'Timeline identified' },
    decide: { items: ['Timeline identified · High', 'Next step: confirm Okta support', 'HLP-142 created'],
      key: 'HLP-142', title: 'Okta SAML mapping' },
    ship: { branch: 'HLP-142-okta-saml', items: ['Task planner scoped it', 'Code builder opened PR #482', 'Checks passed', 'Merged by Sam'], approver: 'Sam' },
    tell: { doc: 'Set up SSO with Okta', section: 'Help center', reply: 'Okta is documented. Steps attached.', replyMeta: 'Sam · source linked', initial: 'S',
      deal: 'Enterprise renewal · $42k', outcome: 'Signed' },
  },
  {
    caption: 'Nimbus · Billing details',
    hear: { initial: 'D', text: 'Invoices still show our old company name.', meta: 'Dev P. · Nimbus · email',
      time: '09:41', speaker: 'Dev', quote: 'Every invoice since June is wrong.', chip: 'Relationship risk' },
    decide: { items: ['Relationship risk · Medium', 'Next step: reissue August invoices', 'HLP-151 created'],
      key: 'HLP-151', title: 'Editable billing name' },
    ship: { branch: 'HLP-151-billing-name', items: ['Task planner scoped it', 'Code builder opened PR #322', 'Checks passed', 'Merged by Priya'], approver: 'Priya' },
    tell: { doc: 'Update your billing details', section: 'Help center', reply: 'You can edit the billing name now. Invoices reissued.', replyMeta: 'Priya · source linked', initial: 'P',
      deal: 'Starter · 14 seats', outcome: 'Retained' },
  },
  {
    caption: 'Orbit Labs · Export bug',
    hear: { initial: 'L', text: 'CSV export times out on our big workspace.', meta: 'Lin Z. · Orbit Labs · chat',
      time: '16:20', speaker: 'Lin', quote: 'Three analysts are blocked.', chip: 'Champion identified' },
    decide: { items: ['Coverage gap · 3 reports', 'Next step: prioritise the fix', 'HLP-158 created'],
      key: 'HLP-158', title: 'Export timeout' },
    ship: { branch: 'HLP-158-export-timeout', items: ['Task planner scoped it', 'Code builder opened PR #330', 'Checks passed', 'Merged by Sam'], approver: 'Sam' },
    tell: { doc: 'Exporting large workspaces', section: 'Help center', reply: 'Fixed. Large exports now run in the background.', replyMeta: 'Sam · source linked', initial: 'S',
      deal: 'Growth · expansion', outcome: '+20 seats' },
  },
  {
    caption: 'Fieldline · Feature request',
    hear: { initial: 'A', text: 'Can we get Slack alerts for failed payments?', meta: 'Aisha K. · Fieldline · chat',
      time: '11:15', speaker: 'Aisha', quote: 'With alerts we would move the whole team.', chip: 'Buying intent' },
    decide: { items: ['Buying intent · High', 'Next step: add to Q4 roadmap', 'HLP-160 created'],
      key: 'HLP-160', title: 'Slack payment alerts' },
    ship: { branch: 'HLP-160-slack-alerts', items: ['Task planner scoped it', 'Code builder opened PR #341', 'Checks passed', 'Merged by Priya'], approver: 'Priya' },
    tell: { doc: 'Set up Slack alerts', section: 'Help center', reply: 'Shipped. Pick a channel under Settings.', replyMeta: 'Priya · source linked', initial: 'P',
      deal: 'New deal · $24k', outcome: 'Closed won' },
  },
];

const HOLD_TICKS = 8; // rest on the finished state before the next example

function decideItems(sc: Scenario): Item[] {
  return [{ at: S.d1, label: sc.decide.items[0] }, { at: S.d2, label: sc.decide.items[1] }, { at: S.d3, label: sc.decide.items[2] }];
}
function shipItems(sc: Scenario): Item[] {
  return [
    { at: S.s1, label: sc.ship.items[0] },
    { at: S.s2, label: sc.ship.items[1] },
    { at: S.approved, label: 'Waiting for approval', done: `Approved by ${sc.ship.approver}`, avatar: sc.ship.approver[0] },
    { at: S.s4, label: sc.ship.items[2] },
    { at: S.s5, label: sc.ship.items[3] },
  ];
}

// Stepped wire geometry. Stage labels alternate between an upper and a lower level and the
// line moves between them with an S-curve, entering and leaving at the viewport edges.
const GAP = 40;
const LEVEL_Y = [15, 100]; // y of the wire on each level; badges sit centred on it
const LEVEL_TOP = [0, 85]; // padding-top for nodes on each level so the 30px badge is centred on the wire
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
  if (state === 'done') {
    return (
      <i className="ck done" aria-hidden="true">
        <svg viewBox="0 0 12 12" width="9" height="9" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round"><path d="M2.5 6.5 5 9l4.5-6" /></svg>
      </i>
    );
  }
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
            {done && it.done && it.avatar ? <Avatar initial={it.avatar} size={16} /> : null}
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
  const [idx, setIdx] = useState(0);
  const [inView, setInView] = useState(true);
  const [mounted, setMounted] = useState(false);
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
    const id = window.setTimeout(() => setMounted(true), 120);
    return () => window.clearTimeout(id);
  }, []);

  useEffect(() => {
    const el = wrapRef.current;
    if (!el || typeof IntersectionObserver === 'undefined') return;
    const io = new IntersectionObserver((entries) => setInView(entries.some((e) => e.isIntersecting)), { threshold: 0.2 });
    io.observe(el);
    return () => io.disconnect();
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (window.matchMedia('(prefers-reduced-motion: reduce)').matches) { setReduced(true); setT(LAST); return; }
    if (!inView) return;
    let cur = 0;
    setT(0);
    const id = window.setInterval(() => {
      cur += 1;
      if (cur <= LAST) { setT(cur); return; }
      if (cur >= LAST + HOLD_TICKS) {
        // Next example: reset the line without animating it backwards, then start over.
        settle.current = true;
        cur = 0;
        setIdx((i) => (i + 1) % SCENARIOS.length);
        setT(0);
      }
    }, TICK_MS);
    return () => window.clearInterval(id);
  }, [run, inView]);

  const sc = SCENARIOS[idx];

  const replay = useCallback(() => { settle.current = true; setRun((r) => r + 1); }, []);
  const on = (at: number) => (t >= at ? 'on' : undefined);

  // Stage badges are always visible. A badge fills as the dark line closes in on it
  // (two ticks after departure, so the fill lands with the arrival), stays filled while
  // the stage runs, and settles to a done state when the line leaves for the next stage.
  const departs = [S.depart1, S.depart2, S.depart3, S.depart4, S.exit];
  const badge = (i: number) => {
    const base = mounted ? 'wlabel in' : 'wlabel';
    if (t >= departs[i + 1]) return `${base} done`;
    if (t >= departs[i] + 2) return `${base} running`;
    return base;
  };
  const badgeDelay = (i: number) => ({ transitionDelay: mounted ? `${i * 110}ms, 0ms, 0ms, 0ms` : '0ms' });

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
  const dur = settle.current || reduced ? 0 : Math.min(1.1, Math.max(0.45, 0.3 + dist / 720));
  const headStyle = len
    ? { strokeDasharray: len, strokeDashoffset: len - target, transition: dur ? `stroke-dashoffset ${dur}s cubic-bezier(.45,0,.55,1)` : 'none', opacity: 1 }
    : { opacity: 0 };
  useEffect(() => { prevTarget.current = target; settle.current = false; }, [target]);

  return (
    <div className="wire" ref={wrapRef} role="img" aria-label="One customer question moving through four stages: heard in chat and a meeting, decided into a task, shipped through an agent-opened pull request that a person approves and merges, and told back as a published article and a reply">
      <svg className="wire-svg" width={size.vw || 0} height={132} aria-hidden="true" style={{ left: -Math.max(0, (size.vw - size.w) / 2) }}>
        <path ref={pathRef} className="track" d={d} fill="none" strokeWidth={1.5} />
        <path className="head" d={d} fill="none" strokeWidth={1.5} strokeLinecap="round" style={headStyle} />
      </svg>
      <div className="wire-nodes" aria-hidden="true">
        {/* 01 Hear: chat bubbles and a transcript line */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(0)] }}>
          <div className={badge(0)} style={badgeDelay(0)}><i /><span>01 Hear</span></div>
          <div className="wart hear">
            <div className={`bub ${on(S.hearBubble) ?? ''}`}>
              <Avatar initial={sc.hear.initial} />
              <div>
                <p>{sc.hear.text}</p>
                <small>{sc.hear.meta}</small>
              </div>
            </div>
            <div className={`tline ${on(S.hearLine) ?? ''}`}>
              <span className="mono tt">{sc.hear.time}</span>
              <div><b>{sc.hear.speaker}:</b> “{sc.hear.quote}”</div>
              <span className="wchip am">{sc.hear.chip}</span>
            </div>
          </div>
        </div>

        {/* 02 Decide: bare checklist of pills, then the task row */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(1)] }}>
          <div className={badge(1)} style={badgeDelay(1)}><i /><span>02 Decide</span></div>
          <div className={`wart decide ${on(S.decideCard) ?? ''}`}>
            <Checklist items={decideItems(sc)} t={t} cardAt={S.decideCard} />
            <div className={`trow ${on(S.taskRow) ?? ''}`}>
              <span className="tkey">{sc.decide.key}</span>
              <span className="tname">{sc.decide.title}</span>
              <span className="tstate"><i />Todo</span>
            </div>
          </div>
        </div>

        {/* 03 Ship: a dark run log */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(2)] }}>
          <div className={badge(2)} style={badgeDelay(2)}><i /><span>03 Ship</span></div>
          <div className={`wart log ${on(S.shipCard) ?? ''}`}>
            <div className="lhead"><span>run · {sc.decide.key}</span><span className="mono">{sc.ship.branch}</span></div>
            <Checklist items={shipItems(sc)} t={t} cardAt={S.shipCard} />
          </div>
        </div>

        {/* 04 Tell: document preview, a reply bubble, the deal */}
        <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(3)] }}>
          <div className={badge(3)} style={badgeDelay(3)}><i /><span>04 Tell</span></div>
          <div className="wart tell">
            <div className={`docp ${on(S.doc) ?? ''}`}>
              <div className="dtitle">{sc.tell.doc}</div>
              <div className="dline" style={{ width: '92%' }} /><div className="dline" style={{ width: '76%' }} /><div className="dline" style={{ width: '84%' }} />
              <div className="dfoot"><span>{sc.tell.section}</span><span className="wchip em">Published</span></div>
            </div>
            <div className={`bub reply ${on(S.reply) ?? ''}`}>
              <div>
                <p>{sc.tell.reply}</p>
                <small>{sc.tell.replyMeta}</small>
              </div>
              <Avatar initial={sc.tell.initial} />
            </div>
            <div className={`deal ${on(S.deal) ?? ''}`}><i /><b>{sc.tell.deal}</b><span>{sc.tell.outcome}</span></div>
          </div>
        </div>
      </div>
      <div className="wcaption" aria-hidden="true">
        <span className="wdots">{SCENARIOS.map((x, i) => <i key={x.caption} className={i === idx ? 'on' : undefined} />)}</span>
        <span>{sc.caption}</span>
      </div>
      {shown ? (
        <div className="replay-row">
          <button type="button" className="replay" onClick={replay}>Replay this example</button>
        </div>
      ) : null}
    </div>
  );
}
