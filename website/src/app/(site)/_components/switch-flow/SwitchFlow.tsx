import { BookOpen, MessagesSquare } from 'lucide-react';
import './switch-flow.css';

// What moves into Helpin, and from where. Every source sends its help center;
// Intercom also sends past conversations, so it alone draws a second line.
// Sources follow Settings → Import (frontend/src/components/settings/ImportTab.tsx).
// Other products appear by name only, never with their logos.
// Narrow screens have no room for lines, so there each row lists its own sources instead.

const SOURCES = ['Intercom', 'Zendesk', 'Help Scout', 'Freshdesk', 'Crisp', 'HubSpot', 'Mintlify or GitBook'];

// Geometry shared with switch-flow.css: sources are 44px tall with a 10px gap (54px pitch,
// 368px in all); the card's rows are centred at 131px (conversations) and 289px (help center).
const HEIGHT = 368;
const center = (index: number) => 22 + index * 54;
const curve = (from: number, to: number) => `M0 ${from} C52 ${from}, 48 ${to}, 100 ${to}`;

export function SwitchFlow() {
  return <figure className="swf" aria-label="A help center can be copied into Helpin from Intercom, Zendesk, Help Scout, Freshdesk, Crisp, HubSpot, or a docs site such as Mintlify or GitBook. From Intercom, past conversations move too.">
    <ul className="swf-sources" aria-hidden="true">
      {SOURCES.map((name, index) => <li key={name} data-lead={index === 0 || undefined}>{name}</li>)}
    </ul>
    <svg className="swf-lines" viewBox={`0 0 100 ${HEIGHT}`} preserveAspectRatio="none" aria-hidden="true" focusable="false">
      {SOURCES.map((name, index) => {
        const d = curve(center(index) + (index === 0 ? 6 : 0), 289 + (index - 3) * 5);
        return <g key={name}>
          <path d={d} className="swf-line" vectorEffect="non-scaling-stroke" />
          <path d={d} className="swf-flow" vectorEffect="non-scaling-stroke" style={{ animationDelay: `${index * -0.45}s` }} />
        </g>;
      })}
      <path d={curve(center(0) - 6, 131)} className="swf-line swf-line-lead" vectorEffect="non-scaling-stroke" />
      <path d={curve(center(0) - 6, 131)} className="swf-flow swf-flow-lead" vectorEffect="non-scaling-stroke" />
    </svg>
    <div className="swf-card" aria-hidden="true">
      <div className="swf-card-head"><span className="swf-mark"><img src="/brand/helpin-icon-white.svg" width={14} height={14} alt="" /></span>Your Helpin workspace</div>
      <div className="swf-row">
        <span className="swf-row-icon"><MessagesSquare size={18} /></span>
        <span><b>Past conversations</b><small>Replies, internal notes, and tags, from the date you choose.</small><em data-lead="true">From Intercom</em><span className="swf-from"><i data-lead="true">Intercom</i></span></span>
      </div>
      <div className="swf-row">
        <span className="swf-row-icon"><BookOpen size={18} /></span>
        <span><b>Help center</b><small>Articles and collections, copied from the public address.</small><em>From any of these</em><span className="swf-from">{SOURCES.map((name, index) => <i key={name} data-lead={index === 0 || undefined}>{name}</i>)}</span></span>
      </div>
    </div>
  </figure>;
}
