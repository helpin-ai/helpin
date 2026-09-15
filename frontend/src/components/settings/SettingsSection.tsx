import { useState, type ReactNode } from 'react';
import { ArrowDown01Icon } from '@/lib/icons';
import { Card } from '@/components/ui/card';
import { cn } from '@/lib/utils';

/** Settings disclosure with the same section chrome as Support routing. Content stays mounted. */
export function SettingsSection({ title, description, defaultOpen = false, className, children }: {
  title: string;
  description?: string;
  defaultOpen?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);
  return (
    <Card className={cn('gap-0 rounded-lg border-border/70 py-0', className)}>
      <details open={open} onToggle={(event) => setOpen(event.currentTarget.open)}>
        <summary className="flex cursor-pointer list-none items-center gap-3 px-4 py-4 transition-colors hover:bg-muted/40 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring [&::-webkit-details-marker]:hidden">
          <span className="min-w-0 flex-1">
            <span role="heading" aria-level={2} className="block text-sm font-semibold text-quiet-text-primary">{title}</span>
            {description && <span className="mt-0.5 block text-xs text-quiet-text-secondary">{description}</span>}
          </span>
          <ArrowDown01Icon aria-hidden className={cn('size-4 shrink-0 text-quiet-text-secondary transition-transform', open && 'rotate-180')} />
        </summary>
        <div className="border-t border-quiet-divider-strong px-4 pb-4 pt-1">{children}</div>
      </details>
    </Card>
  );
}
