import { HeroVortex } from '../new/_components/HeroVortex';
import { ChevronRight } from 'lucide-react';
import Link from 'next/link';
import { CtaRow, DEMO_URL } from '../new/_components/ui';
import { PricingPlans } from './PricingPlans';
import { AIUsage, CloudComparison, HostingOptions, PricingFAQ } from './PricingSections';

export default function PricingPage() {
  return <div className="pricing-page">
    <section className="pricing-hero"><HeroVortex variant="orbit" tone="dark" /><div className="wrap"><div className="pricing-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Pricing</span></div><span className="eyebrow">Helpin pricing</span><h1>One platform.<br /><span>Your whole team, connected.</span></h1><p className="lede">All product modules. Unlimited teammates. Choose managed Cloud hosting or run Helpin yourself.</p><nav className="pricing-jump" aria-label="Pricing options"><a href="#cloud-plans">Cloud plans ↓</a><a href="#self-hosted">Self-hosting ↓</a><a href="#enterprise">Enterprise ↓</a></nav></div></section>
    <PricingPlans />
    <HostingOptions />
    <AIUsage />
    <CloudComparison />
    <PricingFAQ />
    <section className="pricing-final" aria-labelledby="pricing-final-title"><div className="wrap"><div><h2 id="pricing-final-title">Choose your plan. Bring your team.</h2><p className="lede">Try Cloud without a card, or <a href="/new/self-hosting">explore self-hosting</a>.</p></div><CtaRow primaryLabel="Start free trial" secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /></div></section>
  </div>;
}
