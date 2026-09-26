'use client';

import { useState, type CSSProperties } from 'react';
import { BookOpen, Bot, CircleDollarSign, FolderKanban, MessageSquare, Pause, Play, Users, type LucideIcon } from 'lucide-react';
import { useBentoPlayback } from './useBentoPlayback';
import './connected-workspace.css';

// Paths run from each outer node into the Helpin hub so every pulse travels inward.
const NODES: { label: string; icon: LucideIcon; x: number; y: number; width: number; path: string }[] = [
  { label: 'Conversations', icon: MessageSquare, x: 108, y: 184, width: 158, path: 'M187 184 H232 Q248 184 248 168 V134 Q248 118 264 118 H608' },
  { label: 'Knowledge', icon: BookOpen, x: 142, y: 338, width: 136, path: 'M210 338 H228 Q244 338 244 322 V190 Q244 174 260 174 H504 Q520 174 520 158 V150 Q520 134 536 134 H608' },
  { label: 'Customers', icon: Users, x: 108, y: 490, width: 136, path: 'M176 490 H202 Q218 490 218 474 V218 Q218 202 234 202 H542 Q558 202 558 186 V168 Q558 152 574 152 H608' },
  { label: 'AI agents', icon: Bot, x: 1172, y: 184, width: 128, path: 'M1108 184 H1048 Q1032 184 1032 168 V134 Q1032 118 1016 118 H672' },
  { label: 'Projects', icon: FolderKanban, x: 1138, y: 338, width: 120, path: 'M1078 338 H1052 Q1036 338 1036 322 V190 Q1036 174 1020 174 H776 Q760 174 760 158 V150 Q760 134 744 134 H672' },
  { label: 'CRM', icon: CircleDollarSign, x: 1172, y: 490, width: 100, path: 'M1122 490 H1078 Q1062 490 1062 474 V218 Q1062 202 1046 202 H738 Q722 202 722 186 V168 Q722 152 706 152 H672' },
];

function Node({ label, icon: Icon, x, y, width }: { label: string; icon: LucideIcon; x: number; y: number; width: number }) {
  return <g transform={`translate(${x - width / 2} ${y - 22})`}><rect width={width} height={44} rx={10} className="connection-node" /><Icon x={14} y={14} width={16} height={16} strokeWidth={1.5} className="connection-node-icon" /><text x={40} y={27} className="connection-node-label">{label}</text></g>;
}

function Hub({ x, y, size = 64 }: { x: number; y: number; size?: number }) {
  return <g transform={`translate(${x - size / 2} ${y - size / 2})`}><rect x={-7} y={-7} width={size + 14} height={size + 14} rx={20} className="connection-hub-halo" /><rect width={size} height={size} rx={15} className="connection-hub" /><image href="/brand/helpin-icon-white.svg" x={size * .23} y={size * .23} width={size * .54} height={size * .54} /></g>;
}

export function ConnectedWorkspace() {
  const { container, playing, cycle } = useBentoPlayback(9000);
  const [paused, setPaused] = useState(false);
  return <div ref={container} className="connected-workspace" data-playing={playing && !paused}>
    <div className="connection-art" key={cycle} aria-hidden="true">
      <svg className="connection-desktop" viewBox="0 0 1280 640" width={1280} height={640} fill="none">
        {NODES.map(({ label, path }, index) => <g key={label} style={{ '--connection-delay': `${index * .22}s` } as CSSProperties}><path d={path} className="connection-line" /><path d={path} pathLength={100} className="connection-pulse" /></g>)}
        {NODES.map(node => <Node key={node.label} {...node} />)}
        <Hub x={640} y={134} />
      </svg>
      <svg className="connection-mobile" viewBox="0 0 360 184" width={360} height={184} fill="none">
        {['M96 37 H132 Q148 37 148 53 V76 H156', 'M96 145 H132 Q148 145 148 129 V108 H156', 'M264 37 H228 Q212 37 212 53 V76 H204', 'M264 145 H228 Q212 145 212 129 V108 H204'].map((path, index) => <g key={path} style={{ '--connection-delay': `${index * .3}s` } as CSSProperties}><path d={path} className="connection-line" /><path d={path} pathLength={100} className="connection-pulse" /></g>)}
        <Node label="Support" icon={MessageSquare} x={64} y={37} width={116} /><Node label="Knowledge" icon={BookOpen} x={71} y={145} width={130} />
        <Node label="Agents" icon={Bot} x={302} y={37} width={104} /><Node label="Projects" icon={FolderKanban} x={296} y={145} width={116} />
        <Hub x={180} y={92} size={48} />
      </svg>
    </div>
    <button className="connection-toggle" type="button" aria-label={`${paused ? 'Play' : 'Pause'} connectivity animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={13} aria-hidden="true" /> : <Pause size={13} aria-hidden="true" />}</button>
  </div>;
}
