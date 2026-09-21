'use client';

import { useId, useState, type CSSProperties } from 'react';
import { BookOpen, FilePlus2, MessageSquare, MousePointer2, Pause, Play, Plug, ShieldCheck, type LucideIcon } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-controls.css';

const CONTROLS = [
  { id: 'responses', title: 'Choose how AI responds.', copy: 'Let agents answer customers, keep their help internal, or turn automatic replies off. Choose the mode that fits your team.', description: 'The support inbox has AI-first, Internal, and Off response modes. The selection moves from AI-first to Internal, keeping suggestions with the team.' },
  { id: 'tools', title: 'Give agents the right tools.', copy: 'Select the tools each agent can use, from workspace knowledge to selected tools on your connected MCP servers.', description: 'A support agent receives three selected tools: search knowledge, read conversations, and look up an issue through an external MCP connection.' },
  { id: 'approvals', title: 'Review actions that need you.', copy: 'Configure agent approval settings so your team can review a requested action before it runs.', description: 'An agent proposes creating a task for Maya’s CSV export issue. Sam reviews the action and approves it; a green confirmation shows Approved by Sam.' },
] as const;
const delay = (seconds: number) => ({ '--sc-delay': `${seconds}s` }) as CSSProperties;
function Icon({ glyph: Glyph, x, y, size = 18 }: { glyph: LucideIcon; x: number; y: number; size?: number }) { return <g transform={`translate(${x} ${y})`}><Glyph width={size} height={size} strokeWidth={1.6} /></g>; }
function Helpin({ x, y }: { x: number; y: number }) { return <g><rect x={x} y={y} width={30} height={30} rx={8} fill="#174b32" /><image href="/brand/helpin-icon-white.svg" x={x + 6} y={y + 6} width={18} height={18} /></g>; }
function Tick({ x, y, at }: { x: number; y: number; at: number }) { return <path className="sc-tick" style={delay(at)} d={`M${x} ${y + 5} l4 4 9 -10`} pathLength={1} />; }
function Responses() {
  return <>
    <rect x={28} y={22} width={364} height={310} rx={14} className="sc-panel" />
    <Helpin x={47} y={40} /><text x={88} y={54} className="sc-title">Support inbox</text><text x={88} y={72} className="sc-meta">AI response mode</text><path d="M47 88 H373" className="sc-divider" />
    <rect x={42} y={163} width={336} height={53} rx={9} className="sc-mode-selection" />
    {[
      { name: 'AI-first', detail: 'Agents reply to customers' },
      { name: 'Internal', detail: 'Agents assist your team' },
      { name: 'Off', detail: 'Your team handles every reply' },
    ].map(({ name, detail }, i) => <g key={name}><text x={59} y={126 + i * 59} className="sc-body">{name}</text><text x={59} y={145 + i * 59} className="sc-meta">{detail}</text><circle cx={352} cy={132 + i * 59} r={9} fill="#fff" stroke="#cddbd2" /></g>)}
    <circle cx={352} cy={132} r={5} className="sc-mode-initial" fill="#168052" /><circle cx={352} cy={191} r={5} className="sc-mode-final" fill="#168052" />
    <path d="M47 282 H373" className="sc-divider" />
    <g className="sc-mode-initial"><text x={210} y={309} textAnchor="middle" className="sc-small sc-green">Agents can reply to customers</text></g>
    <g className="sc-mode-final"><text x={210} y={309} textAnchor="middle" className="sc-small sc-green">Suggestions stay with your team</text></g>
    <g className="sc-mode-cursor sc-cursor"><Icon glyph={MousePointer2} x={348} y={193} size={22} /></g>
  </>;
}
function Tools() {
  return <>
    <rect x={28} y={22} width={364} height={310} rx={14} className="sc-panel" />
    <Helpin x={47} y={40} /><text x={88} y={54} className="sc-title">Support agent</text><text x={88} y={72} className="sc-meta">Selected tools</text><path d="M47 88 H373" className="sc-divider" />
    {[
      { icon: BookOpen, name: 'Search knowledge', detail: 'Workspace docs', start: .4 },
      { icon: MessageSquare, name: 'Read conversations', detail: 'Customer history', start: 1.15 },
      { icon: Plug, name: 'Look up an issue', detail: 'External MCP · Connected tracker', start: 1.9 },
    ].map(({ icon, name, detail, start }, i) => <g key={name}>
      <rect x={42} y={103 + i * 59} width={336} height={53} rx={9} className="sc-tool-highlight" style={delay(start)} />
      <rect x={52} y={114 + i * 59} width={30} height={30} rx={7} fill="#f0f5f2" /><g className="sc-green"><Icon glyph={icon} x={58} y={120 + i * 59} /></g>
      <text x={94} y={125 + i * 59} className="sc-body">{name}</text><text x={94} y={145 + i * 59} className="sc-meta">{detail}</text>
      <rect x={341} y={119 + i * 59} width={22} height={22} rx={6} fill="#eaf5ee" stroke="#cbdfd2" /><Tick x={345} y={124 + i * 59} at={start + .2} />
    </g>)}
    <path d="M47 282 H373" className="sc-divider" /><text x={210} y={309} textAnchor="middle" className="sc-small sc-green">Only the tools you select</text>
  </>;
}
function Approvals({ id }: { id: string }) {
  return <>
    <defs><clipPath id={`${id}-sam`}><circle cx={98} cy={304} r={10} /></clipPath></defs>
    <rect x={28} y={22} width={364} height={310} rx={14} className="sc-panel" />
    <rect x={47} y={40} width={30} height={30} rx={8} fill="#f7f0df" /><g className="sc-amber"><Icon glyph={ShieldCheck} x={53} y={46} /></g><text x={88} y={54} className="sc-title">Approval required</text><text x={88} y={72} className="sc-meta">Support agent · Proposed action</text><path d="M47 88 H373" className="sc-divider" />
    <g className="sc-green"><Icon glyph={FilePlus2} x={48} y={106} /></g><text x={75} y={120} className="sc-body">Create an engineering task</text>
    <rect x={47} y={139} width={326} height={78} rx={9} fill="#f5f7f6" stroke="#e5ece7" /><text x={61} y={163} className="sc-body">Investigate incomplete CSV exports</text><text x={61} y={186} className="sc-meta">Maya Chen · Northstar Labs</text><text x={61} y={204} className="sc-small sc-green">Original conversation attached</text>
    <g className="sc-approval-pending"><rect x={103} y={238} width={102} height={33} rx={7} fill="#174b32" /><text x={154} y={259} textAnchor="middle" className="sc-button-text">Approve</text><rect x={215} y={238} width={102} height={33} rx={7} fill="#fff" stroke="#dce5df" /><text x={266} y={259} textAnchor="middle" className="sc-small">Cancel</text></g>
    <g className="sc-approved"><rect x={83} y={238} width={254} height={33} rx={7} fill="#e4f4e9" stroke="#b5d9c3" /><rect x={83} y={238} width={254} height={33} rx={7} className="sc-success-ring" /><circle cx={105} cy={255} r={10} fill="#17824f" /><g className="sc-white-tick"><Tick x={99} y={250} at={2.8} /></g><text x={125} y={259} className="sc-small sc-green">Approved by Sam</text></g>
    <path d="M47 285 H373" className="sc-divider" /><image href="/new/avatars/sam.webp" x={88} y={294} width={20} height={20} clipPath={`url(#${id}-sam)`} /><text x={118} y={308} className="sc-small">Sam Rivera · Your team stays in control</text>
    <rect x={28} y={22} width={364} height={310} rx={14} className="sc-success-outline" />
    <g className="sc-approval-cursor sc-cursor"><Icon glyph={MousePointer2} x={169} y={258} size={22} /></g>
  </>;
}
function ControlCard({ control }: { control: typeof CONTROLS[number] }) {
  const { container, playing, cycle } = useBentoPlayback(8500);
  const [paused, setPaused] = useState(false);
  const id = `sc-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  return <article className="support-control-card">
    <div className="support-control-art" ref={container} data-playing={playing && !paused}>
      <div className="support-control-toolbar"><span>OrbitDesk</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${control.id} settings animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
      <svg key={cycle} viewBox="0 0 420 354" width={420} height={354} className="support-control-svg" role="img" aria-labelledby={`${id}-title ${id}-desc`}><title id={`${id}-title`}>{control.title}</title><desc id={`${id}-desc`}>{control.description}</desc><g aria-hidden="true">{control.id === 'responses' ? <Responses /> : control.id === 'tools' ? <Tools /> : <Approvals id={id} />}</g></svg>
    </div>
    <div className="support-control-copy"><h3>{control.title}</h3><p>{control.copy}</p></div>
  </article>;
}
export function SupportControls() { return <div className="support-control-cards">{CONTROLS.map(control => <ControlCard key={control.id} control={control} />)}</div>; }
