import { ArrowRight, BookOpen, Bot, Kanban, MessagesSquare, Users } from 'lucide-react';
import { SelfHostingArt, EnterprisePlanningArt } from './PricingDeploymentArt';
import { HelpinBrand } from '@/components/HelpinBrand';
import { DEMO_URL, FAQList, GITHUB_URL, SectionHead } from '../new/_components/ui';
import { WORKFLOW_COMPARISON, FAQS } from './pricing-data';

export function HostingOptions() {
  return <>
    <section id="self-hosted" className="pricing-deployment pricing-self-hosted" aria-labelledby="pricing-self-hosted-title"><div className="wrap pricing-deployment-grid">
      <div className="pricing-deployment-copy"><span className="eyebrow">Run it yourself</span><h2 id="pricing-self-hosted-title">The connected workspace.<br />On your infrastructure.</h2><p className="pricing-deployment-lede">Bring your customer history, team, and agents into an installation you operate. Configure the services your workflows need and manage the deployment yourself.</p><div className="pricing-open-price"><strong>$0</strong><span>software license<br />Open-source edition.</span></div><p className="pricing-note">Your team covers infrastructure, maintenance, and provider usage. Separately licensed Enterprise capabilities are not included in the open-source license.</p><div className="pricing-links"><a className="btn btn-primary" href="/new/self-hosting">Explore self-hosting<ArrowRight size={16} /></a><a className="btn-link" href={GITHUB_URL}>View on GitHub →</a></div></div>
      <div><SelfHostingArt /><p className="pricing-note">Control the deployment. Understand the connections.</p></div>
    </div></section>
    <section id="enterprise" className="pricing-deployment pricing-enterprise" aria-labelledby="pricing-enterprise-title"><div className="wrap pricing-deployment-grid">
      <div className="pricing-deployment-copy"><span className="eyebrow">Requirements beyond a standard setup</span><h2 id="pricing-enterprise-title">Start with your requirements.<br />Plan the right deployment.</h2><p className="pricing-deployment-lede">Tell us how your team works, what you need to connect, and the operating requirements you need to meet. We’ll discuss the fit, rollout, and licensing.</p><p className="pricing-note">Enterprise capabilities use separate commercial terms.</p><div className="pricing-links"><a className="btn btn-primary" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Discuss your requirements<ArrowRight size={16} /></a></div></div>
      <div className="pricing-enterprise-visual"><EnterprisePlanningArt /><p className="pricing-note">Agree on the requirements before the rollout.</p></div>
    </div></section>
  </>;
}

const WORKFLOW_ICONS = [Kanban, MessagesSquare, Users, BookOpen, Bot];

export function WorkflowComparison() {
  return <section id="compare-your-stack" className="pricing-workflow-section"><div className="wrap pricing-narrow"><SectionHead eyebrow="Look at the work, not just the subscriptions" title={"What happens between\nyour tools matters too."} lede="Follow a customer request through your current process. Where does the context disappear? Who has to explain it again? What happens after the work is finished?" />
    <div className="pricing-stack">
      {WORKFLOW_COMPARISON.map((item, index) => {
        const Icon = WORKFLOW_ICONS[index];
        return <article className="pricing-workflow-card" key={item.tool}>
          <div className="pricing-workflow-heading"><span className="pricing-workflow-icon"><Icon size={21} strokeWidth={1.6} aria-hidden="true" /></span><h3>{item.workflow}</h3></div>
          <p>{item.description}</p>
          <div className="pricing-workflow-tools"><img src={`/favicons/${item.domain}.png`} width={18} height={18} alt="" /><span>{item.tool}</span></div>
        </article>;
      })}
      <article className="pricing-workflow-card pricing-stack-total">
        <HelpinBrand variant="light-on-dark" />
        <h3>The relationship and the work, together.</h3>
        <p>Connect the customer’s conversations to the projects, decisions, and follow-ups that come next.</p>
        <span className="pricing-workflow-connected">One customer history.</span>
      </article>
    </div>
    <p className="pricing-note">Compare the workflows you need, the connections you will keep, and the total cost of running them. Replacing more tools is not the goal; making the work easier to follow is.</p>
  </div></section>;
}

export { AIUsage } from './PricingAIUsage';
export { CloudComparison } from './CloudComparison';

export function PricingFAQ() {
  return <section id="pricing-questions"><div className="wrap pricing-faq"><SectionHead eyebrow="Before you choose" title="Know what you’re signing up for." /><div><FAQList items={FAQS.map(({q,a}) => [q,a] as const)} className="pricing-faq-items" /><a className="btn-link" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk through your requirements →</a></div></div></section>;
}
