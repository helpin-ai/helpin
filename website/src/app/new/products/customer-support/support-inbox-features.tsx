'use client';

import { useState } from 'react';
import { ArrowDown, Check, Inbox, ListFilter, Mail, Pause, Play, ReceiptText, Route, Tag } from 'lucide-react';
import { useBentoPlayback } from '../../_components/useBentoPlayback';
import './support-inbox-features.css';

const FEATURES = [
  {
    id: 'email', label: 'Email forwarding', icon: Mail,
    title: 'Keep your support address.',
    copy: 'Forward email into a shared or team inbox. Verify the setup, configure your reply address, and handle email alongside live chat.',
    description: 'Email travels from support@orbitdesk.example into the OrbitDesk shared inbox. The CSV export request arrives, and the forwarding setup is verified.',
  },
  {
    id: 'routing', label: 'Routing & assignment', icon: Route,
    title: 'Give every question an owner.',
    copy: 'Route conversations with rules or AI triage. Send them to the right team, then assign an owner manually or with round-robin assignment.',
    description: 'An invoice request matches a routing rule. The route leads to the Billing inbox, where round-robin assignment selects Sam Rivera.',
  },
  {
    id: 'organize', label: 'Tags & saved views', icon: ListFilter,
    title: 'Make the work easy to find.',
    copy: 'Tag conversations by topic or product area. Combine filters into personal or shared views that keep the right work in front of your team.',
    description: 'A CSV export request receives Bug and Exports tags. A saved view filters open export issues, keeping two matching requests and filtering out an unrelated billing question.',
  },
] as const;

type Feature = typeof FEATURES[number];

function EmailWorkflow() {
  return <>
    <div className="ops-source"><span>YOUR SUPPORT ADDRESS</span><div><Mail size={17} /><strong>support@orbitdesk.example</strong></div></div>
    <div className="ops-delivery"><span className="ops-delivery-line" /><span className="ops-envelope"><Mail size={15} /></span><span className="ops-delivery-caption">Email forwarding</span><ArrowDown size={13} /></div>
    <div className="ops-mailbox">
      <div className="ops-mailbox-heading"><Inbox size={16} /><strong>Shared inbox</strong><span>OrbitDesk</span></div>
      <div className="ops-mail-row ops-arrival"><span className="ops-unread" /><div><strong>CSV export stops early</strong><span>Maya Chen · Northstar Labs</span></div><Mail size={13} /></div>
      <div className="ops-mail-row ops-existing"><span className="ops-unread" /><div><strong>Update our billing details</strong><span>Alex Morgan · Email</span></div><Mail size={13} /></div>
    </div>
    <div className="ops-complete ops-delivered"><Check size={13} /><span>Forwarding verified. Ready for your team.</span></div>
  </>;
}

function RoutingWorkflow() {
  return <>
    <div className="ops-route-request"><ReceiptText size={18} /><div><span>INCOMING REQUEST</span><strong>Update the name on our invoice</strong></div></div>
    <div className="ops-rule"><span>Message contains</span><span className="ops-keyword">invoice</span><Check size={13} /></div>
    <svg className="ops-branches" viewBox="0 0 300 58" preserveAspectRatio="none" aria-hidden="true"><path d="M150 0 V18 Q150 24 144 24 H56 Q50 24 50 30 V58 M150 0 V58 M150 24 H244 Q250 24 250 30 V58" /><path className="ops-selected-route" d="M150 0 V58" pathLength="1" /><circle className="ops-route-dot" cx="150" cy="54" r="3" /></svg>
    <div className="ops-destinations"><span>Support</span><span className="ops-billing"><Inbox size={13} />Billing</span><span>Technical</span></div>
    <div className="ops-owner-link"><span /></div>
    <div className="ops-owner"><img src="/new/avatars/sam.webp" width={30} height={30} alt="" /><div><strong>Sam Rivera</strong><span>Assigned · Round-robin</span></div><span className="ops-owner-check"><Check size={14} /></span></div>
    <div className="ops-complete"><span>A clear destination. A teammate responsible.</span></div>
  </>;
}

function OrganizeWorkflow() {
  return <>
    <div className="ops-tagged-request"><span>CONVERSATION / 142</span><strong>CSV export stops early</strong><div className="ops-tags"><span className="ops-tag-bug"><Tag size={11} />Bug</span><span className="ops-tag-exports"><Tag size={11} />Exports</span></div></div>
    <div className="ops-saved-view"><div><ListFilter size={15} /><strong>Open export issues</strong><span>Team view</span></div><div className="ops-filters"><span>Status: Open</span><span>Tag: Exports</span></div></div>
    <div className="ops-filter-results">
      <div className="ops-filter-row ops-match"><Check size={12} /><span>CSV export stops early</span><small>Bug</small></div>
      <div className="ops-filter-row ops-filtered"><ReceiptText size={12} /><span>Update billing details</span><small>Billing</small></div>
      <div className="ops-filter-row ops-match ops-match-last"><Check size={12} /><span>Missing date column</span><small>Bug</small></div>
    </div>
    <div className="ops-complete ops-view-ready"><Check size={13} /><span>Your view. Just the work that belongs.</span></div>
  </>;
}

function WorkflowCard({ feature, index }: { feature: Feature; index: number }) {
  const { container, playing, cycle } = useBentoPlayback(9000);
  const [paused, setPaused] = useState(false);
  const Icon = feature.icon;
  return <article className="ops-card" aria-labelledby={`ops-title-${feature.id}`}>
    <div ref={container} className={`ops-art ops-art-${feature.id}`} data-playing={playing && !paused}>
      <div className="ops-art-toolbar"><span><span className="ops-workspace-mark">O</span>OrbitDesk</span><button type="button" aria-label={`${paused ? 'Play' : 'Pause'} ${feature.label.toLowerCase()} animation`} aria-pressed={paused} onClick={() => setPaused(value => !value)}>{paused ? <Play size={12} aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}</button></div>
      <div key={cycle} className="ops-scene" role="img" aria-label={feature.description}><div aria-hidden="true">{feature.id === 'email' ? <EmailWorkflow /> : feature.id === 'routing' ? <RoutingWorkflow /> : <OrganizeWorkflow />}</div></div>
    </div>
    <div className="ops-card-copy"><span className="ops-card-label"><Icon size={14} aria-hidden="true" />{feature.label}<span>0{index + 1}</span></span><h3 id={`ops-title-${feature.id}`}>{feature.title}</h3><p>{feature.copy}</p></div>
  </article>;
}

export function SupportInboxFeatures() {
  return <><div className="ops-cards">{FEATURES.map((feature, index) => <WorkflowCard key={feature.id} feature={feature} index={index} />)}</div><p className="ops-plan-note">Round-robin assignment is available on eligible plans.</p></>;
}
