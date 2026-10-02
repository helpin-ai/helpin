import type { ReactNode } from 'react';
import { resolveTeamColor, teamColorStyle } from '@/lib/teamColor';
import { cn } from '@/lib/utils';

/** Decorative team identity; the adjacent team name supplies the accessible label. */
export function TeamBadge({ name, color, className, children }: {
  name: string;
  color?: string | null;
  className?: string;
  children?: ReactNode;
}) {
  const style = teamColorStyle(resolveTeamColor(name, color));
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex h-[20px] w-[20px] shrink-0 items-center justify-center rounded-[4px] text-xs font-semibold',
        'bg-[var(--team-color-bg)] text-[var(--team-color-fg)]',
        className,
      )}
      style={style}
    >
      {children ?? name.trim().charAt(0).toUpperCase()}
    </span>
  );
}
