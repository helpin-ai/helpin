import { cn } from '@/lib/utils';
import type { ActivityState } from './utils';

export type DotKind = ActivityState | 'active_step' | 'blocked';

interface StatusDotProps {
  state: DotKind;
  size?: 'sm' | 'md';
  className?: string;
}

/**
 * Single source of truth for the dock's status dot semantics.
 * - completed: filled green
 * - running: hollow ember ring (actively executing)
 * - active_step: hollow ember ring + halo (the currently-executing pipeline step)
 * - awaiting: filled amber dot (paused waiting on a human — approval/input/auth)
 * - attention: filled red dot (failed — something broke)
 * - cancelled: hollow muted dot (user-initiated stop, terminal)
 * - blocked: dashed muted ring (a pipeline step waiting on a dep)
 * - queued: filled muted dot (waiting to start)
 */
export function StatusDot({ state, size = 'sm', className }: StatusDotProps) {
  const dim = size === 'md' ? 'h-2.5 w-2.5' : 'h-2 w-2';
  switch (state) {
    case 'completed':
      return (
        <span
          aria-hidden
          className={cn('inline-block shrink-0 rounded-full bg-emerald-500', dim, className)}
        />
      );
    case 'running':
      return (
        <span
          aria-hidden
          className={cn(
            'inline-block shrink-0 rounded-full border-[1.5px] border-orange-500 bg-transparent',
            dim,
            className,
          )}
        />
      );
    case 'active_step':
      return (
        <span
          aria-hidden
          className={cn(
            'relative inline-flex shrink-0 items-center justify-center',
            dim,
            className,
          )}
        >
          <span className="absolute inset-0 animate-ping rounded-full bg-orange-500/40" />
          <span className="relative inline-block h-full w-full rounded-full border-[1.5px] border-orange-500" />
        </span>
      );
    case 'awaiting':
      return (
        <span
          aria-hidden
          className={cn(
            'inline-block shrink-0 rounded-full bg-amber-500 dark:bg-amber-400',
            dim,
            className,
          )}
        />
      );
    case 'attention':
      return (
        <span
          aria-hidden
          className={cn(
            'inline-block shrink-0 rounded-full border-[1.5px] border-destructive bg-destructive/80',
            dim,
            className,
          )}
        />
      );
    case 'cancelled':
      return (
        <span
          aria-hidden
          className={cn(
            'inline-block shrink-0 rounded-full border border-muted-foreground/40 bg-transparent',
            dim,
            className,
          )}
        />
      );
    case 'blocked':
      return (
        <span
          aria-hidden
          className={cn(
            'inline-block shrink-0 rounded-full border border-dashed border-muted-foreground/60 bg-transparent',
            dim,
            className,
          )}
        />
      );
    case 'queued':
    default:
      return (
        <span
          aria-hidden
          className={cn(
            'inline-block shrink-0 rounded-full bg-muted-foreground/40',
            dim,
            className,
          )}
        />
      );
  }
}
