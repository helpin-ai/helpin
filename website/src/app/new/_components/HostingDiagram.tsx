'use client';

import { useId, type CSSProperties } from 'react';
import { Database, HardDrive, LockKeyhole, Server, Workflow } from 'lucide-react';
import { useBentoPlayback } from './useBentoPlayback';

export function HostingDiagram() {
  const id = `hosting-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`;
  const { container, playing, cycle } = useBentoPlayback(7000);
  return <div ref={container} className="hosting-art" data-playing={playing}>
    <svg key={cycle} viewBox="0 0 600 400" width={600} height={400} role="img" aria-labelledby={`${id}-title ${id}-desc`}>
      <title id={`${id}-title`}>Helpin on your infrastructure</title>
      <desc id={`${id}-desc`}>Support, docs, and agents run with a database, agent runtime, and file storage inside your installation.</desc>
      <rect x={24} y={30} width={552} height={344} rx={20} fill="#0d1410" stroke="#314438" strokeDasharray="4 6" />
      <g color="#82cfa5"><Server x={46} y={49} width={18} height={18} strokeWidth={1.5} /></g>
      <text x={75} y={63} className="hosting-label">YOUR INFRASTRUCTURE</text>
      <rect x={160} y={94} width={280} height={78} rx={14} fill="#172b20" stroke="#47755a" />
      <rect x={178} y={112} width={40} height={40} rx={10} fill="#244733" /><image href="/brand/helpin-icon-white.svg" x={186} y={120} width={24} height={24} />
      <text x={233} y={140} className="hosting-title">Helpin</text>
      {['M300 172 V195 H126 V228','M300 172 V228','M300 172 V195 H474 V228'].map((d,i) => <g key={d} fill="none"><path d={d} stroke="#355341" strokeWidth={1.5} /><path className="hosting-flow" d={d} pathLength={100} style={{'--hosting-delay':`${.2+i*.3}s`} as CSSProperties} /></g>)}
      {[
        { x: 52, icon: Database, title: 'Database', detail: 'PostgreSQL' },
        { x: 226, icon: Workflow, title: 'Agent Runtime', detail: 'Agent execution' },
        { x: 400, icon: HardDrive, title: 'File storage', detail: 'S3-compatible' },
      ].map(({x,icon:Icon,title,detail}) => <g key={title}>
        <rect x={x} y={228} width={148} height={84} rx={12} fill="#131e17" stroke="#2b4234" />
        <g color="#80d2a3"><Icon x={x+14} y={243} width={18} height={18} strokeWidth={1.6} /></g>
        <text x={x+14} y={281} className="hosting-node-title">{title}</text><text x={x+14} y={298} className="hosting-node-detail">{detail}</text>
      </g>)}
      <g color="#80d2a3"><LockKeyhole x={185} y={338} width={15} height={15} strokeWidth={1.6} /></g>
      <text x={210} y={351} className="hosting-detail">Your data, on your servers.</text>
    </svg>
  </div>;
}
