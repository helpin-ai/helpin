import Link from 'next/link';
import type { CSSProperties, ReactNode } from 'react';
import { AtSign, BookOpen, Bug, Check, Clock3, GitPullRequest, Hash, Inbox, ListChecks, MessageCircle, MessageSquare, Plug, ReceiptText, Reply, Send, Terminal, type LucideIcon } from 'lucide-react';
import { LinkArrow } from '../ui';
import './desk-stack.css';

// The support desk as four cards that stack while the page scrolls. Each scene is a still
// from the same fictional story (OrbitDesk supporting Northstar Labs); the copy beside it
// carries the claim, so the scenes stay static and need no JavaScript.

const MAYA = '/new/avatars/maya.webp';

function Row({ who, initials, photo, team, text, children }: { who: string; initials?: string; photo?: string; team?: string; text: string; children?: ReactNode }) {
  return <div className="ds-row">
    {photo ? <img className="ds-avatar" src={photo} width={28} height={28} alt="" /> : <span className="ds-avatar ds-initials">{initials}</span>}
    <span className="ds-row-main"><b>{who}</b><small>{team ? <><i>{team}</i> · </> : null}{text}</small></span>
    {children}
  </div>;
}

function Chip({ icon: Icon, tone, children }: { icon?: LucideIcon; tone?: 'soon' | 'on'; children: ReactNode }) {
  return <span className="ds-chip" data-tone={tone}>{Icon ? <Icon size={11} /> : null}{children}</span>;
}

function InboxScene() {
  return <div className="ds-panel">
    <div className="ds-head"><Inbox size={14} />Inbox<span>4 open</span></div>
    <Row who="Maya Chen" photo={MAYA} team="Technical" text="CSV export stops at 10,000 rows"><Chip icon={Clock3} tone="soon">9m</Chip><Chip icon={MessageCircle}>Chat</Chip></Row>
    <Row who="Alex Morgan" initials="AM" team="Billing" text="Invoice for last month"><Chip icon={AtSign}>Email</Chip></Row>
    <Row who="Noah Williams" initials="NW" team="Technical" text="Webhook deliveries are retrying"><Chip icon={Hash}>Slack</Chip></Row>
    <Row who="Lena Fischer" initials="LF" team="Billing" text="Can we add two more teammates?"><Chip icon={Send}>Telegram</Chip></Row>
  </div>;
}

function AnswerScene() {
  return <div className="ds-chat">
    <p className="ds-bubble ds-bubble-customer">How do I export only the contacts I selected?</p>
    <div className="ds-bubble"><b>Echo</b><p>Select the contacts you need, open Actions, then choose Export selected contacts.</p><Chip icon={BookOpen}>Export guide</Chip></div>
    <div className="ds-bubble"><b>Echo <small>Follow-up</small></b><p>Did the export include everyone you selected?</p></div>
  </div>;
}

function AccountScene() {
  const tools: [LucideIcon, string, string][] = [
    [ReceiptText, 'Billing', 'Northstar Labs · Scale plan'],
    [Terminal, 'Error logs', 'Export stopped at 10,000 of 18,400 rows'],
    [ListChecks, 'Issue tracker', 'EXP-142 · Fix in review'],
  ];
  return <div className="ds-panel">
    <div className="ds-head"><Plug size={14} />Connected tools<span>Checked by Echo</span></div>
    {tools.map(([Icon, label, detail]) => <div className="ds-row" key={label}>
      <span className="ds-avatar ds-tool"><Icon size={14} /></span>
      <span className="ds-row-main"><b>{label}</b><small>{detail}</small></span>
      <span className="ds-tick"><Check size={12} /></span>
    </div>)}
  </div>;
}

function FixScene() {
  return <>
    <div className="ds-panel"><Row who="Maya Chen" photo={MAYA} text="CSV export stops at 10,000 rows"><Chip icon={ListChecks} tone="on">EXP-142</Chip></Row></div>
    <div className="ds-panel">
      <div className="ds-head"><ListChecks size={14} />EXP-142<span>Linked to Maya’s conversation</span></div>
      <div className="ds-row"><span className="ds-avatar ds-tool"><GitPullRequest size={14} /></span><span className="ds-row-main"><b>Fix export pagination</b><small>Proposed by Forge · Reviewed by Lens</small></span><Chip tone="soon">Needs approval</Chip></div>
      <div className="ds-row"><span className="ds-avatar ds-tool"><MessageSquare size={14} /></span><span className="ds-row-main"><b>Update for Maya</b><small>Waiting for the fix</small></span><Chip>Draft</Chip></div>
    </div>
  </>;
}

const CARDS: { icon: LucideIcon; title: string; copy: string; scene: ReactNode; label: string; link?: { href: string; text: string } }[] = [
  {
    icon: Inbox, title: 'Brings every channel together',
    copy: 'Chat, email, Slack, and Telegram land in one shared inbox. The AI routes each conversation to the right team, and Helpin warns before a response target is missed.',
    scene: <InboxScene />, label: 'An inbox with four open conversations, one each from chat, email, Slack, and Telegram. Each is routed to the Technical or Billing team, and the first has nine minutes left on its response target.',
  },
  {
    icon: Reply, title: 'Answers from your docs, then follows up',
    copy: 'The Echo agent answers from your docs, checks whether the customer still needs help, and hands over with what it found.',
    scene: <AnswerScene />, label: 'Echo answers a question about exporting selected contacts from the export guide, then follows up to ask whether it worked.',
  },
  {
    icon: Plug, title: 'Checks the account before replying',
    copy: 'Connect tools for billing, error logs, or issue tracking so agents can check the facts behind a question. You choose what they can use and change.',
    scene: <AccountScene />, label: 'Echo checks three connected tools: billing, error logs, and the issue tracker.',
  },
  {
    icon: Bug, title: 'Turns a bug report into a fix',
    copy: 'AI agents can plan, code, and review fixes using the customer’s report and the linked task. Your team checks the proposed changes before approving them.',
    scene: <FixScene />, label: 'Maya’s conversation is linked to task EXP-142. A fix proposed by Forge and reviewed by Lens waits for approval, and an update for Maya is still a draft.',
    link: { href: '/products/ai-agents', text: 'Explore AI agents' },
  },
];

export function DeskStack() {
  return <ol className="dstack">
    {CARDS.map(({ icon: Icon, title, copy, scene, label, link }, index) => <li key={title} className="dstack-card" style={{ '--i': index } as CSSProperties}>
      <div className="dstack-copy">
        <div className="dstack-top"><span className="developer-feature-icon"><Icon size={22} aria-hidden="true" /></span><span className="dstack-count" aria-hidden="true">{index + 1} / {CARDS.length}</span></div>
        <h3>{title}</h3>
        <p>{copy}</p>
        {link ? <Link prefetch={false} className="inline-link" href={link.href}>{link.text}<LinkArrow /></Link> : null}
      </div>
      <div className="dstack-art" role="img" aria-label={label}><div aria-hidden="true">{scene}</div></div>
    </li>)}
  </ol>;
}
