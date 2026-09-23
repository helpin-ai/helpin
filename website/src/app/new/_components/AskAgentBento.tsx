'use client';

import { useId, type CSSProperties } from 'react';
import { useBentoPlayback } from './useBentoPlayback';
import { Check, Code2, Database, FileText, GitBranch, ListChecks, MessageSquare, Plug, Ticket, Video, type LucideIcon } from 'lucide-react';

type Variant = 'answers' | 'coordination' | 'mcp';
const DESCRIPTIONS: Record<Variant, { title: string; description: string }> = {
  answers: { title: 'Workspace answers', description: 'Conversation, meeting, and project context converge into an Ask Agent answer: SSO approval is pending and the Okta guide is needed.' },
  coordination: { title: 'Agent coordination', description: 'A rollout review branches into Engineering and Docs agents working in parallel. Both results feed the next steps.' },
  mcp: { title: 'Connected tools', description: 'Selected issue tracker and customer data tools bring outside context into Ask Agent.' },
};
const delay = (seconds: number) => ({ '--ab-delay': `${seconds}s` }) as CSSProperties;

function Icon({ icon: Glyph, x, y, color = '#6be5ad', size = 28 }: { icon: LucideIcon; x: number; y: number; color?: string; size?: number }) {
  return <g transform={`translate(${x} ${y})`} color={color}><Glyph width={size} height={size} strokeWidth={1.7} aria-hidden="true" /></g>;
}

function Flow({ d, start }: { d: string; start: number }) {
  return <g fill="none"><path className="ab-wire" d={d} /><path className="ab-flow" d={d} pathLength={100} style={delay(start)} /></g>;
}

function Node({ id, x, y, width, label, icon, color = '#6be5ad', phase = 0, complete = false }: { id: string; x: number; y: number; width: number; label: string; icon: LucideIcon; color?: string; phase?: number; complete?: boolean }) {
  return <g transform={`translate(${x} ${y})`}>
    <rect className="ab-panel" width={width} height={64} rx={12} fill={`url(#${id}-panel)`} />
    <rect className="ab-node-glow" width={width} height={64} rx={12} style={delay(phase)} />
    <rect x={12} y={16} width={32} height={32} rx={8} fill={color} opacity={.1} />
    <Icon icon={icon} x={17} y={21} size={22} color={color} />
    <text x={53} y={37} className="ab-node-label">{label}</text>
    {complete && <g className="ab-check" style={delay(phase + .4)}><circle cx={width - 22} cy={32} r={9} fill="#6be5ad" /><Icon icon={Check} x={width - 28} y={26} size={12} color="#092219" /></g>}
  </g>;
}

function Answers({ id }: { id: string }) {
  return <>
    <Flow d="M174 68 H190 Q212 68 212 100 V144 Q212 164 236 164 H250" start={.2} />
    <Flow d="M174 164 H250" start={.55} />
    <Flow d="M174 260 H190 Q212 260 212 236 V184 Q212 164 236 164 H250" start={.9} />
    <Node id={id} x={16} y={36} width={158} label="Conversation" icon={MessageSquare} phase={0} />
    <Node id={id} x={16} y={132} width={158} label="Meeting" icon={Video} phase={.35} />
    <Node id={id} x={16} y={228} width={158} label="Project" icon={FileText} phase={.7} />
    <rect x={263} y={68} width={230} height={200} rx={16} style={{ fill: 'var(--art-dark-soft)', stroke: 'var(--art-dark-border)' }} />
    <g transform="translate(250 82)">
      <rect className="ab-panel" width={246} height={200} rx={16} fill={`url(#${id}-panel)`} />
      <rect className="ab-result-glow" width={246} height={200} rx={16} style={delay(2.1)} />
      <Icon icon={GitBranch} x={19} y={19} />
      <text x={58} y={40} className="ab-panel-title">Ask Agent</text>
      <path d="M20 61 H226" style={{ stroke: 'var(--art-dark-border)' }} />
      <g className="ab-answer-reveal" style={delay(2.1)}>
        <text x={22} y={99} className="ab-answer-title">SSO approval<tspan x={22} dy={29}>pending</tspan></text>
        <text x={22} y={160} className="ab-detail">Okta guide needed</text>
        <circle cx={23} cy={181} r={3} fill="#6be5ad" /><text x={33} y={185} className="ab-caption">3 sources connected</text>
      </g>
    </g>
  </>;
}

