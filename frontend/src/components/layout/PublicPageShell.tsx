import type { ReactNode } from 'react';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import { PublicPageVortex } from './PublicPageVortex';
import './public-page-shell.css';

interface PublicPageShellProps {
  children: ReactNode;
}

export function PublicPageShell({ children }: PublicPageShellProps) {
  return (
    <div className="public-page">
      <div className="public-page-main">
        <header className="public-page-header">
          <a href="https://helpin.ai" aria-label="Helpin home"><HelpinLogo /></a>
          <a href="https://helpin.ai" className="public-page-back">Back to website <span aria-hidden="true">↗</span></a>
        </header>
        <main className="public-page-content">{children}</main>
        <footer className="public-page-footer">
          <span>© {new Date().getFullYear()} Helpin</span>
          <span>Made for working together.</span>
        </footer>
      </div>
      <aside className="public-page-brand" aria-label="Helpin: good work happens together">
        <div className="public-page-brand-heading">
          <p className="public-page-eyebrow">A little more connected</p>
          <h2>Good work<br />happens together.</h2>
        </div>
        <PublicPageVortex />
        <div className="public-page-brand-footer"><span>A shared space for your team.</span></div>
      </aside>
    </div>
  );
}
