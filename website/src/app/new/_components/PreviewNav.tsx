import Link from 'next/link';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, SIGNUP_URL, GithubIcon } from './ui';

const LINKS = [
  { label: 'Product', href: '#record' },
  { label: 'Developers', href: '#developers' },
  { label: 'Open Source', href: '#open-source' },
  { label: 'Pricing', href: '#pricing' },
  { label: 'Docs', href: 'https://github.com/helpin-ai/helpin/blob/develop/docs/README.md' },
];

export function PreviewNav() {
  return (
    <nav className="pnav">
      <div className="wrap">
        <Link href="/new" className="logo" aria-label="Helpin"><HelpinBrand /></Link>
        <div className="navlinks">
          {LINKS.map((l) => (
            <span key={l.label}><a href={l.href}>{l.label}</a></span>
          ))}
        </div>
        <div className="navright">
          <a className="gh" href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
            <GithubIcon />
            GitHub
          </a>
          <a href="https://app.helpin.ai">Sign in</a>
          <Link className="btn btn-primary" href={SIGNUP_URL}>Start free →</Link>
        </div>
      </div>
    </nav>
  );
}
