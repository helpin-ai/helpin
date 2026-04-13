import { ArrowReloadHorizontalIcon } from '@/lib/pmIcons';
import { Badge } from '@/components/ui/badge';
import { cn } from '@/lib/utils';

interface RecurringTemplateBadgeProps {
  occurrenceNumber?: number;
  className?: string;
  compact?: boolean;
}

export function RecurringTemplateBadge({
  occurrenceNumber,
  className,
  compact = false,
}: RecurringTemplateBadgeProps) {
  return (
    <Badge
      variant="outline"
      className={cn(
        'gap-1 border-sky-200 bg-sky-50 text-sky-700 dark:border-sky-900/60 dark:bg-sky-950/30 dark:text-sky-200',
        compact ? 'px-1.5 py-0 text-[10px]' : 'px-2 py-0.5 text-[11px]',
        className,
      )}
    >
      <ArrowReloadHorizontalIcon className={cn('shrink-0', compact ? 'h-2.5 w-2.5' : 'h-3 w-3')} />
      <span className="truncate">{occurrenceNumber ? `Recurring #${occurrenceNumber}` : 'Recurring'}</span>
    </Badge>
  );
}
