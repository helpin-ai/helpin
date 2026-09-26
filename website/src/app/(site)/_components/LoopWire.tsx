'use client';

import { useEffect, useRef, useState } from 'react';
import { Pause, Play, Link2, Search, FileWarning, Check as CheckIcon } from 'lucide-react';
import { StreamingText } from './StreamingText';
import { useBentoPlayback } from './useBentoPlayback';

// One illustrative OrbitDesk workflow. Static rendering shows the complete story;
// playback restores progressive reveals as the wire reaches each stage.
const TICK_MS = 100;
const LAST = 150;
const QUESTION = 'Exports still time out, even with the smaller report.';
const S = { decide: 60, ship: 85, reviewed: 113, released: 121, tell: 125, sent: 140 };

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
      <span>{review && !done ? (state === 'active' ? 'Reviewing and testing' : 'Awaiting review and tests') : label}</span>
    </li>
  );
}

export function LoopWire() {
  const { container, playing: automatic, cycle } = useBentoPlayback(19000);
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

  return (
    <div className="wire wire-story" ref={container} data-playing={playing} data-step={stage}>
      <button className="wire-playback" type="button" onClick={() => setPaused(value => !value)}
        aria-label={paused ? 'Play workflow animation' : 'Pause workflow animation'} title={paused ? 'Play animation' : 'Pause animation'}>
        {paused ? <Play size={14} /> : <Pause size={14} />}
      </button>
      <div role="img" aria-label="Illustrative OrbitDesk workflow: Maya Chen at Northstar Labs reports that CSV exports still time out after trying a smaller report. Helpin AI checks the earlier conversation and connected logs, then hands Sam the findings and customer report. Task ORB-492 links her conversation to the fix. A coding agent prepares a code change, Sam reviews it and tests pass, then the team releases it. Sam approves the follow-up, then Helpin AI sends it to Maya in the original conversation.">
        <svg className="wire-svg" width={size.vw || 0} height={132} aria-hidden="true" style={{ left: -Math.max(0, (size.vw - size.w) / 2) }}>
          <path ref={pathRef} className="track" d={d} fill="none" strokeWidth={1.5} />
          <path className="head" d={d} fill="none" strokeWidth={1.5} strokeLinecap="round" style={{ strokeDasharray: len, strokeDashoffset: len - target, opacity: len ? 1 : 0, transition: playing && t > 0 ? 'stroke-dashoffset .8s cubic-bezier(.45,0,.55,1)' : 'none' }} />
        </svg>
        <div className="wire-nodes" aria-hidden="true">
          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(0)] }}>
            <div className={badge(0)}><i /><span>01 Answer</span></div>
            <div className="wart hear">
              <div className="bub on wire-reveal" data-revealed={t >= 5}>
                <Avatar name="maya" />
                <div>
                  <p className="wire-question"><StreamingText key={cycle} text={QUESTION} active={playing} delay={500} duration={1100} /></p>
                  <small>Maya Chen · Northstar Labs</small>
                </div>
              </div>
              <div className="wire-log-checks wire-reveal" data-revealed={t >= 20}>
                <div><Search size={12} /><span>{t >= 32 ? 'History and logs checked' : 'Checking history and logs…'}</span></div>
                <div className="wire-reveal" data-revealed={t >= 32}><FileWarning size={12} /><span>CSV export · Request timed out</span></div>
              </div>
              <div className="wire-ai-note wire-reveal" data-revealed={t >= 42}>
                <img src="/brand/helpin-icon-ink.svg" width={16} height={16} alt="" />
                <div><b>Helpin AI</b><p><StreamingText key={cycle} text="I checked our earlier conversation and the logs. I’m sending Sam the findings." active={playing} delay={4200} duration={1300} /></p></div>
              </div>
              <div className="wire-tags wire-reveal" data-revealed={t >= 56}><span className="wchip">Bug</span><span className="wchip">CSV export</span></div>
              <div className="wire-handoff wire-reveal" data-revealed={t >= 56}><Avatar name="sam" /><span>Handed to Sam · Findings attached</span></div>
            </div>
          </div>

          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(1)] }}>
            <div className={badge(1)}><i /><span>02 Decide</span></div>
            <div className={`wart decide on wire-task wire-reveal${t >= 78 ? ' connected' : ''}`} data-revealed={t >= 68}>
              <div className="wire-task-meta"><span className="tkey">ORB-492</span><span className="wchip am">Bug</span></div>
              <h3>Fix large CSV exports</h3>
              <p>Previous workaround failed. Customer report and logs attached.</p>
              <div className="wire-source wire-reveal" data-revealed={t >= 72}><Link2 size={13} /><span>Maya’s conversation</span><CheckIcon className="wire-source-check" size={13} /></div>
              <div className="wire-source wire-reveal" data-revealed={t >= 76}><Link2 size={13} /><span>Northstar Labs</span><CheckIcon className="wire-source-check" size={13} /></div>
              <div className="wire-task-owner"><Avatar name="sam" /><span>Sam Rivera · Engineering</span></div>
            </div>
          </div>

          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(2)] }}>
            <div className={badge(2)}><i /><span>03 Ship</span></div>
            <div className="wart log on wire-reveal" data-revealed={t >= 93}>
              <div className="lhead"><span>Coding agent</span><span>ORB-492</span></div>
              <p className="wire-code-title">CSV export fix</p>
              <ul className="wl">
                <WorkStep label="Change prepared by agent" at={98} start={93} t={t} />
                <WorkStep label="Reviewed and tested by Sam" at={S.reviewed} start={98} t={t} review />
                <WorkStep label="Released by the team" at={S.released} start={S.reviewed} t={t} />
              </ul>
              <div className="wire-code-link"><Link2 size={12} />Original request stays attached</div>
            </div>
          </div>

          <div className="wnode" style={{ paddingTop: LEVEL_TOP[levelOf(3)] }}>
            <div className={badge(3)}><i /><span>04 Follow up</span></div>
            <div className={`wart tell${t >= S.sent ? ' delivered' : ''}`}>
              <div className="wire-followup wire-reveal" data-revealed={t >= 133}><span>Original conversation</span><span className="wire-delivery">{t >= S.sent ? 'Sent' : 'Draft'}</span></div>
              <div className="bub reply on wire-reveal" data-revealed={t >= 135}>
                <div><p>The fix is live, Maya. Please try your export again.</p><small>Helpin AI · To Maya</small></div>
                <span className="bav"><img src="/brand/helpin-icon-white.svg" width={15} height={15} alt="" /></span>
              </div>
              <div className="wire-followup-note wire-reveal" data-revealed={t >= 138}><CheckIcon size={13} /><span>{t >= S.sent ? 'Approved by Sam · Sent by Helpin AI' : 'Helpin AI draft · Awaiting approval'}</span></div>
              <div className="wire-resolution wire-reveal" data-revealed={t >= 145}><CheckIcon size={15} /><span>Customer notified. Loop closed.</span></div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
