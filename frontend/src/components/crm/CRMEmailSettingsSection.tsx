import type { ReactNode } from 'react';
import { ArrowRight01Icon } from '@/lib/icons';

/** Native disclosure keeps unsaved form state mounted when closed. */
export function CRMEmailSettingsSection({ title, summary, children }: {
  title: string;
  summary?: string;
  children: ReactNode;
}) {
  return (
    <details className="group/email-section border-b border-quiet-divider-light py-4">
      <summary className="flex cursor-pointer list-none items-center gap-3 rounded-sm focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-quiet-field [&::-webkit-details-marker]:hidden">
        <ArrowRight01Icon className="h-3.5 w-3.5 shrink-0 text-muted-foreground transition-transform group-open/email-section:rotate-90" />
        <span className="min-w-0 flex-1">
          <span className="block text-sm font-medium">{title}</span>
          {summary && <span className="mt-1 block text-xs text-muted-foreground">{summary}</span>}
        </span>
      </summary>
      <div className="pt-5">{children}</div>
    </details>
  );
}
