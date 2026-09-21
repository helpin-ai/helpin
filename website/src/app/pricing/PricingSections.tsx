import { ArrowRight } from 'lucide-react';
import { SelfHostingArt, EnterprisePlanningArt } from './PricingDeploymentArt';
import { HelpinBrand } from '@/components/HelpinBrand';
import { DEMO_URL, FAQList, GITHUB_URL, SectionHead } from '../new/_components/ui';
import { WORKFLOW_COMPARISON, FAQS } from './pricing-data';

export function HostingOptions() {
  return <>
    <section id="self-hosted" className="pricing-deployment pricing-self-hosted" aria-labelledby="pricing-self-hosted-title"><div className="wrap pricing-deployment-grid">
      <div className="pricing-deployment-copy"><span className="eyebrow">Open source</span><h2 id="pricing-self-hosted-title">Run the whole product yourself.</h2><p className="pricing-deployment-lede">Support, projects, CRM, meetings, knowledge and agents—on your infrastructure.</p><div className="pricing-open-price"><strong>$0</strong><span>Software license<br />for the open-source product</span></div><p className="pricing-note">You operate the installation and pay hosting and provider costs. Enterprise features are licensed separately.</p><div className="pricing-links"><a className="btn btn-primary" href="/new/self-hosting">Explore self-hosting<ArrowRight size={16} /></a><a className="btn-link" href={GITHUB_URL}>View on GitHub →</a></div></div>
      <SelfHostingArt />
    </div></section>
    <section id="enterprise" className="pricing-deployment pricing-enterprise" aria-labelledby="pricing-enterprise-title"><div className="wrap pricing-deployment-grid">
      <div className="pricing-deployment-copy"><span className="eyebrow">Enterprise</span><h2 id="pricing-enterprise-title">Discuss your deployment needs.</h2><p className="pricing-deployment-lede">Need Enterprise features or help planning your rollout? Talk to us about your requirements and licensing.</p><p className="pricing-note">Enterprise code includes subscription billing, managed AI routes and commercial policies under a separate license.</p><div className="pricing-links"><a className="btn btn-primary" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk to us<ArrowRight size={16} /></a></div></div>
      <EnterprisePlanningArt />
    </div></section>
  </>;
}

export function WorkflowComparison() {
  return <section id="compare-your-stack"><div className="wrap pricing-narrow"><SectionHead eyebrow="Compare your stack" title="Bring the customer’s work together." lede="Review what you track across separate tools. Move the work into Helpin, or connect the tools you keep through APIs and selected integrations." />
    <div className="pricing-stack">{WORKFLOW_COMPARISON.map(item => <div key={item.tool}><span><img src={`/favicons/${item.domain}.png`} width={20} height={20} alt="" />{item.tool}</span><span>{item.workflow}</span></div>)}<div className="pricing-stack-total"><HelpinBrand /><span>One customer history across the work</span></div></div>
    <p className="pricing-note">Compare the workflows, subscription costs and integrations your team needs. Savings depend on the tools you replace.</p>
  </div></section>;
}

export { AIUsage } from './PricingAIUsage';
export { CloudComparison } from './CloudComparison';

export function PricingFAQ() {
  return <section id="pricing-questions"><div className="wrap pricing-faq"><SectionHead eyebrow="Questions" title="Before you choose." lede="Cloud billing, open source and AI usage." /><div><FAQList items={FAQS.map(({q,a}) => [q,a] as const)} className="pricing-faq-items" /><a className="btn-link" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk through your options →</a></div></div></section>;
}
