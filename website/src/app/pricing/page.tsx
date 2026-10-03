import { HeroVortex } from '../(site)/_components/HeroVortex';

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
    <section className="pricing-hero"><HeroVortex variant="orbit" tone="dark" /><div className="wrap"><h1>Your team and AI agents.<br /><span>One workspace price.</span></h1><p className="lede">Let us host Helpin with AI usage included, or self-host and use your own providers. Invite your team without paying per seat. Agent features and limits vary by plan.</p></div></section>
    <PricingPlans />
    <AIUsage />
    <CloudComparison />
    <PricingFAQ />
    <section className="final-cta final-cta-connected" aria-labelledby="pricing-final-title"><div className="wrap"><ConnectedWorkspace /><div className="final"><span className="eyebrow">Free trial</span><h2 id="pricing-final-title">Try Growth free for 14 days.</h2><p className="lede">Try the agents and automation in Growth with your team. No card required. Choose a plan when the trial ends.</p><CtaRow primaryLabel="Start free trial" primaryHref={`${SIGNUP_URL}?plan=growth`} secondaryHref={DEMO_URL} secondaryLabel="Book a demo" /><p className="pricing-final-note">Prefer your own servers? <a href="/self-hosting">Self-host the open-source edition</a></p></div></div></section>
  </div>;
}
