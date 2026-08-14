import { Progress } from '@/components/ui/progress';
import { cn } from '@/lib/utils';

interface InlineCompletionProgressProps {
  completed: number;
  total: number;
  showCount?: boolean;
  className?: string;
  testIdPrefix?: string;
}

export function InlineCompletionProgress({
  completed,
  total,
  showCount = true,
  className,
  testIdPrefix,
}: InlineCompletionProgressProps) {
  if (total <= 0) return null;

  const safeCompleted = Math.min(Math.max(completed, 0), total);
  const percentage = Math.round((safeCompleted / total) * 100);

  return (
    <div className={cn('flex min-w-0 flex-1 items-center gap-1.5', className)}>
      {showCount ? (
        <span
          className="shrink-0 text-xs font-normal text-muted-foreground"
          data-testid={testIdPrefix ? `${testIdPrefix}-count` : undefined}
        >
          ({safeCompleted}/{total})
        </span>
      ) : null}
      <Progress
        value={percentage}
        aria-label={`${safeCompleted} of ${total} complete`}
        className="mx-1 h-1.5 min-w-12 flex-1 bg-muted"
        indicatorClassName="bg-green-500"
        data-testid={testIdPrefix ? `${testIdPrefix}-progress` : undefined}
      />
      <span
        className="shrink-0 text-xs tabular-nums text-muted-foreground"
        data-testid={testIdPrefix ? `${testIdPrefix}-percentage` : undefined}
      >
        {percentage}%
      </span>
    </div>
  );
}
