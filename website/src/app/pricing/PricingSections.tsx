import { DEMO_URL, FAQList, SectionHead, LinkArrow } from '../(site)/_components/ui';
import { FAQS } from './pricing-data';

export { AIUsage } from './PricingAIUsage';
export { CloudComparison } from './CloudComparison';

export function PricingFAQ() {
  return <section id="pricing-questions"><div className="wrap pricing-faq"><div><SectionHead eyebrow="Questions, answered" title="FAQs about Helpin’s pricing" /><p className="pricing-faq-contact">Something else? <a href={DEMO_URL} target="_blank" rel="noopener noreferrer">Talk to our team<LinkArrow external /></a></p></div><FAQList items={FAQS.map(({ q, a }) => [q, a] as const)} className="pricing-faq-items" /></div></section>;
}
