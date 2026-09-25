import { useState, type ReactNode } from 'react';
import { Link } from '@tanstack/react-router';
import { QuietIconAction } from '@/components/design-system/quiet';
import { ArrowRight01Icon, Copy01Icon, Tick01Icon } from '@/lib/icons';
import type { Capability } from '@/lib/capabilityTypes';
import { cn } from '@/lib/utils';

export const setupTextActionClassName =
  'inline-flex items-center gap-1 text-[12.5px] font-medium text-quiet-text-primary underline decoration-quiet-field underline-offset-4 transition-colors hover:decoration-quiet-text-primary focus-visible:rounded-sm focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-quiet-text-primary';

/** Link to a workspace-relative route such as `settings/ai`. */
export function SetupSettingsLink({ slug, path, children }: { slug: string; path: string; children: ReactNode }) {
  const to = `/w/${slug}/${path.replace(/^\/+/, '')}`;
  return (
    <Link to={to} className={setupTextActionClassName}>
      {children}
      <ArrowRight01Icon className="h-3.5 w-3.5" aria-hidden="true" />
    </Link>
  );
}

/** Shown in place of admin-only actions for members without workspace settings access. */
export function SetupAdminHint() {
  return <p className="text-[12.5px] text-quiet-text-tertiary">A workspace admin can finish this step.</p>;
}

/**
 * Server configuration guidance. It is instructions for whoever runs the
 * server, so it is copyable text rather than a button.
 */
export function ServerConfigHint({ hint }: { hint: string }) {
  const [copied, setCopied] = useState(false);
  const copy = async () => {
    try {
      await navigator.clipboard?.writeText(hint);
      setCopied(true);
      window.setTimeout(() => setCopied(false), 2000);
    } catch {
      setCopied(false);
    }
  };
  return (
    <div className="flex min-w-0 items-start gap-1.5">
      <div className="min-w-0">
        <span className="block text-[11.5px] font-semibold uppercase tracking-[0.03em] text-quiet-muted">Server configuration</span>
        <code className="mt-1 block whitespace-pre-wrap break-words rounded-[6px] bg-quiet-icon-well px-2 py-1 font-mono text-[12px] leading-5 text-quiet-text-secondary">{hint}</code>
      </div>
      <QuietIconAction className="mt-5 shrink-0" aria-label={copied ? 'Copied' : 'Copy server configuration'} onClick={() => void copy()}>
        {copied ? <Tick01Icon className="h-[15px] w-[15px]" aria-hidden="true" /> : <Copy01Icon className="h-[15px] w-[15px]" aria-hidden="true" />}
      </QuietIconAction>
    </div>
  );
}

export type SetupResult = { tone: 'positive' | 'negative'; message: string };

/**
 * Polite live region for inline check results. It stays mounted, even when
 * empty, so screen readers announce the text when it appears.
 */
export function SetupResultMessage({ result, className }: { result: SetupResult | null; className?: string }) {
  return (
    <p
      role="status"
      aria-live="polite"
      className={cn(
        // sr-only rather than hidden, so the region stays in the accessibility tree.
        'text-[12.5px] leading-5 empty:sr-only',
        result?.tone === 'positive' ? 'text-quiet-positive' : 'text-quiet-accent',
        className,
      )}
    >
      {result ? result.message : ''}
    </p>
  );
}

/**
 * Generic next action for a capability without a dedicated inline step.
 * Inline tests are handled by the dedicated steps; here they fall back to settings.
 */
export function CapabilityActionView({ capability, slug, canManage }: { capability: Capability; slug: string; canManage: boolean }) {
  const action = capability.action;
  if (!action || capability.status === 'ready' || capability.status === 'unavailable') return null;
  if (action.kind === 'server_config') return <ServerConfigHint hint={action.label} />;
  if (!canManage) return <SetupAdminHint />;
  if (action.path) return <SetupSettingsLink slug={slug} path={action.path}>{action.label}</SetupSettingsLink>;
  return null;
}
