'use client';

import { useId, useState, type CSSProperties, type ReactNode } from 'react';
import { BookOpen, FileText, GitBranch, Link2, Pause, Play, type LucideIcon } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-scene.css';

type Variant = 'answer' | 'handoff' | 'followup';
const CONTENT = {
  answer: ['An answer with a source', 'Maya asks how to export selected contacts. Helpin AI finds the export guide and replies with steps and a linked source.'],
  handoff: ['A handoff with the history', 'Maya reports an incomplete CSV export. Helpin AI finds the pagination error in connected logs and hands the conversation to Sam Rivera with its findings and customer history attached.'],
  followup: ['A fix followed by a customer update', 'Maya’s export issue is linked to EXP-142. After the fix is reviewed, tested and released, Helpin AI sends Maya an update approved by Sam in the original conversation.'],
} as const;
const timing = (seconds: number) => ({ '--sw-delay': `${seconds}s` }) as CSSProperties;
function Reveal({ at, children }: { at: number; children: ReactNode }) { return <g className="sw-reveal" style={timing(at)}>{children}</g>; }
function Glyph({ icon: Icon, x, y, size = 18 }: { icon: LucideIcon; x: number; y: number; size?: number }) { return <g transform={`translate(${x} ${y})`}><Icon width={size} height={size} strokeWidth={1.6} /></g>; }
function Helpin({ x, y }: { x: number; y: number }) { return <g><rect x={x} y={y} width={28} height={28} rx={8} fill="#174b32" /><image href="/brand/helpin-icon-white.svg" x={x + 6} y={y + 6} width={16} height={16} /></g>; }
function Avatar({ id, x, y, person }: { id: string; x: number; y: number; person: 'maya' | 'sam' }) { return <g><defs><clipPath id={`${id}-${person}`}><circle cx={x + 15} cy={y + 15} r={15} /></clipPath></defs><image href={`/new/avatars/${person}.webp`} x={x} y={y} width={30} height={30} clipPath={`url(#${id}-${person})`} /></g>; }
function Tick({ x, y, at }: { x: number; y: number; at: number }) { return <path className="sw-tick" d={`M${x} ${y + 5} l4 4 9 -10`} pathLength={1} style={timing(at)} />; }
function Typed({ text, x, y, at }: { text: string; x: number; y: number; at: number }) { return <text x={x} y={y} className="sw-body">{Array.from(text).map((character, i) => <tspan key={i} className="sw-character" style={timing(at + i * .024)}>{character}</tspan>)}</text>; }
function Answer({ id }: { id: string }) {
  return <>
    <rect x={25} y={18} width={370} height={324} rx={14} className="sw-panel" />
    <Avatar id={id} x={43} y={35} person="maya" /><text x={84} y={48} className="sw-title">Maya Chen</text><text x={84} y={65} className="sw-meta">Northstar Labs</text><rect x={290} y={39} width={87} height={22} rx={11} fill="#eaf5ed" /><text x={333} y={54} textAnchor="middle" className="sw-small sw-green">AI handling</text>
    <path d="M43 79 H377" className="sw-divider" />
    <Reveal at={.1}><rect x={43} y={92} width={278} height={53} rx={10} fill="#f3f5f4" /><text x={57} y={114} className="sw-body">How do I export just the contacts</text><text x={57} y={134} className="sw-body">I selected?</text></Reveal>
    <Reveal at={.55}><Helpin x={43} y={161} /><text x={82} y={179} className="sw-title">Helpin AI</text></Reveal>
    <g className="sw-thinking">{[0,1,2].map(i => <circle key={i} cx={169 + i * 7} cy={174} r={2} style={timing(i * .16)} />)}</g>
    <Reveal at={1.5}><rect x={82} y={192} width={295} height={102} rx={10} fill="#edf7f0" stroke="#cfe5d6" /></Reveal>
    <Typed x={96} y={217} at={1.65} text="Select your contacts, then choose" /><Typed x={96} y={238} at={2.48} text="Export → Selected contacts." />
    <Reveal at={3.35}><rect x={96} y={254} width={176} height={26} rx={6} fill="#fff" stroke="#d7e7dd" /><g className="sw-green"><Glyph icon={BookOpen} x={104} y={260} size={14} /></g><text x={124} y={271} className="sw-small sw-green">Export your contacts ↗</text></Reveal>
    <Reveal at={4.1}><circle cx={58} cy={318} r={10} fill="#e2f2e7" /><Tick x={52} y={313} at={4.2} /><text x={77} y={322} className="sw-small sw-green">Answered with product knowledge</text></Reveal>
  </>;
}
function Handoff({ id }: { id: string }) {
  return <>
    <rect x={25} y={18} width={370} height={324} rx={14} className="sw-panel" />
    <Avatar id={id} x={43} y={35} person="maya" /><text x={84} y={48} className="sw-title">CSV export stops early</text><text x={84} y={65} className="sw-meta">Maya Chen · Northstar Labs</text><path d="M43 79 H377" className="sw-divider" />
    <Reveal at={.3}><rect x={43} y={93} width={334} height={85} rx={9} fill="#fcf8ed" stroke="#eee4c9" /><g className="sw-amber"><Glyph icon={FileText} x={56} y={106} size={15} /></g><text x={78} y={118} className="sw-small sw-amber">INTERNAL HANDOFF NOTE</text><text x={56} y={141} className="sw-body">Logs: pagination stops at 10,000 rows.</text><text x={56} y={162} className="sw-meta">Findings and customer history attached.</text></Reveal>
    <path d="M93 221 H328" className="sw-wire" /><path d="M93 221 H328" className="sw-route" pathLength={1} style={timing(1.3)} />
    <Helpin x={51} y={205} /><text x={45} y={254} className="sw-meta">Helpin AI</text>
    <rect x={152} y={205} width={118} height={30} rx={15} fill="#fff" stroke="#e0e7e2" /><text x={211} y={224} textAnchor="middle" className="sw-small">Human handoff</text>
    <Avatar id={id} x={331} y={205} person="sam" /><text x={324} y={254} className="sw-meta">Sam Rivera</text>
    <g className="sw-handoff-signal"><circle cx={94} cy={221} r={4} fill="#49a575" /></g>
    <Reveal at={2.5}><rect x={43} y={278} width={334} height={45} rx={9} fill="#e9f6ee" stroke="#b7dbc5" /><rect x={43} y={278} width={334} height={45} rx={9} className="sw-success-ring" /><circle cx={65} cy={300} r={11} fill="#17824f" /><g className="sw-white-tick"><Tick x={59} y={295} at={2.8} /></g><text x={85} y={298} className="sw-title sw-green">Assigned to Sam</text><text x={85} y={313} className="sw-meta">Same conversation. Full history.</text></Reveal>
  </>;
}
function Followup() {
  return <>
    <rect x={39} y={22} width={342} height={53} rx={10} className="sw-panel" /><g className="sw-green"><Glyph icon={Link2} x={53} y={39} /></g><text x={80} y={44} className="sw-title">CSV export stops early</text><text x={80} y={62} className="sw-meta">Maya Chen · Original request</text>
    <path d="M210 75 V104 M210 223 V253" className="sw-wire" /><path d="M210 75 V104" className="sw-route" pathLength={1} style={timing(.5)} /><path d="M210 223 V253" className="sw-route" pathLength={1} style={timing(2.6)} />
    <Reveal at={1}><rect x={25} y={104} width={370} height={119} rx={12} className="sw-panel" /><g className="sw-green"><Glyph icon={GitBranch} x={43} y={121} size={17} /></g><text x={68} y={135} className="sw-meta">EXP-142</text><rect x={290} y={117} width={87} height={23} rx={11} fill="#e9f6ee" /><text x={333} y={133} textAnchor="middle" className="sw-small sw-green">Released</text><text x={43} y={165} className="sw-title">Fix incomplete CSV exports</text><text x={43} y={185} className="sw-meta">Reviewed and tested · Sam Rivera</text><path d="M43 196 H377" className="sw-divider" /><g className="sw-green"><Glyph icon={Link2} x={43} y={203} size={12} /></g><text x={63} y={214} className="sw-small sw-green">Customer conversation attached</text></Reveal>
    <Reveal at={3.3}><rect x={39} y={253} width={342} height={88} rx={10} fill="#edf7f0" stroke="#c6e0cf" /><Helpin x={51} y={266} /><text x={88} y={282} className="sw-title">Helpin AI → Maya</text><rect x={302} y={265} width={65} height={22} rx={6} fill="#fff" stroke="#d4e5da" /><text x={334} y={280} textAnchor="middle" className="sw-small">Sent</text><text x={53} y={305} className="sw-body">The export fix is live. Try your report again.</text><text x={53} y={325} className="sw-meta">Approved by Sam · Sent after release.</text></Reveal>
  </>;
}

export function SupportScene({ variant }: { variant: Variant }) {
  const { container, playing, cycle } = useBentoPlayback(9500);
  const [paused, setPaused] = useState(false);
  const id = `sw-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const [title, description] = CONTENT[variant];
  return <div className={`support-scene support-scene-${variant}`} ref={container} data-playing={playing && !paused}>
    <div className="support-scene-toolbar"><span><span className="sw-workspace">O</span>OrbitDesk</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} animation: ${title}`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
    <svg key={cycle} className="support-workflow-art" viewBox="0 0 420 360" width={420} height={360} role="img" aria-labelledby={`${id}-title ${id}-description`}><title id={`${id}-title`}>{title}</title><desc id={`${id}-description`}>{description}</desc><g aria-hidden="true">{variant === 'answer' ? <Answer id={id} /> : variant === 'handoff' ? <Handoff id={id} /> : <Followup />}</g></svg>
  </div>;
}
