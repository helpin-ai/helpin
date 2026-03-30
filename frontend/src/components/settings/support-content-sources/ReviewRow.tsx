import type { ReactNode } from 'react';

export function ReviewRow({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="grid gap-1 rounded-lg border border-border/60 bg-muted/20 px-3 py-2 sm:grid-cols-[160px_minmax(0,1fr)] sm:items-start">
      <span className="text-xs font-medium uppercase tracking-wide text-muted-foreground">{label}</span>
      <div className="text-sm text-foreground">{value}</div>
    </div>
  );
}
