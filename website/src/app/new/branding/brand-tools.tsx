'use client';

import { useState } from 'react';
import { Check, Copy, MessageSquare, FileText, Users, Pause, Play } from 'lucide-react';
import { useBentoPlayback } from '../_components/useBentoPlayback';
import palette from '../../../../public/brand/kit/helpin-palette.json';

export function ColorPalette() {
  const [copied, setCopied] = useState('');
  const [status, setStatus] = useState('');
  async function copyColor(name: string, hex: string) {
    try {
      await navigator.clipboard.writeText(hex);
      setCopied(hex);
      setStatus(`${name}: ${hex} copied to clipboard.`);
    } catch {
      setCopied('');
      setStatus(`Copy unavailable. Select and copy ${hex} below ${name}.`);
    }
  }
  return <>
    {(['primary', 'supporting'] as const).map(group => <div key={group} className={`brand-palette brand-palette-${group}`}>
      {palette.colors.filter(color => color.group === group).map(({ name, hex, role }) => {
        const rgb = hex.slice(1).match(/.{2}/g)!.map(part => parseInt(part, 16)).join(', ');
        return <article className="brand-color" key={hex}>
          <div className="brand-swatch" style={{ background: hex }} aria-hidden="true" />
          <div className="brand-color-info"><h3>{name}</h3><div className="brand-color-value"><code>{hex}</code><button type="button" onClick={() => copyColor(name, hex)} aria-label={`Copy ${name} hex ${hex}`}>{copied === hex ? <Check size={15} /> : <Copy size={15} />}</button></div><span className="brand-rgb">RGB {rgb}</span><p>{role}</p></div>
        </article>;
      })}
    </div>)}
    <p className="brand-copy-status" role="status">{status || 'Select the copy icon to copy a HEX value.'}</p>
  </>;
}

export function BrandMotion() {
  const { container, playing, cycle } = useBentoPlayback(8000);
  const [paused, setPaused] = useState(false);
  return <div ref={container} className="brand-motion" data-playing={playing && !paused}>
    <svg key={cycle} viewBox="0 0 480 250" role="img" aria-label="Conversations, knowledge, and people connect to Helpin, sharing one customer context.">
      {['M114 58 H180 Q200 58 200 78 V105 Q200 125 220 125 H298', 'M114 125 H298', 'M114 192 H180 Q200 192 200 172 V145 Q200 125 220 125 H298'].map((d, i) => <g key={d}><path d={d} className="brand-motion-line" /><path d={d} pathLength={100} className="brand-motion-pulse" style={{ animationDelay: `${i * .35}s` }} /></g>)}
      {[MessageSquare, FileText, Users].map((Icon, i) => <g key={i} transform={`translate(66 ${34 + i * 67})`}><rect width={48} height={48} rx={12} fill="#fff" stroke="#CFD6D2" /><Icon x={14} y={14} width={20} height={20} color="#0F7A50" strokeWidth={1.5} /></g>)}
      <rect x={286} y={81} width={88} height={88} rx={24} fill="#E7F4ED" /><rect x={298} y={93} width={64} height={64} rx={16} fill="#081B16" /><image href="/brand/helpin-icon-white.svg" x={313} y={108} width={34} height={34} />
    </svg>
    <div className="brand-motion-bottom"><span>Shared context. Clear direction.</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} brand motion example`} aria-pressed={paused} onClick={() => setPaused(!paused)}>{paused ? <Play size={15} /> : <Pause size={15} />}</button></div>
  </div>;
}
