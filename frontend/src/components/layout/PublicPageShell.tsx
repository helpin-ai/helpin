import type { ReactNode } from 'react';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import { cn } from '@/lib/utils';
import { PublicPageVortex } from './PublicPageVortex';
import './public-page-shell.css';

interface PublicPageShellProps {
  children: ReactNode;
  /** Replaces the "Back to website" link, for example with Sign out during onboarding. */
  headerAction?: ReactNode;
  /** `wide` fits multi-part forms such as workspace onboarding. */
  contentWidth?: 'narrow' | 'wide';
}

export function PublicPageShell({ children, headerAction, contentWidth = 'narrow' }: PublicPageShellProps) {
  return (
    <div className="public-page">
      <div className="public-page-main">
        <header className="public-page-header">
          <a href="https://helpin.ai" aria-label="Helpin home"><HelpinLogo /></a>
          {headerAction ?? (
            <a href="https://helpin.ai" className="public-page-back">Back to website <span aria-hidden="true">↗</span></a>
          )}
        </header>
        <main className={cn('public-page-content', contentWidth === 'wide' && 'public-page-content--wide')}>{children}</main>
        <footer className="public-page-footer">
          <span>Made for working together.</span>
        </footer>
      </div>
      <aside className="public-page-brand" aria-label="Helpin: good work happens together">
        <div className="public-page-brand-heading">
          <p className="public-page-eyebrow">A little more connected</p>
          <h2>Good work<br />happens together.</h2>
        </div>
        <PublicPageVortex />
      </aside>
    </div>
  );
}
