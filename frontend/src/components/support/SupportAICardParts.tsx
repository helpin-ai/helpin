import type { ReactNode } from 'react';
import { AskAgentAvatar } from '@/components/agents/AskAgentAvatar';
import { formatMessageTime, formatTimestamp } from './helpers';

const cardRow = 'grid grid-cols-[5.5rem_minmax(0,1fr)] items-baseline gap-x-2.5 border-t border-amber-200/60 py-2 first:border-t-0 first:pt-0 dark:border-amber-900/40';
const cardFrame = 'w-full max-w-xl rounded-lg border border-amber-200/70 bg-amber-50/60 p-3 dark:border-amber-900/50 dark:bg-amber-950/20';

export function CardRow({ label, children }: { label: string; children: ReactNode }) {
  return <div className={cardRow}>
    <dt className="text-xs text-muted-foreground">{label}</dt>
    <dd className="min-w-0">{children}</dd>
  </div>;
}

export function CardList({ items }: { items: string[] }) {
  return items.length === 1 ? <>{items[0]}</> : <ul className="space-y-1">{items.map((item, index) => <li key={index}>{item}</li>)}</ul>;
}

/**
 * Shared frame for AI cards in the thread. Older cards fold to one line that
 * opens to the full card.
 */
export function SupportAICard({ kind, title, timestamp, folded = false, summary, children }: {
  kind: 'handoff' | 'code-check';
  title: string;
  timestamp: string;
  folded?: boolean;
  summary?: string;
  children: ReactNode;
}) {
  const label = kind === 'handoff' ? 'AI handoff summary' : 'Code check';
  const dataAttributes = kind === 'handoff' ? { 'data-support-ai-handoff': '' } : { 'data-support-code-check': '' };
  const header = <>
    <AskAgentAvatar plateStyle="solid" radius={50} className="h-5 w-5 shrink-0" />
    <span className="shrink-0 text-xs font-semibold">{title}</span>
    {folded && summary
      ? <span className="min-w-0 truncate text-xs text-muted-foreground">{summary}</span>
      : <span className="text-[11px] text-muted-foreground">Team only</span>}
    <time dateTime={timestamp} title={formatTimestamp(timestamp)} className="ml-auto shrink-0 text-[11px] text-muted-foreground">{formatMessageTime(timestamp)}</time>
  </>;
  return <div className="my-4 flex justify-end" {...dataAttributes}>
    {folded ? (
      <details aria-label={label} className={`${cardFrame} group py-2`}>
        <summary className="flex cursor-pointer list-none items-center gap-2 rounded-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [&::-webkit-details-marker]:hidden">{header}</summary>
        <div className="mt-2">{children}</div>
      </details>
    ) : (
      <section aria-label={label} className={cardFrame}>
        <div className="mb-2 flex items-center gap-2">{header}</div>
        {children}
      </section>
    )}
  </div>;
}
