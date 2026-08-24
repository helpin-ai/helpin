import type { ReactNode } from 'react';

import { ArrowLeft02Icon, ArrowRight01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

interface DesktopDetailRailProps {
  collapsed: boolean;
  onCollapsedChange: (collapsed: boolean) => void;
  children: ReactNode;
  className?: string;
  contentClassName?: string;
  label?: string;
}

export function DesktopDetailRail({
  collapsed,
  onCollapsedChange,
  children,
  className,
  contentClassName,
  label = 'Details',
}: DesktopDetailRailProps) {
  return (
    <aside aria-label={`${label} rail`} className={cn('hidden min-h-0 border-l border-border/60 lg:block', className)}>
      {collapsed ? (
        <button
          type="button"
          className="group flex h-full w-full flex-col items-center bg-background py-3 text-muted-foreground transition-colors hover:bg-muted/30 hover:text-foreground"
          onClick={() => onCollapsedChange(false)}
          aria-expanded={false}
          aria-label={`Open ${label.toLowerCase()}`}
          title={`Open ${label.toLowerCase()}`}
        >
          <ArrowLeft02Icon className="h-4 w-4" />
          <span className="mt-4 text-[10px] font-semibold uppercase tracking-[0.12em] [writing-mode:vertical-rl]">
            {label}
          </span>
        </button>
      ) : (
        <div className="flex h-full min-h-0 min-w-0 flex-col">
          <div className="flex h-9 shrink-0 items-center justify-between border-b border-border/50 px-3">
            <span className="text-[11px] font-semibold uppercase tracking-[0.08em] text-muted-foreground">{label}</span>
            <button
              type="button"
              className="rounded p-1 text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              onClick={() => onCollapsedChange(true)}
              aria-expanded={true}
              aria-label={`Minimize ${label.toLowerCase()}`}
              title={`Minimize ${label.toLowerCase()}`}
            >
              <ArrowRight01Icon className="h-4 w-4" />
            </button>
          </div>
          <div className={cn('min-h-0 min-w-0 flex-1 overflow-y-auto', contentClassName)}>{children}</div>
        </div>
      )}
    </aside>
  );
}
