import type { ReactNode } from 'react';
import { getAvatarColor } from '@/components/pm/UserAvatar';
import { teamColorStyle } from '@/lib/teamColor';
import { cn, getInitials } from '@/lib/utils';

/** Decorative team identity; the adjacent team name supplies the accessible label. */
export function TeamBadge({ name, color, className, children }: {
  name: string;
  color?: string | null;
  className?: string;
  children?: ReactNode;
}) {
  const style = teamColorStyle(color);
  const fallback = getAvatarColor(name.toLowerCase());
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-xs font-semibold',
        style
          ? 'bg-[var(--team-color-bg)] text-[var(--team-color-fg)] dark:bg-[var(--team-color-bg-dark)] dark:text-[var(--team-color-fg-dark)]'
          : [fallback.bg, fallback.text],
        className,
      )}
      style={style}
    >
      {children ?? getInitials(name)}
    </span>
  );
}
