import type { ReactNode } from 'react';
import { BotIcon, GitBranchIcon, Loading01Icon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { buildAutomationRunsPath } from '@/lib/automationUi';
import type { CodingSession } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { formatCodingSessionRelative } from './codingSessionUtils';

export function CodingSessionHeader({
  session,
  statusIcon,
  workspaceSlug,
  onRefresh,
  refreshing = false,
  acting = null,
  onCancelRun,
}: {
  session: CodingSession | null;
  statusIcon: ReactNode;
  workspaceSlug?: string;
  onRefresh: () => void;
  refreshing?: boolean;
  acting?: string | null;
  onCancelRun?: () => void;
}) {
  const canCancel = session !== null
    && session.status !== 'completed'
    && session.status !== 'cancelled'
    && session.status !== 'failed';

  return (
    <div className="space-y-3">
      {/* Top row: Forge identity + actions */}
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          {/* Forge icon */}
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <BotIcon className="h-5 w-5" />
          </div>

          {/* Forge label + repo info */}
          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="text-base font-semibold leading-tight">Forge</h1>
              <Badge variant="outline" className={cn(
                'gap-1 px-2 py-0.5 text-[11px] font-medium',
                session?.status === 'running' && 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-400',
                session?.status === 'completed' && 'border-emerald-300 bg-emerald-50 text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/30 dark:text-emerald-400',
                session?.status === 'failed' && 'border-destructive/30 bg-destructive/5 text-destructive',
                session?.status === 'cancelled' && 'border-muted-foreground/30 text-muted-foreground',
                session?.status === 'paused' && 'border-amber-300 bg-amber-50 text-amber-700 dark:border-amber-800 dark:bg-amber-950/30 dark:text-amber-400',
              )}>
                {statusIcon}
                {session?.status === 'running' ? 'Running' : session?.status ?? 'Loading'}
              </Badge>
              {session?.pause_reason && session.pause_reason !== 'none' ? (
                <Badge variant="secondary" className="px-2 py-0.5 text-[11px]">
                  {session.pause_reason.replaceAll('_', ' ')}
                </Badge>
              ) : null}
            </div>
            {/* Repo + branch pills */}
            <div className="mt-1 flex flex-wrap items-center gap-1.5">
              {session?.repo.repo_name ? (
                <span className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-foreground">
                  {session.repo.repo_name}
                </span>
              ) : null}
              {session?.repo.branch ? (
                <span className="inline-flex items-center gap-1 rounded-md bg-violet-100 px-2 py-0.5 text-[11px] font-medium text-violet-700 dark:bg-violet-950/30 dark:text-violet-400">
                  <GitBranchIcon className="h-3 w-3" />
                  {session.repo.branch}
                </span>
              ) : null}
              {session ? (
                <span className="inline-flex items-center rounded-md bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                  {session.runtime_kind}
                </span>
              ) : null}
              {session?.updated_at ? (
                <span className="text-[11px] text-muted-foreground">
                  Updated {formatCodingSessionRelative(session.updated_at)}
                </span>
              ) : null}
            </div>
          </div>
        </div>

        {/* Actions */}
        <div className="flex flex-wrap items-center gap-2">
          {workspaceSlug ? (
            <Button asChild variant="outline" size="sm">
              <a href={buildAutomationRunsPath(workspaceSlug)}>
                Back to runs
              </a>
            </Button>
          ) : null}
          {onCancelRun ? (
            <Button variant="outline" size="sm" onClick={onCancelRun} disabled={acting !== null || !canCancel}>
              {acting === 'cancel' ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
              Cancel run
            </Button>
          ) : null}
          <Button variant="outline" size="sm" onClick={onRefresh} disabled={refreshing}>
            Refresh
          </Button>
        </div>
      </div>

      {/* Session title + summary */}
      {session?.title ? (
        <div className="min-w-0">
          <p className="text-sm font-medium text-foreground">{session.title}</p>
          {session.summary ? (
            <p className="mt-0.5 text-xs text-muted-foreground">{session.summary}</p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
