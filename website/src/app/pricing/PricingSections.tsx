import { ArrowRight, Check, Code2, MessageCircle, Send, Zap } from 'lucide-react';
import { SelfHostingArt, EnterprisePlanningArt } from './PricingDeploymentArt';
import { AI_PRICING } from '@/generated/aiPricing';
import { HelpinBrand } from '@/components/HelpinBrand';
import { DEMO_URL, FAQList, GITHUB_URL, SectionHead } from '../new/_components/ui';
import { COMPARISON_FEATURES, WORKFLOW_COMPARISON, FAQS } from './pricing-data';

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

export function AIUsage() {
  const icons = [MessageCircle, Send, Code2, Zap];
  return <section id="ai-usage" className="pricing-soft"><div className="wrap"><SectionHead eyebrow="AI usage" title="Know what your agents use." lede="Cloud plans include a monthly AI allowance. Routine answers use less; longer planning, coding and review work uses more. Track usage in your workspace settings." />
    <div className="pricing-ai-grid">{AI_PRICING.tiers.map((tier,index) => { const Icon = icons[index] ?? Zap; return <article key={tier.key}><Icon size={22} aria-hidden="true" /><h3>{tier.label}</h3><p>{tier.description}</p></article>; })}</div>
    <div className="pricing-ai-notes"><p><strong>Included usage resets monthly.</strong> Allowances reset on your renewal date, including on annual subscriptions. Unused allowance does not roll over.</p><p><strong>Extra usage is optional.</strong> Enable it to keep agents working beyond the allowance. Additional usage is metered and billed separately.</p><p><strong>Use your own providers.</strong> Self-hosted installations use the connections you configure. On Cloud, provider connections and usage charges follow your workspace’s billing settings.</p></div>
  </div></section>;
}

function CellValue({ value }: { value: boolean | string | undefined }) {
  if (value === true) return <span className="pricing-included"><Check size={17} aria-hidden="true" /><span className="sr-only">Included</span></span>;
  if (value === false || value === undefined) return <span aria-label="Not included">—</span>;
  return <span>{value}</span>;
}

export function CloudComparison() {
  return <section id="compare-plans"><div className="wrap"><SectionHead eyebrow="Cloud plans" title="Compare capacity and controls." lede="Both plans include the product modules. Choose the capacity, automation and agent controls your team needs. These limits apply to hosted Cloud plans." />
    <div className="pricing-table-scroll" role="region" aria-label="Cloud plan feature comparison" tabIndex={0}><table className="pricing-table"><caption className="sr-only">Starter and Growth Cloud features and limits</caption><thead><tr><th scope="col">Feature</th><th scope="col">Starter</th><th scope="col">Growth</th></tr></thead><tbody>{COMPARISON_FEATURES.map((row,index) => 'category' in row && row.category ? <tr className="pricing-table-group" key={`${row.name}-${index}`}><th scope="colgroup" colSpan={3}>{row.name}</th></tr> : <tr key={`${row.name}-${index}`}><th scope="row">{row.name}</th><td><CellValue value={row.starter} /></td><td><CellValue value={row.growth} /></td></tr>)}</tbody></table></div>
  </div></section>;
}

export function PricingFAQ() {
  return <section id="pricing-questions"><div className="wrap pricing-faq"><SectionHead eyebrow="Questions" title="Before you choose." lede="Cloud billing, open source and AI usage." /><div><FAQList items={FAQS.map(({q,a}) => [q,a] as const)} className="pricing-faq-items" /><a className="btn-link" href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk through your options →</a></div></div></section>;
}
