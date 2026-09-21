import Link from 'next/link';

export const DEMO_URL = 'https://cal.com/helpin-ai/30min';
export const INCLUDED_URL = '/new/self-hosting#whats-included';
export const SIGNUP_URL = 'https://app.helpin.ai/register';
export const GITHUB_URL = 'https://github.com/helpin-ai/helpin';

export function GithubIcon({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" focusable="false">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

export function Frame({ crumb, label, children, className = '' }: { crumb: string; label: string; children: React.ReactNode; className?: string }) {
  return (
    <div className={`frame ${className}`} aria-label={label}>
      <div className="frame-bar">
        <i /><i /><i />
        <span className="crumb">{crumb}</span>
      </div>
      {children}
    </div>
  );
}

export function Chip({ tone, children }: { tone?: 'em' | 'am'; children: React.ReactNode }) {
  return (
    <span className={`chip ${tone ?? ''}`}>
      <i />
      {children}
    </span>
  );
}

export function SectionHead({ eyebrow, title, lede, secondaryLede, tight }: { eyebrow: string; title: string; lede?: string; secondaryLede?: string; tight?: boolean }) {
  return (
    <div className="sec-head" style={tight ? { marginBottom: 28 } : undefined}>
      <span className="eyebrow">{eyebrow}</span>
      <h2>{title}</h2>
      {lede ? <p className="lede">{lede}</p> : null}
      {secondaryLede ? <p className="lede">{secondaryLede}</p> : null}
    </div>
  );
}

export function CtaRow({ secondaryHref = GITHUB_URL, secondaryLabel = 'View on GitHub', primaryLabel = 'Start free' }: { secondaryHref?: string; secondaryLabel?: string; primaryLabel?: string }) {
  const external = secondaryHref.startsWith('http');
  return (
    <div className="cta-row">
      <Link className="btn btn-primary" href={SIGNUP_URL}>{primaryLabel} →</Link>
      <a className="btn btn-secondary" href={secondaryHref} target={external ? '_blank' : undefined} rel={external ? 'noopener noreferrer' : undefined}>
        {secondaryLabel === 'View on GitHub' ? <GithubIcon /> : null}
        {secondaryLabel} →
      </a>
    </div>
  );
}


export function Availability({ category }: { category: string }) {
  return <span className="eyebrow">{category}</span>;
}
export function CtaNote({ trial = false, support = false }: { trial?: boolean; support?: boolean }) {
  return <p className="cta-note">{trial ? `14-day cloud trial · No card${support ? ' · Or self-host free' : ''}` : 'Open source · Run it yourself or use our cloud'}</p>;
}
export type FAQItem = readonly [question: string, answer: string, href?: string, label?: string];
export function FAQList({ items, className }: { items: readonly FAQItem[]; className: string }) {
  return <div className={className}>{items.map(([question, answer, href, label]) => <details key={question}><summary>{question}</summary><p>{answer}{href && <> <a className="faq-more" href={href}>{label ?? 'Learn more'} →</a></>}</p></details>)}</div>;
}
