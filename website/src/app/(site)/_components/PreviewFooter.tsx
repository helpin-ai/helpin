import Link from 'next/link';
import { HeroVortex } from './HeroVortex';
import { ArrowUpRight, Mail } from 'lucide-react';
import { HelpinBrand } from '@/components/HelpinBrand';
import { DISCORD_URL, DiscordIcon, GITHUB_URL, GithubIcon } from './ui';
import { DOCS } from './docsLinks';
import { SUPPORT_COMPETITORS } from '../compare/compare-data';

// Keep the footer focused on current pages; detailed capabilities live on each page.
const COLUMNS = [
  { title: 'Products', links: [
    { label: 'Support', href: '/products/customer-support' },
    { label: 'Customer portal', href: '/products/customer-portal' },
    { label: 'Knowledge', href: '/products/knowledge' },
    { label: 'AI Agents', href: '/products/ai-agents' },
    { label: 'Projects', href: '/products/projects' },
    { label: 'CRM', href: '/products/crm' },
    { label: 'Meetings', href: '/products/meetings' },
  ] },
  { title: 'Resources', links: [
    { label: 'Developers', href: '/developers' },
    { label: 'Self-hosting', href: '/self-hosting' },
    { label: 'Documentation', href: DOCS.home },
  ] },
  { title: 'Compare', links: [
    ...SUPPORT_COMPETITORS.map(competitor => ({ label: `vs ${competitor.name}`, href: `/compare/${competitor.slug}` })),
    { label: 'All comparisons', href: '/compare' },
  ] },
  { title: 'Community', links: [
    { label: 'Join our Discord', href: DISCORD_URL },
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
            <Link prefetch={false} href="/" className="logo" aria-label="Helpin homepage">
              <HelpinBrand variant="light-on-dark" />
            </Link>
            <p>Open-source customer support.<br />{homepage ? "With AI agents that do more than answer." : <>With AI agents that do<br />more than answer.</>}</p>
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
                        <Link prefetch={false} href={link.href}>{link.label}</Link>
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
            <a href={DISCORD_URL} target="_blank" rel="noopener noreferrer" aria-label="Helpin on Discord"><DiscordIcon size={18} /></a>
            <a href="mailto:hello@helpin.ai" aria-label="Email Helpin"><Mail size={18} aria-hidden="true" /></a>
          </div>
        </div>
        {homepage && <p className="footer-descriptor">Helpin is open-source customer support software with AI agents, a help center, and built-in projects and CRM.</p>}
        <div className="footer-bottom">
          <nav className="footer-legal" aria-label="Legal and brand resources">
            <Link prefetch={false} href="/branding">Branding</Link>
            <Link prefetch={false} href="/privacy">Privacy</Link>
            <Link prefetch={false} href="/terms">Terms</Link>
            <a href={`${GITHUB_URL}/blob/develop/SECURITY.md`} target="_blank" rel="noopener noreferrer">Security</a>
          </nav>
          <p>© {new Date().getFullYear()} Helpin AI.</p>
        </div>
      </div>
    </footer>
  );
}
