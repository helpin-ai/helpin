'use client';

import { useId, type CSSProperties } from 'react';
import { BookOpen, Clock3, FilePenLine, ListChecks, MessageSquare, MousePointer2, ShieldCheck, type LucideIcon } from 'lucide-react';
import { useBentoPlayback } from './useBentoPlayback';

type Variant = 'tools' | 'approvals' | 'triggers';
const labels: Record<Variant, [string, string]> = {
  tools: ['Choose an agent’s tools', 'A support agent has three selected tools: search knowledge, read conversations, and draft replies.'],
  approvals: ['Review an agent’s action', 'An agent prepares a reply to Maya. A teammate approves the message before it is sent.'],
  triggers: ['Schedule recurring work', 'A weekday schedule starts a rollout review agent, which prepares a summary of blockers.'],
};
const delay = (seconds: number) => ({ '--ca-delay': `${seconds}s` }) as CSSProperties;
function Icon({ icon: Glyph, x, y, size = 18 }: { icon: LucideIcon; x: number; y: number; size?: number }) {
  return <g transform={`translate(${x} ${y})`} color="currentColor"><Glyph width={size} height={size} strokeWidth={1.6} aria-hidden="true" /></g>;
}
function HelpinIcon({ x, y }: { x: number; y: number }) {
  return <g><rect x={x} y={y} width={30} height={30} rx={8} fill="#173e2e" /><image href="/brand/helpin-icon-white.svg" x={x + 7} y={y + 7} width={16} height={16} /></g>;
}
function Tick({ x, y, start }: { x: number; y: number; start: number }) {
  return <path className="ca-tick" d={`M${x} ${y+4} l4 4 8 -9`} pathLength={1} fill="none" stroke="#138253" strokeWidth={2} strokeLinecap="round" strokeLinejoin="round" style={delay(start)} />;
}
function Tools() {
  return <>
    <rect x={40} y={35} width={340} height={254} rx={14} className="ca-panel" />
    <HelpinIcon x={59} y={53} /><text x={100} y={66} className="ca-title">Support agent</text><text x={100} y={84} className="ca-meta">Selected tools</text>
    <rect x={283} y={55} width={78} height={23} rx={11} fill="#edf6f0" /><text x={322} y={70} textAnchor="middle" className="ca-green ca-small">3 enabled</text>
    <path d="M59 102 H361" className="ca-divider" />
    {[
      { icon: BookOpen, label: 'Search knowledge', detail: 'Find a trusted answer' },
      { icon: MessageSquare, label: 'Read conversations', detail: 'Understand the history' },
      { icon: FilePenLine, label: 'Draft replies', detail: 'Prepare the next response' },
    ].map(({icon,label,detail}, i) => <g key={label}>
      <rect className="ca-tool-highlight" x={50} y={111+i*54} width={320} height={49} rx={8} style={delay(.4+i*.65)} />
      <rect x={59} y={119+i*54} width={30} height={30} rx={8} fill="#f1f4f2" /><Icon icon={icon} x={65} y={125+i*54} />
      <text x={101} y={130+i*54} className="ca-body">{label}</text><text x={101} y={147+i*54} className="ca-meta">{detail}</text>
      <rect x={329} y={124+i*54} width={23} height={23} rx={6} fill="#edf6f0" stroke="#cce3d5" /><Tick x={334} y={130+i*54} start={.55+i*.65} />
    </g>)}
  </>;
}
function Approvals({ id }: { id: string }) {
  return <>
    <defs><clipPath id={`${id}-avatar`}><circle cx={68} cy={120} r={12} /></clipPath></defs>
    <rect x={40} y={35} width={340} height={254} rx={14} className="ca-panel" />
    <rect x={58} y={53} width={30} height={30} rx={8} fill="#f5f1e7" /><g className="ca-amber"><Icon icon={ShieldCheck} x={64} y={59} /></g>
    <text x={100} y={65} className="ca-title">Review before sending</text><text x={100} y={84} className="ca-meta">Support agent · Proposed action</text>
    <path d="M59 102 H361" className="ca-divider" />
    <image href="/new/avatars/maya.webp" x={56} y={108} width={24} height={24} clipPath={`url(#${id}-avatar)`} /><text x={89} y={124} className="ca-body">Reply to Maya</text>
    <rect x={58} y={144} width={304} height={61} rx={8} fill="#f5f7f6" /><text x={72} y={167} className="ca-body">The Okta setup guide is ready<tspan x={72} dy={20}>for your pilot.</tspan></text>
    <g className="ca-pending">
      <rect x={58} y={225} width={96} height={32} rx={7} fill="#173e2e" /><text x={106} y={245} textAnchor="middle" className="ca-button">Approve</text>
      <rect x={164} y={225} width={78} height={32} rx={7} fill="#fff" stroke="#dfe6e1" /><text x={203} y={245} textAnchor="middle" className="ca-small">Reject</text>
    </g>
    <g className="ca-approved"><rect x={58} y={225} width={200} height={32} rx={7} fill="#edf6f0" /><Tick x={70} y={237} start={2.35} /><text x={93} y={245} className="ca-green ca-small">Approved by Sam</text></g>
    <g className="ca-cursor" aria-hidden="true"><Icon icon={MousePointer2} x={130} y={243} size={23} /></g>
  </>;
}
function Triggers() {
  return <>
    <path d="M210 86 V234" className="ca-wire" /><path d="M210 86 V234" className="ca-route" pathLength={1} />
    {[
      { y: 30, icon: Clock3, title: 'Every weekday', detail: '09:00 · Scheduled trigger' },
      { y: 130, title: 'Review rollout tasks', detail: 'Agent gathers the blockers' },
      { y: 230, icon: ListChecks, title: 'Summary ready', detail: 'Next steps for your team' },
    ].map(({y,icon,title,detail},i) => <g key={title}>
      <rect x={58} y={y} width={304} height={60} rx={12} className="ca-panel" />
      <rect x={58} y={y} width={304} height={60} rx={12} className="ca-node-active" style={delay(i*1.05)} />
      {icon ? <><rect x={72} y={y+15} width={30} height={30} rx={8} fill="#f0f5f2" /><g className="ca-green"><Icon icon={icon} x={78} y={y+21} /></g></> : <HelpinIcon x={72} y={y+15} />}
      <text x={114} y={y+26} className="ca-body">{title}</text><text x={114} y={y+44} className="ca-meta">{detail}</text>
      <Tick x={332} y={y+26} start={.35+i*1.05} />
    </g>)}
  </>;
}
export function AgentControlArt({ variant }: { variant: Variant }) {
  const id = `ca-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const { container, playing, cycle } = useBentoPlayback(7500);
  const [title, description] = labels[variant];
  return <div ref={container} className="control-art" data-playing={playing}>
    <svg key={cycle} className="ca-svg" viewBox="0 0 420 320" width={420} height={320} role="img" aria-labelledby={`${id}-title ${id}-desc`}>
      <title id={`${id}-title`}>{title}</title><desc id={`${id}-desc`}>{description}</desc>
      {variant === 'tools' ? <Tools /> : variant === 'approvals' ? <Approvals id={id} /> : <Triggers />}
    </svg>
  </div>;
}
