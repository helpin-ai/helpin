'use client';

import { useState } from 'react';
import { ArrowRight, Check } from 'lucide-react';
import { PLANS } from './pricing-data';

export function PricingPlans() {
  const [annual, setAnnual] = useState(false);
  return <section id="cloud-plans" className="pricing-plans"><div className="wrap">
    <div className="pricing-plan-heading"><div><span className="eyebrow">Helpin Cloud</span><h2>We host it. Your team gets to work.</h2></div><div className="pricing-billing-control"><div className="pricing-billing" role="group" aria-label="Billing period"><button type="button" aria-pressed={!annual} onClick={() => setAnnual(false)}>Monthly</button><button type="button" aria-pressed={annual} onClick={() => setAnnual(true)}>Annually</button></div><span className="pricing-annual-saving">Save 20% with annual billing</span></div></div>
    <div className="pricing-plan-grid">{PLANS.map(plan => <article className="pricing-plan" data-featured={plan.popular} key={plan.name} aria-labelledby={`plan-${plan.name.toLowerCase()}`}>
      <div className="pricing-plan-top">
        <header><h3 id={`plan-${plan.name.toLowerCase()}`}>{plan.name}</h3><span className="pricing-plan-position">{plan.popular ? 'More capacity & control' : 'The essentials, connected'}</span></header>
        <p className="pricing-plan-description">{plan.description}</p>
        <div className="pricing-price" aria-live="polite" aria-atomic="true"><div className="pricing-price-amount"><span>$</span><strong>{annual ? plan.annual : plan.price}</strong><span>/ month</span></div><p>Per workspace · {annual ? `$${(plan.annual * 12).toLocaleString('en-US')} billed annually` : 'billed monthly'} · USD</p></div>
        <a className={`btn ${plan.popular ? 'btn-primary' : 'btn-secondary'}`} href={plan.href}>{plan.cta}<ArrowRight size={16} aria-hidden="true" /></a>
        <p className="pricing-plan-trial">14-day Growth trial · No card required</p>
      </div>
      <div className="pricing-plan-details">
        <dl className="pricing-plan-metrics"><div><dt>Teammates</dt><dd>{plan.seats}</dd></div><div><dt><a href="#ai-usage">Monthly AI allowance</a></dt><dd>{plan.aiUsage}</dd></div></dl>
        <h4>{plan.popular ? 'Everything in Starter, plus' : 'Included in Starter'}</h4>
        <ul>{plan.features.filter(feature => feature !== 'Everything in Starter, plus:').map(feature => <li key={feature}><Check size={14} strokeWidth={1.6} aria-hidden="true" /><span>{feature}</span></li>)}</ul>
      </div>
    </article>)}</div>
    <div className="pricing-plan-footnote"><p>Prices exclude applicable taxes. Both plans include all product modules.</p><a href="#compare-plans">Compare every feature<ArrowRight size={14} aria-hidden="true" /></a></div>
  </div></section>;
}
