import { HeroVortex } from '../(site)/_components/HeroVortex';
import { ChevronRight } from 'lucide-react';
import Link from 'next/link';
import { ConnectedWorkspace } from '../(site)/_components/ConnectedWorkspace';
import { CtaRow, DEMO_URL, SIGNUP_URL } from '../(site)/_components/ui';
import { PricingPlans } from './PricingPlans';
import { AIUsage, CloudComparison, PricingFAQ } from './PricingSections';
import { PLANS } from './pricing-data';
import { JsonLd, organization, softwareApplication } from '@/lib/structured-data';

const PRICING_JSON_LD = { '@context': 'https://schema.org', '@graph': [organization, softwareApplication(PLANS)] };

export default function PricingPage() {
  return <div className="pricing-page">
    <JsonLd data={PRICING_JSON_LD} />
    <section className="pricing-hero"><HeroVortex variant="orbit" tone="dark" /><div className="wrap"><div className="pricing-breadcrumb"><Link href="/">Helpin</Link><ChevronRight size={12} aria-hidden="true" /><span>Pricing</span></div><h1>Every module. Every teammate.<br /><span>One price per workspace.</span></h1><p className="lede">Self-host the complete open-source product for free, or let us run it with AI included. No per-seat fees on any plan.</p><nav className="pricing-jump" aria-label="Pricing options"><a href="#cloud-plans">Compare plans ↓</a><a href="#self-hosted">Self-hosting ↓</a></nav></div></section>
    <PricingPlans />
    <AIUsage />
    <CloudComparison />
    <PricingFAQ />
    <section className="final-cta final-cta-connected" aria-labelledby="pricing-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Free trial</span><h2 id="pricing-final-title">Try Growth free for 14 days.</h2><p className="lede">No card required. When the trial ends, choose Starter or Growth.</p><CtaRow primaryLabel="Start free trial" primaryHref={`${SIGNUP_URL}?plan=growth`} secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="pricing-final-note">Prefer your own servers? <a href="/self-hosting">Self-host the open-source edition</a></p></div></div></section>
  </div>;
}
