import type { ReactNode } from 'react';
import { Folder01Icon, GitBranchIcon, Loading01Icon, CancelCircleIcon, ArrowReloadHorizontalIcon } from '@/lib/icons';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { buildAutomationRunsPath } from '@/lib/automationUi';
import type { CodingSession } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatCodingSessionRelative } from './codingSessionUtils';

function capitalize(text: string) {
  return text.replace(/\b\w/g, (c) => c.toUpperCase());
}

function resolveSessionAgentName(session: CodingSession | null) {
  const title = session?.title?.trim();
  if (!title || title.toLowerCase() === 'coding session') {
    return 'Agent';
  }
  return title;
}

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
  const agentName = resolveSessionAgentName(session);
  const sessionTitle = session?.title?.trim() ?? '';
  const sessionSummary = session?.summary?.trim() ?? '';
  const showDetailBlock = (sessionTitle && sessionTitle !== agentName) || !!sessionSummary;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <AgentAvatar name={agentName} className="h-9 w-9 shrink-0" />

          <div className="min-w-0">
            <div className="flex items-center gap-2">
              <h1 className="text-base font-semibold leading-tight">{agentName}</h1>
              <Badge variant="outline" className="gap-1 px-2 py-0.5 text-[11px] font-medium capitalize">
                {statusIcon}
                {session?.status === 'running' ? 'Running' : capitalize(session?.status ?? 'Loading')}
              </Badge>
              {session?.pause_reason && session.pause_reason !== 'none' ? (
                <Badge variant="outline" className="px-2 py-0.5 text-[11px] capitalize">
                  {capitalize(session.pause_reason.replaceAll('_', ' '))}
                </Badge>
              ) : null}
            </div>

            <div className="mt-1 flex flex-wrap items-center gap-1.5">
              {session?.repo.repo_name ? (
                <a
                  href={`https://github.com/${session.repo.repo_name}`}
                  target="_blank"
                  rel="noreferrer"
                  className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-foreground transition-colors hover:bg-muted/80 hover:text-primary"
                >
                  <Folder01Icon className="h-3 w-3 text-muted-foreground" />
                  {session.repo.repo_name}
                </a>
              ) : null}
              {session?.repo.branch ? (
                session.repo.repo_name ? (
                  <a
                    href={`https://github.com/${session.repo.repo_name}/tree/${session.repo.branch}`}
                    target="_blank"
                    rel="noreferrer"
                    className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-foreground transition-colors hover:bg-muted/80 hover:text-primary"
                  >
                    <GitBranchIcon className="h-3 w-3 text-muted-foreground" />
                    {session.repo.branch}
                  </a>
                ) : (
                  <span className="inline-flex items-center gap-1 rounded-md bg-muted px-2 py-0.5 text-[11px] font-medium text-foreground">
                    <GitBranchIcon className="h-3 w-3 text-muted-foreground" />
                    {session.repo.branch}
                  </span>
                )
              ) : null}
            </div>

            <div className="mt-1 flex flex-wrap items-center gap-1.5 text-[11px] text-muted-foreground">
              {session ? <span>{session.runtime_kind}</span> : null}
              {session?.updated_at ? (
                <>
                  <span>·</span>
                  <span>Updated {formatCodingSessionRelative(session.updated_at)}</span>
                </>
              ) : null}
            </div>
          </div>
        </div>

        {/* Actions — pr-10 reserves space for the sheet close button */}
        <div className="flex items-center gap-1.5 pr-10">
          {workspaceSlug ? (
            <Button asChild variant="outline" size="sm">
              <a href={buildAutomationRunsPath(workspaceSlug)}>
                Back to runs
              </a>
            </Button>
          ) : null}
          {onCancelRun ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="outline" size="icon" className="h-8 w-8" onClick={onCancelRun} disabled={acting !== null || !canCancel}>
                  {acting === 'cancel' ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <CancelCircleIcon className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>Cancel run</TooltipContent>
            </Tooltip>
          ) : null}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button variant="outline" size="icon" className="h-8 w-8" onClick={onRefresh} disabled={refreshing}>
                <ArrowReloadHorizontalIcon className={cn('h-3.5 w-3.5', refreshing && 'animate-spin')} />
              </Button>
            </TooltipTrigger>
            <TooltipContent>Refresh</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {showDetailBlock ? (
        <div className="min-w-0">
          {sessionTitle && sessionTitle !== agentName ? (
            <p className="text-sm font-medium text-foreground">{sessionTitle}</p>
          ) : null}
          {sessionSummary ? (
            <p className={sessionTitle && sessionTitle !== agentName ? 'mt-0.5 text-xs text-muted-foreground' : 'text-xs text-muted-foreground'}>
              {sessionSummary}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
