import { cn } from '@/lib/utils';
import { EpicColorSwatch } from './EpicColorSwatch';

export function EpicBadge({ name, color, className }: { name: string; color?: string | null; className?: string }) {
  return (
    <span
      title={name}
      className={cn(
        'inline-flex min-w-0 max-w-full items-center gap-1.5 text-xs text-muted-foreground',
        className,
      )}
    >
      <EpicColorSwatch color={color} />
      <span className="truncate">{name}</span>
    </span>
  );
}
