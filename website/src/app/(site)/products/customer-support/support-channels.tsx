import { ArrowRight, AtSign, DoorOpen, Inbox, MessageCircle, Send, type LucideIcon } from 'lucide-react';
import './support-channels.css';

const CHANNELS: { icon: LucideIcon; title: string; copy: string; href?: string }[] = [
  { icon: MessageCircle, title: 'Chat widget', copy: 'Add chat to your site or app. Signed-in customers are identified, so nobody has to explain who they are.' },
  { icon: AtSign, title: 'Email', copy: 'Keep your support address. Forward it to Helpin and reply from your verified address.' },
  { icon: Send, title: 'Telegram', copy: 'Connect a Telegram bot. New conversations go to the team inbox you choose, and you decide whether AI replies there.' },
  { icon: DoorOpen, title: 'Customer portal', copy: 'Customers sign in to open requests, follow every reply, and search their history.', href: '#support-portal' },
];

// Four channels converge on the shared inbox. Paths are drawn in a 1000×140 box that stretches with
// the row; non-scaling strokes keep the dots even at every width.
const MOBILE_PATHS = ['M250 0 C250 70, 480 60, 494 140', 'M750 0 C750 70, 520 60, 506 140'];
const PATHS = [
  'M125 0 C125 70, 470 60, 490 140',
  'M375 0 C375 70, 492 70, 496 140',
  'M625 0 C625 70, 508 70, 504 140',
  'M875 0 C875 70, 530 60, 510 140',
];

function Lines({ paths, className }: { paths: string[]; className: string }) {
  return <svg className={className} viewBox="0 0 1000 140" preserveAspectRatio="none" aria-hidden="true" focusable="false">
    {paths.map((d, index) => <g key={d}>
      <path d={d} className="sch-line" vectorEffect="non-scaling-stroke" />
      <path d={d} className="sch-flow" vectorEffect="non-scaling-stroke" style={{ animationDelay: `${index * -0.6}s` }} />
    </g>)}
  </svg>;
}

export function SupportChannels() {
  return <div className="sch">
    <ul className="sch-channels">
      {CHANNELS.map(({ icon: Icon, title, copy, href }) => <li key={title}>
        <span className="sch-icon"><Icon size={22} strokeWidth={1.6} aria-hidden="true" /></span>
        <h3>{title}</h3>
        <p>{copy}</p>
        {href ? <a className="sch-link" href={href}>See the portal <ArrowRight size={14} aria-hidden="true" /></a> : null}
      </li>)}
    </ul>
    <Lines paths={PATHS} className="sch-lines sch-lines-wide" />
    <Lines paths={MOBILE_PATHS} className="sch-lines sch-lines-narrow" />
    <div className="sch-inbox">
      <span className="sch-inbox-icon"><Inbox size={18} strokeWidth={1.7} aria-hidden="true" /></span>
      <span><b>Shared inbox</b><small>Team inboxes, routing, and the customer’s history</small></span>
    </div>
  </div>;
}
