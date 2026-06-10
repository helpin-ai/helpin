import type { ReactNode } from 'react';
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
    <div className={cn('mx-auto max-w-7xl space-y-5', className)}>
      <div className="flex items-start justify-between gap-4">
        <div className="space-y-1">
          <h1 className="text-xl font-semibold">{title}</h1>
          {description && <p className="text-sm text-muted-foreground">{description}</p>}
        </div>
        {actions && <div className="shrink-0">{actions}</div>}
      </div>
      {children}
    </div>
  );
}
