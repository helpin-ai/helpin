import type { ElementType, ReactNode } from 'react';

import { cn } from '@/lib/utils';

interface TaskDetailSectionHeadingProps {
  title: string;
  icon?: ElementType;
  meta?: ReactNode;
  className?: string;
}

export function TaskDetailSectionHeading({
  title,
  icon: Icon,
  meta,
  className,
}: TaskDetailSectionHeadingProps) {
  return (
    <div className={cn('border-b border-border/50 pb-2', className)}>
      <div className="flex items-center gap-1.5">
        {Icon ? <Icon className="h-3.5 w-3.5 text-muted-foreground" /> : null}
        <h3 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">
          {title}
        </h3>
        {meta}
      </div>
    </div>
  );
}
