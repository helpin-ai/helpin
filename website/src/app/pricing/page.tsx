import { BookOpen, Bot, CalendarDays, Kanban, MessagesSquare, Users } from 'lucide-react';
import { CtaRow, DEMO_URL } from '../new/_components/ui';
import { PricingPlans } from './PricingPlans';
import { AIUsage, CloudComparison, HostingOptions, PricingFAQ, WorkflowComparison } from './PricingSections';

const PRODUCTS = [
  { label: 'Support', Icon: MessagesSquare }, { label: 'Projects', Icon: Kanban },
  { label: 'CRM', Icon: Users }, { label: 'Meetings', Icon: CalendarDays },
  { label: 'Knowledge', Icon: BookOpen }, { label: 'AI agents', Icon: Bot },
];

export default function PricingPage() {
  return <div className="pricing-page">
    <section className="pricing-hero"><div className="wrap"><span className="eyebrow">Open source or Helpin Cloud</span><h1>One product.<br />Choose how you run it.</h1><p className="lede">Self-host the open-source product, or let us handle the hosting. Cloud plans include unlimited teammates and a monthly AI allowance.</p><nav className="pricing-jump" aria-label="Pricing options"><a href="#cloud-plans">Cloud plans ↓</a><a href="#self-hosted">Self-hosting & Enterprise ↓</a></nav><ul className="pricing-products" aria-label="Included product modules">{PRODUCTS.map(({label,Icon}) => <li key={label}><Icon size={18} aria-hidden="true" />{label}</li>)}</ul></div></section>
    <PricingPlans />
    <HostingOptions />
    <WorkflowComparison />
    <AIUsage />
    <CloudComparison />
    <PricingFAQ />
    <section className="pricing-closing"><div className="wrap"><span className="eyebrow">Start with your team’s next customer request</span><h2>Try Helpin Cloud for 14 days.</h2><p>Explore the product on a Growth trial, then choose Starter or Growth.</p><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="pricing-note">No card required · Or <a href="/new/self-hosting">self-host the open-source product</a></p></div></section>
  </div>;
}
