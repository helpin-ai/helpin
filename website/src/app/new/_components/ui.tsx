import Link from 'next/link';

export const SIGNUP_URL = 'https://app.helpin.ai/register';
export const GITHUB_URL = 'https://github.com/helpin-ai/helpin';

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

export function SectionHead({ eyebrow, title, lede, tight }: { eyebrow: string; title: string; lede?: string; tight?: boolean }) {
  return (
    <div className="sec-head" style={tight ? { marginBottom: 28 } : undefined}>
      <span className="eyebrow">{eyebrow}</span>
      <h2>{title}</h2>
      {lede ? <p className="lede">{lede}</p> : null}
    </div>
  );
}

export function CtaRow({ secondaryHref = '#os', secondaryLabel = 'View on GitHub' }: { secondaryHref?: string; secondaryLabel?: string }) {
  return (
    <div className="cta-row">
      <Link className="btn btn-primary" href={SIGNUP_URL}>Start free trial →</Link>
      <a className="btn btn-secondary" href={secondaryHref}>{secondaryLabel}</a>
    </div>
  );
}
