import type { ReactNode } from 'react';

import { cn } from '@/lib/utils';

export function StreamingStatusText({
  children,
  className,
}: {
  children: ReactNode;
  className?: string;
}) {
  return (
    <div className={cn('text-sm leading-6', className)} data-agent-streaming-status>
      <span className="agent-streaming-text" role="status" aria-live="polite">
        {children}
      </span>
    </div>
  );
}
