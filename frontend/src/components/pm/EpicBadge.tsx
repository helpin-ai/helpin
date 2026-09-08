import { cn } from '@/lib/utils';
import { getEpicBadgeTextColor, resolveEpicColor } from './epicColor';

export function EpicBadge({ name, color, className }: { name: string; color?: string | null; className?: string }) {
  const backgroundColor = resolveEpicColor(color);
  return (
    <span
      title={name}
      className={cn(
        'inline-flex min-w-0 max-w-full items-center rounded-sm px-1.5 py-0.5 text-xs font-medium',
        className,
      )}
      style={{ backgroundColor, color: getEpicBadgeTextColor(backgroundColor) }}
    >
      <span className="truncate">{name}</span>
    </span>
  );
}
