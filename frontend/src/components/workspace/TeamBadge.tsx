import type { ReactNode } from 'react';
import { resolveTeamColor, teamColorStyle, TEAM_PRESET_COLORS } from '@/lib/teamColor';
import { cn } from '@/lib/utils';

/** Decorative team identity; the adjacent team name supplies the accessible label. */
export function TeamBadge({ name, color, className, children }: {
  name: string;
  color?: string | null;
  className?: string;
  children?: ReactNode;
}) {
  const resolvedColor = resolveTeamColor(name, color);
  const style = teamColorStyle(resolvedColor);
  // Preset badges use the white initials shown in the team identity design.
  // Custom shades retain their contrast-aware text (including white backgrounds).
  if (style && TEAM_PRESET_COLORS.includes(resolvedColor)) style['--team-color-fg'] = '#ffffff';
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
