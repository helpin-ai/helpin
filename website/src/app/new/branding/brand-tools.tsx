'use client';

import { useState } from 'react';
import { Check, Copy } from 'lucide-react';
import palette from '../../../../public/brand/kit/helpin-palette.json';

const CORE_COLORS = ['Forest', 'Helpin green', 'Sage', 'Ink', 'White', 'Soft surface'];

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
    <div className="brand-palette">
      {CORE_COLORS.map(name => {
        const { hex } = palette.colors.find(color => color.name === name)!;
        return <article className="brand-color" key={hex}>
          <div className="brand-swatch" style={{ background: hex }} aria-hidden="true" />
          <div className="brand-color-info"><h3>{name}</h3><div className="brand-color-value"><code>{hex}</code><button type="button" onClick={() => copyColor(name, hex)} aria-label={`Copy ${name} hex ${hex}`}>{copied === hex ? <Check size={14} aria-hidden="true" /> : <Copy size={14} aria-hidden="true" />}</button></div></div>
        </article>;
      })}
    </div>
    <p className="brand-copy-status" role="status">{status || 'Copy a HEX value using the icon beside it.'}</p>
  </>;
}
