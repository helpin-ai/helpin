import type { CSSProperties } from 'react';
import { cn } from '@/lib/utils';
import { resolveEpicColor } from './epicColor';

export function EpicBadge({ name, color, className }: { name: string; color?: string | null; className?: string }) {
  return (
    <span
      title={name}
      className={cn(
        'inline-flex min-w-0 max-w-full items-center rounded-sm px-1.5 py-0.5 text-xs font-medium',
        'bg-[color-mix(in_srgb,var(--epic-color)_12%,transparent)] text-[color-mix(in_srgb,var(--epic-color)_25%,var(--foreground))]',
        className,
      )}
      style={{ '--epic-color': resolveEpicColor(color) } as CSSProperties}
    >
      <span className="truncate">{name}</span>
    </span>
  );
}
