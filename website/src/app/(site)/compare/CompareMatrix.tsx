'use client';

import { useEffect, useId, useState } from 'react';
import Link from 'next/link';
import { ArrowRight, Check, ChevronLeft, ChevronRight, Minus, X } from 'lucide-react';
import { MATRIX, type MatrixCell } from './matrix-data';
import type { Status } from './compare-data';
import { SIGNUP_URL } from '../_components/ui';

type Tool = { slug: string; name: string; category: string };

const ICON: Record<Status, typeof Check> = { yes: Check, partial: Minus, no: X };
const LABEL: Record<Status, string> = { yes: 'Yes', partial: 'Partly', no: 'No' };

function Cell({ cell }: { cell: MatrixCell }) {
  const Icon = cell.status ? ICON[cell.status] : null;
  return (
    <span className="cmp-mx-cell">
      {Icon && cell.status ? <span className="cmp-status-icon" data-status={cell.status}><Icon size={13} strokeWidth={2.6} aria-hidden="true" />{cell.text ? null : <span className="sr-only">{LABEL[cell.status]}</span>}</span> : null}
      {cell.text ? <span>{cell.text}</span> : null}
    </span>
  );
}

// Helpin stays pinned while the other tools page past it, a few at a time. Every column
// is in the server HTML; paging only hides the ones outside the current window.
export function CompareMatrix({ tools }: { tools: Tool[] }) {
  const id = useId();
  const [visible, setVisible] = useState(3);
  const [start, setStart] = useState(0);

  useEffect(() => {
    const wide = window.matchMedia('(min-width: 1100px)');
    const medium = window.matchMedia('(min-width: 720px)');
    const update = () => setVisible(wide.matches ? 3 : medium.matches ? 2 : 1);
    update();
    wide.addEventListener('change', update);
    medium.addEventListener('change', update);
    return () => { wide.removeEventListener('change', update); medium.removeEventListener('change', update); };
  }, []);

  const maxStart = Math.max(0, tools.length - visible);
  const first = Math.min(start, maxStart);
  const shown = (index: number) => index >= first && index < first + visible;
  const range = `${first + 1}–${Math.min(first + visible, tools.length)} of ${tools.length}`;

  return (
    <div className="cmp-mx" style={{ '--mx-cols': visible } as React.CSSProperties}>
      <div className="cmp-mx-pager">
        <span>Comparing Helpin with {visible === 1 ? tools[first].name : `tools ${range}`}</span>
        <div>
          <button type="button" aria-controls={id} aria-label="Show previous tools" disabled={first === 0} onClick={() => setStart(Math.max(0, first - visible))}><ChevronLeft size={16} aria-hidden="true" /></button>
          <button type="button" aria-controls={id} aria-label="Show more tools" disabled={first >= maxStart} onClick={() => setStart(Math.min(maxStart, first + visible))}><ChevronRight size={16} aria-hidden="true" /></button>
        </div>
      </div>
      <table id={id} className="cmp-mx-table">
        <caption className="sr-only">Helpin compared with {tools.map(tool => tool.name).join(', ')}. A check means included, a dash means partly or on some plans, and a cross means not available.</caption>
        <thead>
          <tr>
            <td className="cmp-mx-corner"><span>A check means included; a dash, partly or on some plans.</span></td>
            <th scope="col" className="cmp-mx-helpin">
              <img className="cmp-mark" src="/brand/helpin-icon-ink.svg" width={30} height={30} alt="" />
              <strong>Helpin</strong>
              <a className="btn btn-primary" href={SIGNUP_URL}>Start free</a>
            </th>
            {tools.map((tool, index) => (
              <th key={tool.slug} scope="col" hidden={!shown(index)}>
                <strong>{tool.name}</strong>
                <Link className="btn btn-secondary" href={`/compare/${tool.slug}`}><span className="cmp-mx-long">Full comparison</span><span className="cmp-mx-short" aria-hidden="true">Compare</span><ArrowRight size={13} aria-hidden="true" /></Link>
              </th>
            ))}
          </tr>
        </thead>
        {MATRIX.map(group => (
          <tbody key={group.group}>
            <tr className="cmp-mx-group"><th scope="rowgroup" colSpan={2 + Math.min(visible, tools.length)}>{group.group}</th></tr>
            {group.rows.map(row => (
              <tr key={row.label}>
                <th scope="row">{row.label}</th>
                <td className="cmp-mx-helpin"><Cell cell={row.helpin} /></td>
                {tools.map((tool, index) => <td key={tool.slug} hidden={!shown(index)}><Cell cell={row.tools[tool.slug]} /></td>)}
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}
