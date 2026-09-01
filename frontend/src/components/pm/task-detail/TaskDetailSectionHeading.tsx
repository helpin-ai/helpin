import type { ElementType, ReactNode } from 'react';

import { QuietSectionHeader } from '@/components/design-system/quiet';
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
    <QuietSectionHeader
      title={title}
      icon={Icon}
      action={meta}
      className={cn('border-b border-quiet-divider-light pb-2 pt-1.5', className)}
    />
  );
}
