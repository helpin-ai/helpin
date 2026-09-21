import Link from 'next/link';
import { ArrowRight, ChevronRight } from 'lucide-react';
import { ConnectedWorkspace } from '../ConnectedWorkspace';
import { CtaRow, CtaNote, FAQList, type FAQItem, GITHUB_URL } from '../ui';

export const REPO = `${GITHUB_URL}/blob/develop`;
export function DocLink({href,children}:{href:string;children:React.ReactNode}) {
  return <a className="platform-text-link" href={href} target="_blank" rel="noopener noreferrer">{children}<ArrowRight size={15}/></a>;
}
export function PlatformBreadcrumb({label}:{label:string}) {
  return <div className="platform-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12}/><span>{label}</span></div>;
}
export function PlatformFAQ({items}:{items:readonly FAQItem[]}) {
  return <FAQList items={items} className="platform-faqs" />;
}
export function PlatformClosing({id,title,description,eyebrow="Build on your terms",primaryLabel="Start free",primaryHref,supportingLine}:{id:string;title:string;description:string;eyebrow?:string;primaryLabel?:string;primaryHref?:string;supportingLine?:React.ReactNode}) {
  return <section className="final-cta final-cta-connected" aria-labelledby={id}><div className="wrap"><ConnectedWorkspace/><div className="final"><span className="eyebrow">{eyebrow}</span><h2 id={id}>{title}</h2><p className="lede">{description}</p><CtaRow primaryLabel={primaryLabel} primaryHref={primaryHref}/>{supportingLine ? <p className="developer-supporting-note">{supportingLine}</p> : <CtaNote />}</div></div></section>;
}
