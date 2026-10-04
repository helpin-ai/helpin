'use client';

import { Check, LoaderCircle, MousePointer2, type LucideIcon } from 'lucide-react';
import type { CSSProperties } from 'react';
import './workflow-parts.css';

export type ContextSource = { icon: LucideIcon; label: string; detail: string };
export function WorkflowSources({ sources, phase, compact = false }: { sources: ContextSource[]; phase: number; compact?: boolean }) {
  return <div className={`wf-sources${compact ? ' wf-sources-compact' : ''}`}>
    {sources.map(({ icon: Icon, label, detail }, index) => {
      const active = phase === index + 1;
      const done = phase > index + 1;
      if (!active && !done) return null;
      return <div className="wf-source" data-active={active} key={label}>
        <Icon size={15} /><div><strong>{label}</strong>{!compact && <span>{detail}</span>}</div>
        {done ? <Check size={14} /> : <LoaderCircle className="wf-spinner" size={14} />}
      </div>;
    })}
  </div>;
}
export function WorkflowAgent({ name, role, status, working = false }: { name: string; role: string; status: string; working?: boolean }) {
  return <div className="wf-agent"><img className={name === 'Helpin AI' ? 'wf-helpin-mark' : undefined} src={name === 'Helpin AI' ? '/brand/helpin-icon-white.svg' : `/new/agents/${name.toLowerCase()}.svg`} width={30} height={30} alt="" loading="lazy" /><div><strong>{name}<span>{role}</span></strong><small>{working && <i className="wf-working-dot" />}{status}</small></div></div>;
}
export function WorkflowClick({ className = '', delay = 0 }: { className?: string; delay?: number }) {
  return <span aria-hidden="true" className={`wf-click ${className}`} style={{ '--wf-click-delay': `${delay}ms` } as CSSProperties}><i /><MousePointer2 size={24} fill="white" stroke="#173e2e" /></span>;
}
