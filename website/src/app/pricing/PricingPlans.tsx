'use client';

import { useState } from 'react';
import { ArrowRight, Building2, Check } from 'lucide-react';
import { AI_ALLOWANCE, PLANS } from './pricing-data';
import { DEMO_URL, GITHUB_URL } from '../new/_components/ui';

const SELF_HOSTED_FEATURES = [
  'Every module and Growth feature, no plan limits',
  'Support, projects, CRM, meetings, docs, and automation',
  'AI agents with your own provider keys',
  'No Helpin AI or tool fees',
  'Community help on GitHub',
];

export function PricingPlans() {
  const [annual, setAnnual] = useState(false);
  return <section id="cloud-plans" className="pricing-plans"><div className="wrap">
    <div className="pricing-plan-heading">
      <h2>Same product. You choose who runs it.</h2>
      <div className="pricing-billing" role="group" aria-label="Billing period">
        <button type="button" aria-pressed={!annual} onClick={() => setAnnual(false)}>Monthly</button>
        <button type="button" aria-pressed={annual} onClick={() => setAnnual(true)}>Annually <span>−20%</span></button>
      </div>
    </div>

    <div className="pricing-plan-grid">
      <article id="self-hosted" className="pricing-plan" aria-labelledby="plan-self-hosted">
        <h3 id="plan-self-hosted">Self-hosted</h3>
        <p className="pricing-plan-description">The complete open-source product, on your servers.</p>
        <div className="pricing-price"><div className="pricing-price-amount"><span>$</span><strong>0</strong><span>forever</span></div><p>AGPL-3.0 · You cover hosting and AI providers</p></div>
        <a className="btn btn-secondary" href="/new/self-hosting">Read the self-hosting guide<ArrowRight size={16} aria-hidden="true" /></a>
        <dl className="pricing-plan-metrics"><div><dt>Teammates</dt><dd>Unlimited</dd></div><div><dt>AI usage</dt><dd>Your provider keys</dd></div></dl>
        <ul>{SELF_HOSTED_FEATURES.map(feature => <li key={feature}><Check size={14} strokeWidth={2} aria-hidden="true" /><span>{feature}</span></li>)}</ul>
        <p className="pricing-plan-aside">Community 0.1 is in beta. <a href={GITHUB_URL}>View on GitHub</a></p>
      </article>

      {PLANS.map(plan => {
        const price = annual ? plan.annual : plan.price;
        const ai = AI_ALLOWANCE[plan.key][annual ? 'annual' : 'monthly'];
        return <article className="pricing-plan" data-featured={plan.popular} key={plan.key} aria-labelledby={`plan-${plan.key}`}>
          <h3 id={`plan-${plan.key}`}>{plan.name}</h3>
          <p className="pricing-plan-description">{plan.description}</p>
          <div className="pricing-price" aria-live="polite" aria-atomic="true">
            <div className="pricing-price-amount"><span>$</span><strong>{price}</strong><span>/ month</span></div>
            <p>Per workspace · {annual ? `$${(plan.annual * 12).toLocaleString('en-US')} billed yearly` : 'Billed monthly'}</p>
          </div>
          <a className={`btn ${plan.popular ? 'btn-primary' : 'btn-secondary'}`} href={plan.href}>{plan.cta}<ArrowRight size={16} aria-hidden="true" /></a>
          <dl className="pricing-plan-metrics"><div><dt>Teammates</dt><dd>Unlimited</dd></div><div><dt><a href="#ai-usage">AI usage included</a></dt><dd>${ai} / month</dd></div></dl>
          <h4>{plan.featuresHeading}</h4>
          <ul>{plan.features.map(feature => <li key={feature}><Check size={14} strokeWidth={2} aria-hidden="true" /><span>{feature}</span></li>)}</ul>
        </article>;
      })}
    </div>

    <div className="pricing-plan-footnote">
      <p>Self-hosting is free and complete. Cloud adds hosting, included AI, and support. Cloud plans start with a 14-day Growth trial, no card required. Prices in USD, taxes extra.</p>
      <a href="#compare-plans">Compare Cloud plans<ArrowRight size={14} aria-hidden="true" /></a>
    </div>

    <div id="enterprise" className="pricing-enterprise-strip">
      <Building2 size={20} aria-hidden="true" />
      <p><strong>Enterprise</strong> A commercial license for teams that can’t use AGPL, your own AI provider keys on Cloud, deployment help, and support terms.</p>
      <a className="btn-link" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk to us<ArrowRight size={14} aria-hidden="true" /></a>
    </div>
  </div></section>;
}
