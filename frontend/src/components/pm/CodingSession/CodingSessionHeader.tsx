import { useCallback, useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from 'react';
import { ArrowDown02Icon, ArrowReloadHorizontalIcon, ArrowUp02Icon, CancelCircleIcon, Folder01Icon, GitBranchIcon, Loading01Icon } from '@/lib/icons';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { formatSessionTokenUsage } from '@/lib/agentTokenUsage';
import { buildAutomationActivityPath } from '@/lib/automationUi';
import type { CodingSession } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatCodingSessionRelative } from './codingSessionUtils';
import {
  codingSessionStatusLabel,
  formatCodingSessionElapsed,
} from './codingSessionPresentation';

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

function isActiveSessionStatus(status?: string | null) {
  return status === 'queued' || status === 'running' || status === 'paused';
}

function shouldForceHeaderDetailsOpen(session: CodingSession | null) {
  if (!session) return false;
  if (session.pause_reason && session.pause_reason !== 'none') return true;
  return session.status === 'paused' || session.status === 'failed' || session.status === 'cancelled';
}

function shouldAutoCollapseHeaderDetails(session: CodingSession | null) {
  if (!session) return false;
  if (session.pause_reason && session.pause_reason !== 'none') return false;
  return session.status === 'queued' || session.status === 'running';
}

function defaultHeaderDetailsOpen(session: CodingSession | null) {
  if (!session) return false;
  if (shouldForceHeaderDetailsOpen(session)) return true;
  return session.status !== 'completed';
}

