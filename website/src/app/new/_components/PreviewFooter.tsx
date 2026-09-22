import Link from 'next/link';
import { HeroVortex } from './HeroVortex';
import { ArrowUpRight, Mail } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { GITHUB_URL, GithubIcon } from './ui';

// Keep the footer focused on current pages; detailed capabilities live on each page.
const COLUMNS = [
  { title: 'Products', links: [
    { label: 'Support', href: '/new/products/customer-support' },
    { label: 'Meetings', href: '/new/products/meetings' },
    { label: 'Projects', href: '/new/products/projects' },
    { label: 'CRM', href: '/new/products/crm' },
    { label: 'Knowledge', href: '/new/products/knowledge' },
    { label: 'AI Agents', href: '/new/products/ai-agents' },
  ] },
  { title: 'Resources', links: [
    { label: 'Developers', href: '/new/developers' },
    { label: 'Self-hosting', href: '/new/self-hosting' },
    { label: 'Documentation', href: `${GITHUB_URL}/blob/develop/docs/README.md` },
  ] },
  { title: 'Community', links: [
    { label: 'Contributing', href: `${GITHUB_URL}/blob/develop/CONTRIBUTING.md` },
    { label: 'Releases', href: `${GITHUB_URL}/releases` },
    { label: 'Report an issue', href: `${GITHUB_URL}/issues` },
    { label: 'Contact', href: 'mailto:hello@helpin.ai' },
  ] },
];

export function PreviewFooter({ homepage = false }: { homepage?: boolean }) {
  return (
    <footer className="pfoot section-motion">
      <HeroVortex variant="converge" tone="dark" />
      <div className="wrap">
        <div className="footer-main">
          <div className="footer-brand">
            <Link href="/new" className="logo" aria-label="Helpin homepage">
              <HelpinBrand variant="light-on-dark" />
            </Link>
            <p>One customer history.<br />{homepage ? "A shared workspace for your team and AI agents." : <>A workspace for your team<br />and agents.</>}</p>
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
            {!homepage && <a href={GITHUB_URL} target="_blank" rel="noopener noreferrer" aria-label="Helpin on GitHub"><GithubIcon size={18} /></a>}
            <a href="mailto:hello@helpin.ai" aria-label="Email Helpin"><Mail size={18} aria-hidden="true" /></a>
          </div>
        </div>
        {homepage && <p className="footer-descriptor">Helpin — Support, projects, CRM, meetings, and docs. Connected by customer history. Powered by AI agents.</p>}
        <div className="footer-bottom">
          <nav className="footer-legal" aria-label="Legal and brand resources">
            <Link href="/new/branding">Branding</Link>
            <Link href="/privacy">Privacy</Link>
            <Link href="/terms">Terms</Link>
            <a href={`${GITHUB_URL}/blob/develop/SECURITY.md`} target="_blank" rel="noopener noreferrer">Security</a>
          </nav>
          <p>© {new Date().getFullYear()} Helpin AI.</p>
        </div>
      </div>
    </footer>
  );
}