function Coordination({ id }: { id: string }) {
  return <>
    <Flow d="M260 86 V103 Q260 119 240 119 H134 Q120 119 120 135 V144" start={.35} />
    <Flow d="M260 86 V103 Q260 119 280 119 H386 Q400 119 400 135 V144" start={.35} />
    <Flow d="M120 208 V225 Q120 240 138 240 H240 Q260 240 260 258" start={2.1} />
    <Flow d="M400 208 V225 Q400 240 382 240 H280 Q260 240 260 258" start={2.1} />
    <Node id={id} x={146} y={22} width={228} label="Rollout review" icon={GitBranch} phase={0} complete />
    <Node id={id} x={16} y={144} width={210} label="Engineering" icon={Code2} color="#b8a4fa" phase={1.3} complete />
    <Node id={id} x={294} y={144} width={210} label="Docs" icon={FileText} color="#edc777" phase={1.3} complete />
    <text x={260} y={137} textAnchor="middle" className="ab-caption">IN PARALLEL</text>
    <Node id={id} x={146} y={258} width={228} label="Next steps" icon={ListChecks} phase={3.25} complete />
  </>;
}

function Mcp({ id }: { id: string }) {
  return <>
    <Flow d="M184 102 H198 Q213 102 213 125 V156 Q213 172 230 172" start={.25} />
    <Flow d="M184 256 H198 Q213 256 213 238 V188 Q213 172 230 172" start={.55} />
    <Flow d="M290 172 H336" start={2} />
    <Node id={id} x={16} y={70} width={168} label="Issue tracker" icon={Ticket} color="#b8a4fa" phase={0} />
    <Node id={id} x={16} y={224} width={168} label="Customer data" icon={Database} color="#91bcf8" phase={.3} />
    <g transform="translate(230 130)">
      <rect className="ab-panel" width={60} height={84} rx={14} fill={`url(#${id}-panel)`} />
      <rect className="ab-node-glow" width={60} height={84} rx={14} style={delay(1.3)} />
      <Icon icon={Plug} x={16} y={14} /><text x={30} y={67} textAnchor="middle" className="ab-node-label">Tools</text>
    </g>
    <g transform="translate(336 82)">
      <rect className="ab-panel" width={168} height={194} rx={16} fill={`url(#${id}-panel)`} />
      <rect className="ab-result-glow" width={168} height={194} rx={16} style={delay(3)} />
      <Icon icon={GitBranch} x={17} y={20} size={24} /><text x={49} y={38} className="ab-panel-title">Ask Agent</text>
      <path d="M16 59 H152" style={{ stroke: 'var(--art-dark-border)' }} />
      <g className="ab-answer-reveal" style={delay(3)}>
        <circle cx={84} cy={92} r={17} fill="#123e2e" stroke="#4fc78f" /><Icon icon={Check} x={73} y={81} size={22} />
        <text x={84} y={141} textAnchor="middle" className="ab-answer-title">Context<tspan x={84} dy={26}>ready</tspan></text>
      </g>
    </g>
  </>;
}

export function AskAgentBento({ variant }: { variant: Variant }) {
  const id = `ab-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const { container, playing, cycle } = useBentoPlayback();
  const { title, description } = DESCRIPTIONS[variant];

  return <div ref={container} className="ask-agent-art ab-scene" data-playing={playing}>
    <svg key={cycle} className="ab-svg" viewBox="0 0 520 350" width={520} height={350} role="img" aria-labelledby={`${id}-title ${id}-description`}>
      <title id={`${id}-title`}>{title}</title><desc id={`${id}-description`}>{description}</desc>
      <defs><linearGradient id={`${id}-panel`} x1="0" y1="0" x2="1" y2="1"><stop style={{ stopColor: 'var(--art-dark-raised)' }} /><stop offset="1" style={{ stopColor: 'var(--art-dark)' }} /></linearGradient></defs>
      {variant === 'answers' ? <Answers id={id} /> : variant === 'coordination' ? <Coordination id={id} /> : <Mcp id={id} />}
    </svg>
  </div>;
}
