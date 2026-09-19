'use client';

import { useEffect, useRef, useState } from 'react';
import { Pause, Play, Link2, Check as CheckIcon } from 'lucide-react';
import { useBentoPlayback } from './useBentoPlayback';

// One illustrative OrbitDesk workflow. Static rendering shows the complete story;
// playback restores progressive reveals as the wire reaches each stage.
const TICK_MS = 100;
const LAST = 120;
const QUESTION = 'Our CSV export times out on larger reports.';
const S = { decide: 30, ship: 55, reviewed: 83, released: 91, tell: 95, sent: 110 };

function Avatar({ name }: { name: 'maya' | 'sam' }) {
  return <img className="bav" src={`/new/avatars/${name}.webp`} width={24} height={24} alt="" />;
}

// Stepped wire geometry. Stage labels alternate between an upper and a lower level and the
// line moves between them with an S-curve, entering and leaving at the viewport edges.
const LEVEL_Y = [15, 100]; // y of the wire on each level; badges sit centred on it
const LEVEL_TOP = [0, 85]; // padding-top for nodes on each level so the 30px badge is centred on the wire
const levelOf = (i: number) => i % 2;

// x position of each stage label's left edge, in SVG coordinates (0 = viewport left).
function stageXs(w: number, vw: number, gap: number): number[] {
  const ox = Math.max(0, (vw - w) / 2);
  const cw = (w - gap * 3) / 4;
  return [0, 1, 2, 3].map((i) => ox + i * (cw + gap) - 2);
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

function wirePath(w: number, vw: number, gap: number): string {
  if (!w || !vw) return '';
  const ox = Math.max(0, (vw - w) / 2);
  const cw = (w - gap * 3) / 4;
  const y = (i: number) => LEVEL_Y[levelOf(i)];
  const r = 12; // corner radius of each step
  let d = `M ${-ox} ${y(0)}`;
  for (let i = 0; i < 4; i += 1) {
    const next = i + 1;
    if (next < 4) {
      // Step down or up just before the next stage: two small rounded corners and a short vertical.
      const xStep = ox + next * (cw + gap) - 28;
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

function WorkStep({ label, at, start, t, review = false }: { label: string; at: number; start: number; t: number; review?: boolean }) {
  const done = t >= at;
  const state = done ? 'done' : t >= start ? 'active' : 'todo';
  return (
    <li className={`${state}${review ? ' wire-review' : ''}`}>
      <i className={`ck ${state}`}><CheckIcon size={9} strokeWidth={2} /></i>
      {review ? <Avatar name="sam" /> : null}
      <span>{review && !done ? 'Waiting for review' : label}</span>
    </li>
  );
}

export function LoopWire() {
  const { container, playing: automatic, cycle } = useBentoPlayback(15000);
  const [paused, setPaused] = useState(false);
  const [tick, setTick] = useState(0);
  const playing = automatic && !paused;
  const t = playing ? tick : LAST;
  const [size, setSize] = useState({ w: 0, vw: 0, gap: 40 });
  const [len, setLen] = useState(0);
  const [marks, setMarks] = useState<number[]>([]);
  const pathRef = useRef<SVGPathElement>(null);

  useEffect(() => {
    const el = container.current;
    if (!el) return;
    const measure = () => {
      const nodes = el.querySelector('.wire-nodes');
      const gap = nodes ? parseFloat(getComputedStyle(nodes).columnGap) : 40;
      setSize({ w: el.clientWidth, vw: window.innerWidth, gap });
    };
    measure();
    const observer = new ResizeObserver(measure);
    observer.observe(el);
    window.addEventListener('resize', measure);
    return () => { observer.disconnect(); window.removeEventListener('resize', measure); };
  }, [container]);

  const d = wirePath(size.w, size.vw, size.gap);
  useEffect(() => {
    const path = pathRef.current;
    if (!path || !d) return;
    const total = path.getTotalLength();
    setLen(total);
    setMarks(stageXs(size.w, size.vw, size.gap).map(x => lengthAtX(path, total, x)));
  }, [d, size.w, size.vw, size.gap]);

  useEffect(() => {
    if (!playing) return;
    setTick(0);
    let current = 0;
    const timer = window.setInterval(() => {
      current += 1;
      setTick(current);
      if (current >= LAST) window.clearInterval(timer);
    }, TICK_MS);
    return () => window.clearInterval(timer);
  }, [playing, cycle]);

  const stage = t < S.decide ? 0 : t < S.ship ? 1 : t < S.tell ? 2 : 3;
  const badge = (index: number) => `wlabel in ${t >= LAST || index < stage ? 'done' : index === stage ? 'running' : ''}`;
  const target = t >= LAST ? len : marks[stage] || 0;
  const typing = playing && t >= 5 && t < 15;
  const typedQuestion = typing ? QUESTION.slice(0, Math.max(1, Math.floor(QUESTION.length * (t - 5) / 10))) : QUESTION;

  return (
    <div className="wire wire-story" ref={container} data-playing={playing} data-step={stage}>
      <button className="wire-playback" type="button" onClick={() => setPaused(value => !value)}
        aria-label={paused ? 'Play workflow animation' : 'Pause workflow animation'} title={paused ? 'Play animation' : 'Pause animation'}>
        {paused ? <Play size={14} /> : <Pause size={14} />}
      </button>
      <div role="img" aria-label="Illustrative OrbitDesk workflow: Maya Chen at Northstar Labs reports a CSV export timeout. Helpin AI acknowledges and tags the issue. Task ORB-492 links her conversation to the fix. A coding agent prepares a pull request, checks pass, Sam reviews it, and the team releases it. Sam approves a follow-up and Maya is notified in the original conversation.">
        <svg className="wire-svg" width={size.vw || 0} height={132} aria-hidden="true" style={{ left: -Math.max(0, (size.vw - size.w) / 2) }}>
          <path ref={pathRef} className="track" d={d} fill="none" strokeWidth={1.5} />
          <path className="head" d={d} fill="none" strokeWidth={1.5} strokeLinecap="round" style={{ strokeDasharray: len, strokeDashoffset: len - target, opacity: len ? 1 : 0, transition: playing && t > 0 ? 'stroke-dashoffset .8s cubic-bezier(.45,0,.55,1)' : 'none' }} />
        </svg>
        <div className="wire-nodes" aria-hidden="true">
          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(0)] }}>
            <div className={badge(0)}><i /><span>01 Hear</span></div>
            <div className="wart hear">
              <div className="bub on wire-reveal" data-revealed={t >= 5}>
                <Avatar name="maya" />
                <div>
                  <p className="wire-question"><span className="wire-question-space">{QUESTION}</span><span className="wire-question-text">{typedQuestion}{typing ? <i className="wire-caret" /> : null}</span></p>
                  <small>Maya Chen · Northstar Labs</small>
                </div>
              </div>
              <div className="wire-ai-note wire-reveal" data-revealed={t >= 25}>
                <img src="/brand/helpin-icon-ink.svg" width={16} height={16} alt="" />
                <div><b>Helpin AI</b><p>I’ll flag this for the team and keep you updated here.</p></div>
              </div>
              <div className="wire-tags wire-reveal" data-revealed={t >= 28}><span className="wchip">Bug</span><span className="wchip">CSV export</span></div>
            </div>
          </div>

          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(1)] }}>
            <div className={badge(1)}><i /><span>02 Decide</span></div>
            <div className={`wart decide on wire-task wire-reveal${t >= 48 ? ' connected' : ''}`} data-revealed={t >= 38}>
              <div className="wire-task-meta"><span className="tkey">ORB-492</span><span className="wchip am">Bug</span></div>
              <h3>Fix large CSV exports</h3>
              <p>Keep larger reports from timing out.</p>
              <div className="wire-source wire-reveal" data-revealed={t >= 42}><Link2 size={13} /><span>Maya’s conversation</span><CheckIcon className="wire-source-check" size={13} /></div>
              <div className="wire-source wire-reveal" data-revealed={t >= 46}><Link2 size={13} /><span>Northstar Labs</span><CheckIcon className="wire-source-check" size={13} /></div>
              <div className="wire-task-owner"><Avatar name="sam" /><span>Sam Rivera · Engineering</span></div>
            </div>
          </div>

          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(2)] }}>
            <div className={badge(2)}><i /><span>03 Ship</span></div>
            <div className="wart log on wire-reveal" data-revealed={t >= 63}>
              <div className="lhead"><span>Code Builder</span><span>ORB-492</span></div>
              <p className="wire-code-title">CSV export fix</p>
              <ul className="wl">
                <WorkStep label="Pull request prepared" at={68} start={63} t={t} />
                <WorkStep label="Checks passed" at={76} start={68} t={t} />
                <WorkStep label="Reviewed by Sam" at={S.reviewed} start={76} t={t} review />
                <WorkStep label="Released by the team" at={S.released} start={S.reviewed} t={t} />
              </ul>
              <div className="wire-code-link"><Link2 size={12} />Customer conversation attached</div>
            </div>
          </div>

          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(3)] }}>
            <div className={badge(3)}><i /><span>04 Tell</span></div>
            <div className={`wart tell${t >= S.sent ? ' delivered' : ''}`}>
              <div className="wire-followup wire-reveal" data-revealed={t >= 103}><span>Original conversation</span><span className="wire-delivery">{t >= S.sent ? 'Sent' : 'Draft'}</span></div>
              <div className="bub reply on wire-reveal" data-revealed={t >= 105}>
                <div><p>The export fix is live. Please try your report again.</p><small>To Maya · Northstar Labs</small></div>
                <Avatar name="sam" />
              </div>
              <div className="wire-followup-note wire-reveal" data-revealed={t >= 108}><CheckIcon size={13} /><span>{t >= S.sent ? 'Approved by Sam · Sent after release' : 'Sam reviews before sending'}</span></div>
              <div className="wire-resolution wire-reveal" data-revealed={t >= 115}><CheckIcon size={15} /><span>Customer notified. Loop closed.</span></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
