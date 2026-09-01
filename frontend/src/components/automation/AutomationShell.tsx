import type { ReactNode } from 'react';
import { QuietPageHeader, QuietPageViewport } from '@/components/design-system/quiet';
import { cn } from '@/lib/utils';

export function AutomationShell({
  title,
  description,
  actions,
  children,
  className,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className="flex h-full min-h-0 flex-col">
      <QuietPageHeader variant="shell" title={title} description={description} actions={actions} />
      <QuietPageViewport className="min-h-0 flex-1">
        <div className={cn('space-y-5', className)}>{children}</div>
      </QuietPageViewport>
    </div>
  );
}
