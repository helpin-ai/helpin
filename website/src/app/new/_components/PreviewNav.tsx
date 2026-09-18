import Link from 'next/link';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, SIGNUP_URL, GithubIcon } from './ui';

const LINKS = [
  { label: 'Product', href: '#record', dropdown: true },
  { label: 'How it works', href: '#loop' },
  { label: 'Open source', href: '#os' },
  { label: 'Docs', href: '#dev' },
  { label: 'Pricing', href: '#pricing' },
];

export function PreviewNav() {
  return (
    <nav className="pnav">
      <div className="wrap">
        <Link href="/new" className="logo" aria-label="Helpin"><HelpinBrand /></Link>
        <div className="navlinks">
          {LINKS.map((l) => (
            <span key={l.label} className={l.dropdown ? 'dd' : undefined}><a href={l.href}>{l.label}</a></span>
          ))}
        </div>
        <div className="navright">
          <a className="gh" href={GITHUB_URL} target="_blank" rel="noopener noreferrer">
            <GithubIcon />
            GitHub
          </a>
          <a href="https://app.helpin.ai">Log in</a>
          <Link className="btn btn-primary" href={SIGNUP_URL}>Start free trial</Link>
        </div>
      </div>
    </nav>
  );
}
