import type { ReactNode } from 'react';
import { GitBranchIcon, Loading01Icon } from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { buildAutomationRunsPath } from '@/lib/automationUi';
import type { CodingSession } from '@/lib/pmTypes';
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
    <div className="space-y-2">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 space-y-1.5">
          <h1 className="text-xl font-semibold">
            {session?.title ?? 'Agent Session'}
          </h1>
          {session?.summary ? (
            <p className="text-sm text-muted-foreground">
              {session.summary}
            </p>
          ) : null}
        </div>

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

      <div className="flex flex-wrap items-center gap-2">
        <Badge variant="outline" className="gap-1.5 text-xs">
          {statusIcon}
          {session?.status ?? 'Loading'}
        </Badge>
        {session?.pause_reason && session.pause_reason !== 'none' ? (
          <Badge variant="secondary" className="text-xs">
            {session.pause_reason.replaceAll('_', ' ')}
          </Badge>
        ) : null}
        {session ? (
          <Badge variant="secondary" className="text-xs">
            {session.runtime_kind}
          </Badge>
        ) : null}
        {session ? (
          <Badge variant="secondary" className="text-xs">
            {session.invocation_mode}
          </Badge>
        ) : null}
      </div>

      <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
        <span>{session?.repo.repo_name ?? 'No repository linked'}</span>
        {session?.repo.branch ? (
          <>
            <span>•</span>
            <span className="inline-flex items-center gap-1">
              <GitBranchIcon className="h-3 w-3" />
              {session.repo.branch}
            </span>
          </>
        ) : null}
        {session?.updated_at ? (
          <>
            <span>•</span>
            <span>Updated {formatCodingSessionRelative(session.updated_at)}</span>
          </>
        ) : null}
      </div>
    </div>
  );
}
