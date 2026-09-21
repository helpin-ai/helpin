'use client';

import { useState } from 'react';
import { ArrowRight, Check } from 'lucide-react';
import { PLANS } from './pricing-data';

export function PricingPlans() {
  const [annual, setAnnual] = useState(false);
  return <section id="cloud-plans" className="pricing-plans"><div className="wrap">
    <div className="pricing-plan-heading"><div><span className="eyebrow">Helpin Cloud</span><h2>We host it. Your team gets to work.</h2></div><div className="pricing-billing" role="group" aria-label="Billing period"><button type="button" aria-pressed={!annual} onClick={() => setAnnual(false)}>Monthly</button><button type="button" aria-pressed={annual} onClick={() => setAnnual(true)}>Annual <span>Save 20%</span></button></div></div>
    <div className="pricing-plan-grid">{PLANS.map(plan => <article className="pricing-plan" data-featured={plan.popular} key={plan.name} aria-labelledby={`plan-${plan.name.toLowerCase()}`}>
      <header><span className="pricing-plan-icon"><plan.Icon size={22} aria-hidden="true" /></span><h3 id={`plan-${plan.name.toLowerCase()}`}>{plan.name}</h3>{plan.popular && <span className="pricing-plan-tag">More capacity & control</span>}</header>
      <p className="pricing-plan-description">{plan.description}</p>
      <div className="pricing-price" aria-live="polite" aria-atomic="true"><strong>${annual ? plan.annual : plan.price}</strong><span>/ workspace / month</span><p>{annual ? `$${(plan.annual * 12).toLocaleString('en-US')} billed annually` : 'Billed monthly'} · USD</p></div>
      <div className="pricing-plan-metrics"><div><strong>{plan.seats}</strong><span>Teammates</span></div><div><strong>{plan.aiUsage}</strong><a href="#ai-usage">Monthly AI allowance ↘</a></div></div>
      <a className={`btn ${plan.popular ? 'btn-primary' : 'btn-secondary'}`} href={plan.href}>{plan.cta}<ArrowRight size={16} aria-hidden="true" /></a>
      <p className="pricing-plan-trial">14-day Growth trial · No card required</p>
      <ul>{plan.features.map(feature => <li key={feature}><Check size={16} aria-hidden="true" /><span>{feature}</span></li>)}</ul>
    </article>)}</div>
    <p className="pricing-note">Prices exclude applicable taxes. Both plans include all product modules; capacity and advanced features vary below. <a href="#compare-plans">Compare Cloud plans →</a></p>
  </div></section>;
}
