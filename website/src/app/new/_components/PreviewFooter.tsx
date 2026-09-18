import Link from 'next/link';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL } from './ui';

const COLUMNS: { title: string; links: { label: string; href: string }[] }[] = [
  { title: 'Product', links: [
    { label: 'Support inbox', href: '#record' }, { label: 'Help center', href: '#record' }, { label: 'Projects', href: '#loop' },
    { label: 'CRM and meetings', href: '#record' }, { label: 'Agents and automations', href: '#agents' }, { label: 'Pricing', href: '/pricing' },
  ] },
  { title: 'Open source', links: [
    { label: 'GitHub', href: GITHUB_URL }, { label: 'Install guide', href: `${GITHUB_URL}/blob/develop/community/README.md` },
    { label: 'License', href: `${GITHUB_URL}/blob/develop/LICENSE` }, { label: 'Contributing', href: `${GITHUB_URL}/blob/develop/CONTRIBUTING.md` },
    { label: 'Roadmap and limitations', href: `${GITHUB_URL}/blob/develop/ROADMAP.md` }, { label: 'Security policy', href: `${GITHUB_URL}/blob/develop/SECURITY.md` },
  ] },
  { title: 'Developers', links: [
    { label: 'Documentation', href: `${GITHUB_URL}/blob/develop/docs/README.md` }, { label: 'MCP server', href: `${GITHUB_URL}/blob/develop/docs/public-mcp-server.md` },
    { label: 'SDK', href: `${GITHUB_URL}/tree/develop/packages/sdk-js` }, { label: 'Architecture', href: `${GITHUB_URL}/blob/develop/ARCHITECTURE.md` },
  ] },
  { title: 'Company', links: [
    { label: 'Privacy', href: '/privacy' }, { label: 'Terms', href: '/terms' }, { label: 'Contact', href: 'mailto:hello@helpin.ai' },
  ] },
];

export function PreviewFooter() {
  return (
    <footer className="pfoot">
      <div className="wrap">
        <div className="fgrid">
          <div>
            <Link href="/new" className="logo" aria-label="Helpin"><HelpinBrand /></Link>
            <p className="tagline">Open-source support, docs, projects, and CRM for SaaS teams.</p>
          </div>
          {COLUMNS.map((c) => (
            <div key={c.title}>
              <b>{c.title}</b>
              {c.links.map((l) => (
                l.href.startsWith('http') || l.href.startsWith('mailto')
                  ? <a key={l.label} href={l.href} target="_blank" rel="noopener noreferrer">{l.label}</a>
                  : <Link key={l.label} href={l.href}>{l.label}</Link>
              ))}
            </div>
          ))}
        </div>
        <p className="copyright">© 2026 Helpin AI. AGPL-3.0 application · Apache-2.0 SDK.</p>
      </div>
    </footer>
  );
}
