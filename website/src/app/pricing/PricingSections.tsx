import { ArrowRight, Server, Building2 } from 'lucide-react';
import { DEMO_URL, FAQList, GITHUB_URL, SectionHead } from '../new/_components/ui';
import { FAQS } from './pricing-data';

export function HostingOptions() {
  return <section className="pricing-hosting-options" aria-label="Self-hosting and Enterprise"><div className="wrap pricing-hosting-grid">
    <article id="self-hosted" aria-labelledby="pricing-self-hosted-title">
      <Server size={24} aria-hidden="true" /><span className="eyebrow">Run it yourself</span>
      <h2 id="pricing-self-hosted-title">Your infrastructure. The whole product.</h2>
      <p>Run all product modules with a <strong>$0 open-source software license</strong>. Your team covers infrastructure, maintenance, and provider usage. Enterprise capabilities are licensed separately.</p>
      <div className="pricing-links"><a className="btn-link" href="/new/self-hosting">Explore self-hosting <ArrowRight size={16} /></a><a className="btn-link" href={GITHUB_URL}>View on GitHub →</a></div>
    </article>
    <article id="enterprise" aria-labelledby="pricing-enterprise-title">
      <Building2 size={24} aria-hidden="true" /><span className="eyebrow">Enterprise</span>
      <h2 id="pricing-enterprise-title">Plan the deployment you need.</h2>
      <p>Discuss your requirements, integrations, and rollout with our team. Enterprise capabilities use separate commercial terms.</p>
      <div className="pricing-links"><a className="btn-link" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk to us <ArrowRight size={16} /></a></div>
    </article>
  </div></section>;
}

export { AIUsage } from './PricingAIUsage';
export { CloudComparison } from './CloudComparison';

export function PricingFAQ() {
  return <section id="pricing-questions"><div className="wrap pricing-faq"><SectionHead eyebrow="Before you choose" title="Know what you’re signing up for." /><div><FAQList items={FAQS.map(({q,a}) => [q,a] as const)} className="pricing-faq-items" /><a className="btn-link" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk through your requirements →</a></div></div></section>;
}
