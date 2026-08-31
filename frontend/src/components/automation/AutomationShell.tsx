import type { ReactNode } from 'react';
import { QuietPageHeader } from '@/components/design-system/quiet';
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
    <div className={cn('space-y-5', className)}>
      <QuietPageHeader title={title} description={description} actions={actions} />
      {children}
    </div>
  );
}
