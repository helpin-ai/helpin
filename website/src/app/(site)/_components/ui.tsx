import Link from 'next/link';
import { faqPage, JsonLd } from '@/lib/structured-data';

export const DEMO_URL = 'https://cal.com/helpin-ai/30min';
export const SIGNUP_URL = 'https://app.helpin.ai/register';
export const GITHUB_URL = 'https://github.com/helpin-ai/helpin';
export const DISCORD_URL = 'https://discord.gg/23WcB2chf';

export function GithubIcon({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 16 16" fill="currentColor" aria-hidden="true" focusable="false">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82.64-.18 1.32-.27 2-.27.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8z" />
    </svg>
  );
}

export function DiscordIcon({ size = 16 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" fill="currentColor" aria-hidden="true" focusable="false">
      <path d="M20.317 4.3698a19.7913 19.7913 0 00-4.8851-1.5152.0741.0741 0 00-.0785.0371c-.211.3753-.4447.8648-.6083 1.2495-1.8447-.2762-3.68-.2762-5.4868 0-.1636-.3933-.4058-.8742-.6177-1.2495a.077.077 0 00-.0785-.037 19.7363 19.7363 0 00-4.8852 1.515.0699.0699 0 00-.0321.0277C.5334 9.0458-.319 13.5799.0992 18.0578a.0824.0824 0 00.0312.0561c2.0528 1.5076 4.0413 2.4228 5.9929 3.0294a.0777.0777 0 00.0842-.0276c.4616-.6304.8731-1.2952 1.226-1.9942a.076.076 0 00-.0416-.1057c-.6528-.2476-1.2743-.5495-1.8722-.8923a.077.077 0 01-.0076-.1277c.1258-.0943.2517-.1923.3718-.2914a.0743.0743 0 01.0776-.0105c3.9278 1.7933 8.18 1.7933 12.0614 0a.0739.0739 0 01.0785.0095c.1202.099.246.1981.3728.2924a.077.077 0 01-.0066.1276 12.2986 12.2986 0 01-1.873.8914.0766.0766 0 00-.0407.1067c.3604.698.7719 1.3628 1.225 1.9932a.076.076 0 00.0842.0286c1.961-.6067 3.9495-1.5219 6.0023-3.0294a.077.077 0 00.0313-.0552c.5004-5.177-.8382-9.6739-3.5485-13.6604a.061.061 0 00-.0312-.0286zM8.02 15.3312c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9555-2.4189 2.157-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.9555 2.4189-2.1569 2.4189zm7.9748 0c-1.1825 0-2.1569-1.0857-2.1569-2.419 0-1.3332.9554-2.4189 2.1569-2.4189 1.2108 0 2.1757 1.0952 2.1568 2.419 0 1.3332-.946 2.4189-2.1568 2.4189Z" />
    </svg>
  );
}

export function SectionHead({ eyebrow, title, lede, secondaryLede, tight }: { eyebrow: string; title: React.ReactNode; lede?: string; secondaryLede?: string; tight?: boolean }) {
  return (
    <div className="sec-head" style={tight ? { marginBottom: 28 } : undefined}>
      <span className="eyebrow">{eyebrow}</span>
      <h2>{title}</h2>
      {lede ? <p className="lede">{lede}</p> : null}
      {secondaryLede ? <p className="lede">{secondaryLede}</p> : null}
    </div>
  );
}

export function CtaRow({ secondaryHref = GITHUB_URL, secondaryLabel = 'View on GitHub', primaryLabel = 'Start free', primaryHref = SIGNUP_URL }: { primaryHref?: string; secondaryHref?: string; secondaryLabel?: string; primaryLabel?: string }) {
  const external = secondaryHref.startsWith('http');
  return (
    <div className="cta-row">
      <Link prefetch={false} className="btn btn-primary" href={primaryHref}>{primaryLabel} →</Link>
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
  return <p className="cta-note">{trial ? `14-day free trial · No card required${support ? ' · Or self-host free' : ''}` : 'Open source · Self-host free, or let us run it'}</p>;
}
export type FAQItem = readonly [question: string, answer: string, href?: string, label?: string];
export function FAQList({ items, className }: { items: readonly FAQItem[]; className: string }) {
  return <div className={className}><JsonLd data={faqPage(items)} />{items.map(([question, answer, href, label]) => <details key={question}><summary>{question}</summary><p>{answer}{href && <> <a className="faq-more" href={href}>{label ?? 'Learn more'} →</a></>}</p></details>)}</div>;
}
