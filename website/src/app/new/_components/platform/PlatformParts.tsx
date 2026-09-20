import Link from 'next/link';
import { ArrowRight, ChevronRight } from 'lucide-react';
import { ConnectedWorkspace } from '../ConnectedWorkspace';
import { GITHUB_URL, GithubIcon, SIGNUP_URL } from '../ui';

export const REPO = `${GITHUB_URL}/blob/develop`;
export function DocLink({href,children}:{href:string;children:React.ReactNode}) {
  return <a className="platform-text-link" href={href} target="_blank" rel="noopener noreferrer">{children}<ArrowRight size={15}/></a>;
}
export function PlatformBreadcrumb({label}:{label:string}) {
  return <div className="platform-breadcrumb"><Link href="/new">Helpin</Link><ChevronRight size={12}/><span>{label}</span></div>;
}
export function PlatformFAQ({items}:{items:readonly (readonly [string,string])[]}) {
  return <div className="platform-faqs">{items.map(([q,a])=><details key={q}><summary>{q}</summary><p>{a}</p></details>)}</div>;
}
export function PlatformClosing({id,title,description,hosting=false}:{id:string;title:string;description:string;hosting?:boolean}) {
  return <section className="final-cta final-cta-connected" aria-labelledby={id}><div className="wrap"><ConnectedWorkspace/><div className="final"><span className="eyebrow">Build on your terms</span><h2 id={id}>{title}</h2><p className="lede">{description}</p><div className="cta-row"><a className="btn btn-primary" href={hosting?`${REPO}/community/README.md`:SIGNUP_URL} target={hosting?'_blank':undefined} rel={hosting?'noopener noreferrer':undefined}>{hosting?'Read the install guide':'Start free'} →</a><a className="btn btn-secondary" href={GITHUB_URL} target="_blank" rel="noopener noreferrer"><GithubIcon/>View on GitHub →</a></div></div></div></section>;
}
