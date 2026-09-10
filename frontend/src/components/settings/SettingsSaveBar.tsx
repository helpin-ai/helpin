import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

/** Place first in the settings form so controls stay in its scroll viewport. */
export function SettingsSaveBar({ children, className, visible = true }: {
  children?: ReactNode;
  className?: string;
  visible?: boolean;
}) {
  if (!visible) return null;
  return (
    <div className={cn('sticky top-0 z-10 flex min-h-12 flex-wrap items-center justify-end gap-x-3 gap-y-2 border-b border-border bg-background py-2', className)}>
      {children}
    </div>
  );
}
