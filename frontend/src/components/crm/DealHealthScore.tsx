import { cn } from '@/lib/utils';
import { useDealHealthScore } from '@/hooks/queries/useCRM';

interface DealHealthScoreProps {
  workspaceId: string;
  dealId: string;
  compact?: boolean;
}

function getScoreColor(score: number): string {
  if (score >= 80) return 'text-green-500';
  if (score >= 60) return 'text-yellow-500';
  if (score >= 40) return 'text-orange-500';
  return 'text-red-500';
}

function getScoreBgColor(score: number): string {
  if (score >= 80) return 'bg-green-500';
  if (score >= 60) return 'bg-yellow-500';
  if (score >= 40) return 'bg-orange-500';
  return 'bg-red-500';
}

export function DealHealthScore({ workspaceId, dealId, compact }: DealHealthScoreProps) {
  const { data: healthScore } = useDealHealthScore(workspaceId, dealId);

  if (!healthScore) return null;

  const score = healthScore.score;
  const factors = healthScore.factors as Record<string, unknown>;

  if (compact) {
    return (
      <span className={cn('text-sm font-semibold', getScoreColor(score))}>
        {score}
      </span>
    );
  }

  return (
    <div className="rounded-md border p-4">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-medium">Deal Health</h4>
        <span className={cn('text-2xl font-bold', getScoreColor(score))}>{score}</span>
      </div>
      <div className="mt-2 h-2 w-full overflow-hidden rounded-full bg-muted">
        <div
          className={cn('h-full rounded-full transition-all', getScoreBgColor(score))}
          style={{ width: `${score}%` }}
        />
      </div>
      {factors && Object.keys(factors).length > 0 && (
        <div className="mt-3 space-y-1">
          {Object.entries(factors).map(([key, value]) => (
            <div key={key} className="flex items-center justify-between text-xs">
              <span className="text-muted-foreground">{key.replace(/_/g, ' ')}</span>
              <span className="font-medium">{String(value)}</span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