function useElapsedSince(since?: string | null) {
  const origin = useMemo(() => since ? new Date(since).getTime() : 0, [since]);
  const subscribe = useCallback((cb: () => void) => {
    const id = window.setInterval(cb, 1_000);
    return () => window.clearInterval(id);
  }, []);
  const getSnapshot = useCallback(() => {
    if (!origin || Number.isNaN(origin)) return 0;
    return Date.now() - origin;
  }, [origin]);
  return useSyncExternalStore(subscribe, getSnapshot);
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
  const statusLabel = codingSessionStatusLabel({
    status: session?.status,
    pauseReason: session?.pause_reason,
    executionStage: session?.execution_stage,
  });
  const activeElapsedMs = useElapsedSince(isActiveSessionStatus(session?.status) ? session?.started_at ?? session?.created_at : null);
  const [detailsOpen, setDetailsOpen] = useState(() => defaultHeaderDetailsOpen(session));
  const [detailsManuallySet, setDetailsManuallySet] = useState(false);
  const [headerInteracting, setHeaderInteracting] = useState(false);
  const forceDetailsOpen = shouldForceHeaderDetailsOpen(session);
  const detailsVisible = detailsOpen || forceDetailsOpen || (headerInteracting && !detailsManuallySet);
  const repoHref = session?.repo.repo_name ? `https://github.com/${session.repo.repo_name}` : undefined;
  const branchHref = session?.repo.repo_name && session?.repo.branch
    ? `https://github.com/${session.repo.repo_name}/tree/${session.repo.branch}`
    : undefined;

  useEffect(() => {
    setDetailsManuallySet(false);
    setDetailsOpen(defaultHeaderDetailsOpen(session));
  }, [session?.id]);

  useEffect(() => {
    if (!session || detailsManuallySet) return;
    if (forceDetailsOpen) {
      setDetailsOpen(true);
      return;
    }
    if (session.status === 'completed') {
      setDetailsOpen(false);
      return;
    }
    if (!shouldAutoCollapseHeaderDetails(session)) {
      setDetailsOpen(true);
      return;
    }
    setDetailsOpen(true);
    const timer = window.setTimeout(() => setDetailsOpen(false), 5_000);
    return () => window.clearTimeout(timer);
  }, [session?.id, session?.status, session?.pause_reason, session?.execution_stage, detailsManuallySet, forceDetailsOpen]);

  const toggleDetails = () => {
    setDetailsManuallySet(true);
    setDetailsOpen((current) => !current);
  };

  return (
    <div
      className="space-y-3"
      onMouseEnter={() => setHeaderInteracting(true)}
      onMouseLeave={() => setHeaderInteracting(false)}
      onFocusCapture={() => setHeaderInteracting(true)}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setHeaderInteracting(false);
        }
      }}
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-3">
          <AgentAvatar name={agentName} className="h-10 w-10 shrink-0 rounded-none border-0 bg-transparent shadow-none" genericBare />

          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <h1 className="text-base font-semibold leading-tight">{agentName}</h1>
              <Badge
                variant="outline"
                className={cn(
                  'gap-1 px-2 py-0.5 text-[11px] font-medium capitalize',
                  session?.status === 'completed'
                    && 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
                  (session?.status === 'failed' || session?.status === 'cancelled')
                    && 'border-destructive/30 bg-destructive/10 text-destructive',
                )}
              >
                {statusIcon}
                {statusLabel}
              </Badge>
              {session?.pause_reason && session.pause_reason !== 'none' ? (
                <Badge variant="outline" className="px-2 py-0.5 text-[11px] capitalize">
                  {capitalize(session.pause_reason.replaceAll('_', ' '))}
                </Badge>
              ) : null}
              {session?.parent_run_id ? (
                <Badge variant="outline" className="px-2 py-0.5 text-[11px]">
                  Continued from {session.parent_run_id.slice(0, 8)}
                </Badge>
              ) : null}
            </div>

            <div className="mt-2 flex min-w-0 flex-wrap items-center gap-1.5 text-[11px]">
              {session?.repo.repo_name ? (
                <HeaderCodeChip
                  dataAttribute="data-coding-session-repo-chip"
                  href={repoHref}
                  icon={<Folder01Icon className="h-3 w-3 shrink-0 text-muted-foreground" />}
                  label={session.repo.repo_name}
                  maxClassName="max-w-[18rem]"
                />
              ) : null}

              {session ? (
                <span className="inline-flex h-6 items-center rounded-md border border-border/70 bg-background px-2 font-medium text-foreground">
                  {AGENT_RUNTIME_LABELS[session.runtime_kind] ?? capitalize(session.runtime_kind)}
                </span>
              ) : null}

              {session?.repo.branch ? (
                <HeaderCodeChip
                  dataAttribute="data-coding-session-branch-chip"
                  href={branchHref}
                  icon={<GitBranchIcon className="h-3 w-3 shrink-0 text-muted-foreground" />}
                  label={session.repo.branch}
                  maxClassName="max-w-[22rem]"
                  mono
                />
              ) : null}

              {session && isActiveSessionStatus(session.status) ? (
                <span className="inline-flex h-6 items-center rounded-md border border-border/70 bg-background px-2 font-medium tabular-nums text-foreground">
                  {formatCodingSessionElapsed(activeElapsedMs)}
                </span>
              ) : null}

              {session?.updated_at ? (
                <span className="inline-flex h-6 items-center rounded-md px-1.5 text-muted-foreground">
                  Updated {formatCodingSessionRelative(session.updated_at)}
                </span>
              ) : null}
            </div>
          </div>
        </div>

        {/* Actions — pr-10 reserves space for the sheet close button */}
        <div className="flex items-center gap-1.5 pr-10">
          {workspaceSlug ? (
            <Button asChild variant="outline" size="sm">
              <a href={buildAutomationActivityPath(workspaceSlug)}>
                Back to activity
              </a>
            </Button>
          ) : null}
          {session ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-8 gap-1.5 px-2.5"
              onClick={toggleDetails}
              aria-expanded={detailsVisible}
            >
              {detailsVisible ? <ArrowUp02Icon className="h-3.5 w-3.5" /> : <ArrowDown02Icon className="h-3.5 w-3.5" />}
              {detailsVisible ? 'Hide details' : 'Details'}
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

      {detailsVisible ? (
        <>
          <dl className="grid grid-cols-1 gap-x-4 gap-y-1.5 border-t border-border/60 pt-3 text-[11px] sm:grid-cols-[auto_minmax(0,1fr)_auto_auto]">
            {session?.repo.repo_name ? (
              <>
                <dt className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Repository</dt>
                <dd className="min-w-0 font-medium text-foreground">{session.repo.repo_name}</dd>
              </>
            ) : null}

            {session ? (
              <>
                <dt className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Runtime</dt>
                <dd className="font-medium text-foreground">
                  {AGENT_RUNTIME_LABELS[session.runtime_kind] ?? capitalize(session.runtime_kind)}
                </dd>
              </>
            ) : null}

            {session?.repo.branch ? (
              <>
                <dt className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Branch</dt>
                <dd className="min-w-0 font-mono text-[10.5px] font-medium text-foreground">{session.repo.branch}</dd>
              </>
            ) : null}

            {session?.updated_at ? (
              <>
                <dt className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Updated</dt>
                <dd className="text-muted-foreground">{formatCodingSessionRelative(session.updated_at)}</dd>
              </>
            ) : null}

            {session && isActiveSessionStatus(session.status) ? (
              <>
                <dt className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Elapsed</dt>
                <dd className="font-medium tabular-nums text-foreground">
                  {formatCodingSessionElapsed(activeElapsedMs)}
                </dd>
              </>
            ) : null}

            {session?.tokens_used ? (
              <>
                <dt className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">Tokens</dt>
                <dd className="font-medium text-foreground">
                  {formatSessionTokenUsage(session, { includeUnit: true })}
                </dd>
              </>
            ) : null}
          </dl>

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
        </>
      ) : null}

      {session && detailsVisible ? <CodingSessionLifecycleStrip session={session} /> : null}
    </div>
  );
}

