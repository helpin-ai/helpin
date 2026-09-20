import Link from 'next/link';
import { GridFlow } from './GridFlow';
import { ArrowUpRight, Mail } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, GithubIcon } from './ui';

const COLUMNS = [
  { title: 'Product', links: [
    { label: 'Overview', href: '/new#product' },
    { label: 'Customer records', href: '/new#record' },
    { label: 'Inbox', href: '/new/product#inbox' },
    { label: 'Meetings', href: '/new/products/meetings' },
    { label: 'Projects', href: '/new/products/projects' },
    { label: 'CRM', href: '/new/product#crm' },
    { label: 'Knowledge', href: '/new/products/knowledge' },
  ] },
  { title: 'Agents & workflows', links: [
    { label: 'Ask Agent', href: '/new#ask-agent' },
    { label: 'AI agents', href: '/new/product#agents' },
    { label: 'Coding agents', href: '/new/product#agents' },
    { label: 'Tools & approvals', href: '/new#control' },
    { label: 'Automation', href: `${GITHUB_URL}/blob/develop/docs/agents-and-automation.md` },
    { label: 'External MCP', href: `${GITHUB_URL}/blob/develop/docs/external-mcp-servers.md` },
  ] },
  { title: 'Developers', links: [
    { label: 'Documentation', href: `${GITHUB_URL}/blob/develop/docs/README.md` },
    { label: 'APIs & SDKs', href: '/new#developers' },
    { label: 'JavaScript SDK', href: `${GITHUB_URL}/tree/develop/packages/sdk-js` },
    { label: 'MCP server', href: `${GITHUB_URL}/blob/develop/docs/public-mcp-server.md` },
    { label: 'Helpin CLI', href: `${GITHUB_URL}/blob/develop/community/README.md` },
    { label: 'Self-hosting', href: '/new#open-source' },
  ] },
  { title: 'Open source', links: [
    { label: 'Repository', href: GITHUB_URL },
    { label: 'Contributing', href: `${GITHUB_URL}/blob/develop/CONTRIBUTING.md` },
    { label: 'Report an issue', href: `${GITHUB_URL}/issues` },
    { label: 'Releases', href: `${GITHUB_URL}/releases` },
    { label: 'Contact', href: 'mailto:hello@helpin.ai' },
  ] },
];

export function PreviewFooter() {
  return (
    <footer className="pfoot">
      <GridFlow />
      <div className="wrap">
        <div className="footer-main">
          <div className="footer-brand">
            <Link href="/new" className="logo" aria-label="Helpin homepage">
              <HelpinBrand variant="light-on-dark" />
            </Link>
            <p>One customer history.<br />A workspace for your team<br />and agents.</p>
          </div>
          <nav className="footer-nav" aria-label="Footer navigation">
            {COLUMNS.map((column) => (
              <div className="footer-column" key={column.title}>
                <h2>{column.title}</h2>
                <ul>
                  {column.links.map((link) => (
                    <li key={link.label}>
                      {link.href.startsWith('http') ? (
                        <a href={link.href} target="_blank" rel="noopener noreferrer">
                          {link.label}<ArrowUpRight size={12} aria-hidden="true" />
                        </a>
                      ) : link.href.startsWith('mailto:') ? (
                        <a href={link.href}>{link.label}</a>
                      ) : (
                        <Link href={link.href}>{link.label}</Link>
                      )}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </nav>
        </div>
        <div className="footer-community">
          <div className="footer-socials">
            <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer" aria-label="Helpin on GitHub"><GithubIcon size={18} /></a>
            <a href="mailto:hello@helpin.ai" aria-label="Email Helpin"><Mail size={18} aria-hidden="true" /></a>
          </div>
          <a className="footer-open" href={GITHUB_URL} target="_blank" rel="noopener noreferrer">Open source. Yours to build.<ArrowUpRight size={14} aria-hidden="true" /></a>
        </div>
        <div className="footer-bottom">
          <nav className="footer-legal" aria-label="Legal and brand resources">
            <Link href="/new/branding">Branding</Link>
            <Link href="/privacy">Privacy</Link>
            <Link href="/terms">Terms</Link>
            <a href={`${GITHUB_URL}/blob/develop/SECURITY.md`} target="_blank" rel="noopener noreferrer">Security</a>
          </nav>
          <p>© {new Date().getFullYear()} Helpin AI.</p>
          <p className="footer-license">AGPL-3.0 application · Apache-2.0 SDK</p>
        </div>
      </div>
    </footer>
  );
}
