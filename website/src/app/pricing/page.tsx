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
    <section className="pricing-hero"><GridFlow /><div className="wrap"><div className="pricing-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Pricing</span></div><span className="eyebrow">Open source or Helpin Cloud</span><h1>One product.<br /><span>Choose how you run it.</span></h1><p className="lede">Self-host the open-source product, or let us handle the hosting. Cloud plans include unlimited teammates and a monthly AI allowance.</p><nav className="pricing-jump" aria-label="Pricing options"><a href="#cloud-plans">Cloud plans ↓</a><a href="#self-hosted">Self-hosting & Enterprise ↓</a></nav><ul className="pricing-products" aria-label="Included product modules">{PRODUCTS.map(({label,Icon}) => <li key={label}><Icon size={18} aria-hidden="true" />{label}</li>)}</ul></div></section>
    <PricingPlans />
    <HostingOptions />
    <WorkflowComparison />
    <AIUsage />
    <CloudComparison />
    <PricingFAQ />
    <section className="final-cta final-cta-connected" aria-labelledby="pricing-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Start with your team’s next customer request</span><h2 id="pricing-final-title">Try Helpin Cloud for 14 days.</h2><p className="lede">Explore the product on a Growth trial, then choose Starter or Growth.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="pricing-note">No card required · Or <a href="/new/self-hosting">self-host the open-source product</a></p></div></div></section>
  </div>;
}
