import { BookOpen, Bot, CalendarDays, ChevronRight, Kanban, MessagesSquare, Users } from 'lucide-react';
import Link from 'next/link';
import { GridFlow } from '../new/_components/GridFlow';
import { CtaRow, DEMO_URL } from '../new/_components/ui';
import { ConnectedWorkspace } from '../new/_components/ConnectedWorkspace';
import { PricingPlans } from './PricingPlans';
import { AIUsage, CloudComparison, HostingOptions, PricingFAQ, WorkflowComparison } from './PricingSections';

const PRODUCTS = [
  { label: 'Support', Icon: MessagesSquare }, { label: 'Projects', Icon: Kanban },
  { label: 'CRM', Icon: Users }, { label: 'Meetings', Icon: CalendarDays },
  { label: 'Knowledge', Icon: BookOpen }, { label: 'AI agents', Icon: Bot },
];

export default function PricingPage() {
  return <div className="pricing-page">
    <section className="pricing-hero"><GridFlow /><div className="wrap"><div className="pricing-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Pricing</span></div><span className="eyebrow">Helpin pricing</span><h1>One platform.<br /><span>Your whole team, connected.</span></h1><p className="lede">Bring support, projects, CRM, meetings, docs, and AI agents together. Choose Helpin Cloud for managed hosting, or run the open-source product on your own infrastructure.</p><nav className="pricing-jump" aria-label="Pricing options"><a href="#cloud-plans">Cloud plans ↓</a><a href="#self-hosted">Self-hosting ↓</a><a href="#enterprise">Enterprise ↓</a></nav><ul className="pricing-products" aria-label="Included product modules">{PRODUCTS.map(({label,Icon}) => <li key={label}><Icon size={18} aria-hidden="true" />{label}</li>)}</ul><p className="pricing-note">The customer’s history belongs with the people doing the work.</p></div></section>
    <PricingPlans />
    <HostingOptions />
    <WorkflowComparison />
    <AIUsage />
    <CloudComparison />
    <PricingFAQ />
    <section className="final-cta final-cta-connected" aria-labelledby="pricing-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Start with a real customer request</span><h2 id="pricing-final-title">See what changes<br />when the history stays connected.</h2><p className="lede">Follow a question into the work it creates. See what your team and agents can do when the earlier conversation, current task, and next step are in view.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="pricing-note">Try Cloud without a card, or <a href="/new/self-hosting">explore self-hosting</a>.</p></div></div></section>
  </div>;
}
