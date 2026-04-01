import type { ReactNode } from 'react';

import { cn } from '@/lib/utils';

interface BoardToolbarSlotProps {
  children: ReactNode;
  className?: string;
}

export function BoardToolbarSlot({ children, className }: BoardToolbarSlotProps) {
  return (
    <div
      data-testid="board-toolbar-slot"
      className={cn('flex h-7 items-center', className)}
    >
      {children}
    </div>
  );
}
