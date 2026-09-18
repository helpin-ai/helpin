import Link from 'next/link';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL } from './ui';

const COLUMNS: { title: string; links: { label: string; href: string }[] }[] = [
  { title: 'Product', links: [
    { label: 'Customer records', href: '#record' }, { label: 'Inbox', href: '#loop' }, { label: 'Meetings', href: '#record' },
    { label: 'Projects', href: '#projects' }, { label: 'Docs', href: '#record' }, { label: 'AI agents', href: '#agents' }, { label: 'Integrations', href: '#integrations' },
  ] },
  { title: 'Developers', links: [
    { label: 'Documentation', href: `${GITHUB_URL}/blob/develop/docs/README.md` }, { label: 'API', href: `${GITHUB_URL}/blob/develop/docs/README.md` },
    { label: 'SDK', href: `${GITHUB_URL}/tree/develop/packages/sdk-js` }, { label: 'MCP', href: `${GITHUB_URL}/blob/develop/docs/public-mcp-server.md` },
    { label: 'GitHub', href: GITHUB_URL }, { label: 'Self-hosting', href: `${GITHUB_URL}/blob/develop/community/README.md` },
  ] },
  { title: 'Company', links: [
    { label: 'About', href: '#' }, { label: 'Changelog', href: '#' }, { label: 'Blog', href: '#' }, { label: 'Contact', href: 'mailto:hello@helpin.ai' },
  ] },
  { title: 'Open Source', links: [
    { label: 'GitHub', href: GITHUB_URL }, { label: 'Contributing', href: `${GITHUB_URL}/blob/develop/CONTRIBUTING.md` },
    { label: 'Issues', href: `${GITHUB_URL}/issues` }, { label: 'Releases', href: `${GITHUB_URL}/releases` },
  ] },
  { title: 'Legal', links: [
    { label: 'Privacy', href: '/privacy' }, { label: 'Terms', href: '/terms' }, { label: 'Security', href: `${GITHUB_URL}/blob/develop/SECURITY.md` },
  ] },
];

export function PreviewFooter() {
  return (
    <footer className="pfoot">
      <div className="wrap">
        <div className="fgrid">
          <div>
            <Link href="/new" className="logo" aria-label="Helpin"><HelpinBrand /></Link>
            <p className="tagline">Open-source workspace for SaaS teams.</p>
            <p className="tagline strong">Hear customers. Decide what matters. Ship it.</p>
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
