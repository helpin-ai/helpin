'use client';

import { memo, useState } from 'react';
import { useBentoPlayback } from './useBentoPlayback';
import Link from 'next/link';
import { ArrowRight, BookOpen, Bot, Building2, ChevronRight, FileText, Kanban, MessagesSquare, Pause, Play, UserRound, Video } from 'lucide-react';
import './products-menu.css';

export const PRODUCTS = [
  { label: 'Support', description: 'Answer customer questions with full context.', href: '/products/customer-support', icon: MessagesSquare },
  { label: 'Meetings', description: 'Turn customer calls into clear next steps.', href: '/products/meetings', icon: Video },
  { label: 'Projects', description: 'Plan, build, and ship what customers need.', href: '/products/projects', icon: Kanban },
  { label: 'CRM', description: 'Manage relationships with the work behind them.', href: '/products/crm', icon: Building2 },
  { label: 'AI Agents', description: 'Specialist agents that answer, plan, and act.', href: '/products/ai-agents', icon: Bot },
  { label: 'Knowledge', description: 'Docs and guides your team and agents can trust.', href: '/products/knowledge', icon: BookOpen },
];

export function ProductLink({ item }: { item: typeof PRODUCTS[number] }) {
  const Icon = item.icon;
  return (
    <Link className="nav-product" href={item.href}>
      <span className="nav-product-art" aria-hidden="true"><Icon size={23} strokeWidth={1.5} /></span>
      <span className="nav-product-copy">
        <span className="nav-product-title">{item.label}<ChevronRight size={14} aria-hidden="true" /></span>
        <span className="nav-product-value">{item.description}</span>
      </span>
    </Link>
  );
}

export function AskAgentMenuCard() {
  const [paused, setPaused] = useState(false);
  const { container, playing, cycle } = useBentoPlayback(7000, !paused);
  return <div className="nav-agent-card" ref={container} data-playing={playing}>
    <Link href="/products/ai-agents#agent-ask" className="nav-products-agent-link">
      <div className="nav-agent-motion" key={cycle}>
        <svg viewBox="0 0 300 112" role="img" aria-label="Conversations, documentation, and customer records connect to Ask Agent.">
          {[['M48 20H100C135 20 138 56 180 56H222', MessagesSquare, 4], ['M48 56H222', FileText, 40], ['M48 92H100C135 92 138 56 180 56H222', UserRound, 76]].map(([path, Icon, y], index) => {
            const SourceIcon = Icon as typeof MessagesSquare;
            return <g key={index} className={`nav-agent-source nav-agent-source-${index}`}>
              <path className="nav-agent-track" d={path as string}/>
              <path className="nav-agent-signal" d={path as string} pathLength="100"/>
              <rect className="nav-agent-source-tile" x="16" y={y as number} width="32" height="32" rx="9"/>
              <SourceIcon x="24" y={(y as number)+8} width="16" height="16" strokeWidth="1.6"/>
            </g>;
          })}
          <circle className="nav-agent-received-ring" cx="249" cy="56" r="36"/>
          <rect className="nav-agent-hub" x="222" y="29" width="54" height="54" rx="15"/>
          <image href="/brand/helpin-icon-white.svg" x="234" y="41" width="30" height="30"/>
          <g className="nav-agent-ready"><circle cx="274" cy="80" r="9"/><path d="m270 80 3 3 5-6"/></g>
        </svg>
      </div>
      <span className="nav-products-agent-copy"><b>Ask Agent</b><span>Your workspace assistant. Ask questions, investigate issues, and coordinate specialist agents using customer history.</span></span>
      <span className="nav-products-agent-cta">See Ask Agent in action<ArrowRight size={15} aria-hidden="true" /></span>
    </Link>
    <button className="nav-agent-playback" type="button" aria-label={`${paused ? 'Play' : 'Pause'} Ask Agent menu animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12}/> : <Pause size={12}/>}</button>
  </div>;
}

// The menu stays mounted and is only shown or hidden, and it takes no props, so the nav's
// open/close state changes never need to re-render it.
export const ProductsMenu = memo(function ProductsMenu() {
  return <>
    <div className="nav-products-main">
      <div className="nav-products-intro"><p className="nav-products-title">Everything you need to build exceptional customer experiences</p></div>
      <div className="nav-products-grid">{PRODUCTS.map(item => <ProductLink item={item} key={item.label} />)}</div>
    </div>
    <AskAgentMenuCard/>
  </>;
});