function HeaderCodeChip({
  dataAttribute,
  href,
  icon,
  label,
  maxClassName,
  mono = false,
}: {
  dataAttribute: 'data-coding-session-repo-chip' | 'data-coding-session-branch-chip';
  href?: string;
  icon: ReactNode;
  label: string;
  maxClassName: string;
  mono?: boolean;
}) {
  const className = cn(
    'inline-flex h-6 min-w-0 items-center gap-1 rounded-md border border-border/70 bg-muted/50 px-2 font-medium text-foreground transition-colors hover:bg-muted hover:text-primary',
    maxClassName,
  );
  const content = (
    <>
      {icon}
      <span className={cn('truncate', mono && 'font-mono text-[10.5px]')}>{label}</span>
    </>
  );
  if (href) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noreferrer"
        title={label}
        className={className}
        {...{ [dataAttribute]: true }}
      >
        {content}
      </a>
    );
  }
  return (
    <span title={label} className={className} {...{ [dataAttribute]: true }}>
      {content}
    </span>
  );
}

function CodingSessionLifecycleStrip({ session }: { session: CodingSession }) {
  const items = [
    { key: 'queued', label: 'Queued' },
    { key: 'preparing', label: 'Workspace' },
    { key: 'starting', label: 'Runtime' },
    { key: 'working', label: 'Working' },
    { key: 'terminal', label: session.status === 'completed' ? 'Completed' : session.status === 'failed' ? 'Failed' : 'Done' },
  ];
  const activeKey = (() => {
    if (session.status === 'queued') return 'queued';
    if (session.status === 'paused') return 'working';
    if (session.status === 'completed' || session.status === 'failed' || session.status === 'cancelled') return 'terminal';
    if (session.execution_stage === 'preparing') return 'preparing';
    if (session.execution_stage === 'starting') return 'starting';
    return 'working';
  })();
  const activeIndex = items.findIndex((item) => item.key === activeKey);

  return (
    <div className="flex flex-wrap items-center gap-2 border-t border-border/60 pt-3">
      <span className="mr-1 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        Progress
      </span>
      {items.map((item, index) => {
        const complete = index < activeIndex;
        const active = index === activeIndex;
        return (
          <div key={item.key} className="flex items-center gap-2">
            {index > 0 ? <div className={cn('h-px w-5', complete || active ? 'bg-primary/50' : 'bg-border')} /> : null}
            <span className={cn(
              'inline-flex h-6 items-center rounded-full border px-2 text-[10px] font-medium',
              complete && 'border-primary/20 bg-primary/5 text-primary',
              active && 'border-primary/40 bg-primary/10 text-primary',
              !complete && !active && 'border-border bg-muted/30 text-muted-foreground',
            )}>
              {item.label}
            </span>
          </div>
        );
      })}
    </div>
  );
}
