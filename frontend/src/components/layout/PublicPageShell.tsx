import type { ReactNode } from 'react';
import { useTheme } from 'next-themes';
import { HelpinLogo } from '@/components/layout/HelpinLogo';
import { Moon02Icon, Sun01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';
import { PublicPageVortex } from './PublicPageVortex';
import './public-page-shell.css';

interface PublicPageShellProps {
  children: ReactNode;
  /** Replaces the "Back to website" link, for example with Sign out during onboarding. */
  headerAction?: ReactNode;
  /** `wide` fits multi-part forms such as workspace onboarding. */
  contentWidth?: 'narrow' | 'wide';
  /** Hide the brand panel for task pages such as onboarding; sign-in and signup keep it. */
  brandPanel?: boolean;
  /** Show a light and dark mode switch in the header. */
  themeToggle?: boolean;
}

export function PublicPageShell({ children, headerAction, contentWidth = 'narrow', brandPanel = true, themeToggle = false }: PublicPageShellProps) {
  const action = headerAction ?? (
    <a href="https://helpin.ai" className="public-page-back">Back to website <span aria-hidden="true">↗</span></a>
  );
  return (
    <div className={cn('public-page', !brandPanel && 'public-page--solo')}>
      <div className="public-page-main">
        <header className="public-page-header">
          <a href="https://helpin.ai" aria-label="Helpin home"><HelpinLogo /></a>
          {themeToggle ? <div className="flex items-center gap-3"><ThemeToggle />{action}</div> : action}
        </header>
        <main className={cn('public-page-content', contentWidth === 'wide' && 'public-page-content--wide')}>{children}</main>
        <div className="public-page-footer" aria-hidden="true" />
      </div>
      {brandPanel && (
        <aside className="public-page-brand" aria-label="Helpin: good work happens together">
          <div className="public-page-brand-heading">
            <p className="public-page-eyebrow">A little more connected</p>
            <h2>Good work<br />happens together.</h2>
          </div>
          <PublicPageVortex />
        </aside>
      )}
    </div>
  );
}

function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme();
  const dark = resolvedTheme === 'dark';
  return (
    <button
      type="button"
      aria-label={dark ? 'Switch to light mode' : 'Switch to dark mode'}
      onClick={() => setTheme(dark ? 'light' : 'dark')}
      className="flex h-9 w-9 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
    >
      {dark ? <Sun01Icon className="h-4 w-4" aria-hidden="true" /> : <Moon02Icon className="h-4 w-4" aria-hidden="true" />}
    </button>
  );
}
