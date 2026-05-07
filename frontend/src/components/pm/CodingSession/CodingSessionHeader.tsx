import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ArrowReloadHorizontalIcon, ArrowRight02Icon, CancelCircleIcon, Folder01Icon, GitBranchIcon, InformationCircleIcon, Loading01Icon } from '@/lib/icons';

import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { AGENT_RUNTIME_LABELS } from '@/lib/agentRuntime';
import { formatCompactTokenCount, formatSessionTokenUsage } from '@/lib/agentTokenUsage';
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

function shouldShowElapsedForStatus(status?: string | null) {
  return status === 'queued' || status === 'running' || status === 'paused';
}

function shouldTickElapsedForStatus(status?: string | null) {
  return status === 'queued' || status === 'running';
}

function timestampMs(value?: string | null) {
  if (!value) return 0;
  const ms = new Date(value).getTime();
  return Number.isNaN(ms) ? 0 : ms;
}

function elapsedOriginForSession(session: CodingSession | null) {
  if (!session) return null;
  if (session.status === 'queued') return session.created_at;
  return session.started_at ?? session.created_at;
}

function pausedElapsedMsForSession(session: CodingSession | null) {
  if (!session || session.status !== 'paused') return 0;
  const origin = timestampMs(elapsedOriginForSession(session));
  const end = timestampMs(session.updated_at) || timestampMs(session.last_heartbeat_at) || origin;
  if (!origin || !end) return 0;
  return Math.max(0, end - origin);
}

function lifecycleBadgeClassName(session: CodingSession | null) {
  const status = session?.status;
  if (status === 'completed') {
    return 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400';
  }
  if (status === 'failed' || status === 'cancelled') {
    return 'border-destructive/30 bg-destructive/10 text-destructive';
  }
  if (status === 'paused') {
    return 'border-amber-500/35 bg-amber-500/10 text-amber-700 dark:text-amber-400';
  }
  if (status === 'running') {
    return 'border-primary/30 bg-primary/10 text-primary';
  }
  if (status === 'queued') {
    return 'border-border bg-muted/40 text-muted-foreground';
  }
  return '';
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
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    setNow(Date.now());
    if (!origin || Number.isNaN(origin)) return undefined;
    const id = window.setInterval(() => setNow(Date.now()), 1_000);
    return () => window.clearInterval(id);
  }, [origin]);

  if (!origin || Number.isNaN(origin)) return 0;
  return Math.max(0, now - origin);
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
  const cancelDisabled = acting !== null || !canCancel;
  const refreshDisabled = refreshing;
  const agentName = resolveSessionAgentName(session);
  const sessionTitle = session?.title?.trim() ?? '';
  const sessionSummary = session?.summary?.trim() ?? '';
  const showDetailBlock = (sessionTitle && sessionTitle !== agentName) || !!sessionSummary;
  const statusLabel = codingSessionStatusLabel({
    status: session?.status,
    pauseReason: session?.pause_reason,
    executionStage: session?.execution_stage,
  });
  const elapsedOrigin = elapsedOriginForSession(session);
  const tickingElapsedMs = useElapsedSince(shouldTickElapsedForStatus(session?.status) ? elapsedOrigin : null);
  const elapsedMs = session?.status === 'paused' ? pausedElapsedMsForSession(session) : tickingElapsedMs;
  const showElapsed = shouldShowElapsedForStatus(session?.status);
  const [detailsOpen, setDetailsOpen] = useState(() => defaultHeaderDetailsOpen(session));
  const [headerInteracting, setHeaderInteracting] = useState(false);
  const forceDetailsOpen = shouldForceHeaderDetailsOpen(session);
  const detailsVisible = detailsOpen || forceDetailsOpen || headerInteracting;
  const elapsedLabel = session?.status === 'paused'
    ? `Ran ${formatCodingSessionElapsed(elapsedMs)}`
    : formatCodingSessionElapsed(elapsedMs);
  const repoHref = session?.repo.repo_name ? `https://github.com/${session.repo.repo_name}` : undefined;
  const branchHref = session?.repo.repo_name && session?.repo.branch
    ? `https://github.com/${session.repo.repo_name}/tree/${session.repo.branch}`
    : undefined;

  useEffect(() => {
    setDetailsOpen(defaultHeaderDetailsOpen(session));
  }, [session?.id]);

  useEffect(() => {
    if (!session) return;
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
  }, [session?.id, session?.status, session?.pause_reason, session?.execution_stage, forceDetailsOpen]);

  return (
    <div
      className="space-y-2.5"
      onMouseEnter={() => setHeaderInteracting(true)}
      onMouseLeave={() => setHeaderInteracting(false)}
      onFocusCapture={() => setHeaderInteracting(true)}
      onBlurCapture={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
          setHeaderInteracting(false);
        }
      }}
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-1">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2" data-coding-session-title-row>
              <AgentAvatar name={agentName} className="h-6 w-6 rounded-none border-0 bg-transparent shadow-none" genericBare />
              <h1 className="text-base font-semibold leading-tight">{agentName}</h1>
              <Badge
                variant="outline"
                className={cn(
                  'gap-1 px-2 py-0.5 text-[11px] font-medium capitalize',
                  lifecycleBadgeClassName(session),
                )}
              >
                {statusIcon}
                {statusLabel}
              </Badge>
              {session?.parent_run_id ? (
                <Badge variant="outline" className="px-2 py-0.5 text-[11px]">
                  Continued from {session.parent_run_id.slice(0, 8)}
                </Badge>
              ) : null}
            </div>

            <div className="mt-2 flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1 text-[11px]">
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
                <span className="inline-flex h-6 items-center rounded-md border border-border/60 bg-muted/30 px-2 font-medium text-foreground">
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

              {session && showElapsed ? (
                <span className="inline-flex h-6 items-center rounded-md border border-border/60 bg-muted/30 px-2 font-medium tabular-nums text-foreground">
                  {elapsedLabel}
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
            <Tooltip>
              <TooltipTrigger asChild>
                <Button asChild variant="outline" size="sm">
                  <a href={buildAutomationActivityPath(workspaceSlug)}>
                    Back to activity
                  </a>
                </Button>
              </TooltipTrigger>
              <TooltipContent className="z-[1000]">Return to automation activity</TooltipContent>
            </Tooltip>
          ) : null}
          {onCancelRun ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="outline"
                  size="icon"
                  className={cn(
                    'h-8 w-8 hover:border-destructive/30 hover:bg-destructive/10 hover:text-destructive',
                    cancelDisabled && 'opacity-50',
                  )}
                  onClick={() => {
                    if (!cancelDisabled) onCancelRun();
                  }}
                  aria-disabled={cancelDisabled}
                  aria-label="Cancel run"
                >
                  {acting === 'cancel' ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <CancelCircleIcon className="h-3.5 w-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent className="z-[1000]">Cancel run</TooltipContent>
            </Tooltip>
          ) : null}
          <Tooltip>
            <TooltipTrigger asChild>
              <Button
                variant="outline"
                size="icon"
                className={cn('h-8 w-8', refreshDisabled && 'opacity-50')}
                onClick={() => {
                  if (!refreshDisabled) onRefresh();
                }}
                aria-disabled={refreshDisabled}
                aria-label="Refresh"
              >
                <ArrowReloadHorizontalIcon className={cn('h-3.5 w-3.5', refreshing && 'animate-spin')} />
              </Button>
            </TooltipTrigger>
            <TooltipContent className="z-[1000]">Refresh</TooltipContent>
          </Tooltip>
        </div>
      </div>

      {detailsVisible ? (
        <div className="space-y-2 border-t border-border/60 pt-2.5">
          <div className="flex flex-wrap items-center justify-between gap-3" data-coding-session-detail-row>
            {session ? <CodingSessionLifecycleStrip session={session} /> : null}
            {session?.tokens_used ? (
              <CodingSessionTokenSummary session={session} />
            ) : null}
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
      ) : null}
    </div>
  );
}

function CodingSessionTokenSummary({ session }: { session: CodingSession }) {
  const totalTokens = session.tokens_used || (session.input_tokens ?? 0) + (session.output_tokens ?? 0);
  const compactTotal = formatCompactTokenCount(totalTokens);
  const breakdown = formatSessionTokenUsage(session, { includeUnit: true });

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className="inline-flex h-5 items-center gap-1.5 rounded-md px-1.5 text-[11px] font-medium text-muted-foreground outline-none transition-colors hover:bg-muted hover:text-foreground focus-visible:bg-muted focus-visible:text-foreground"
          aria-label={`Token usage: ${breakdown}`}
        >
          <span>Tokens</span>
          <span className="font-semibold tabular-nums text-foreground">{compactTotal}</span>
          <InformationCircleIcon className="h-3 w-3 text-muted-foreground" aria-hidden="true" />
        </button>
      </TooltipTrigger>
      <TooltipContent side="bottom" align="end" className="z-[1000]">
        {breakdown}
      </TooltipContent>
    </Tooltip>
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
    'inline-flex h-6 min-w-0 items-center gap-1 rounded-md border border-border/60 bg-muted/30 px-2 font-medium text-foreground transition-colors hover:bg-muted hover:text-primary',
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
  const waitingLabel = (() => {
    if (session.pause_reason === 'human_approval') return 'Approval';
    if (session.pause_reason === 'human_input') return 'Input';
    if (session.pause_reason === 'authentication') return 'Sign-in';
    return 'Waiting';
  })();
  const items = [
    { key: 'queued', label: 'Queued' },
    { key: 'preparing', label: 'Preparing' },
    { key: 'starting', label: 'Starting agent' },
    { key: 'working', label: 'Working' },
    ...(session.status === 'paused' ? [{ key: 'waiting', label: waitingLabel }] : []),
    { key: 'terminal', label: session.status === 'completed' ? 'Completed' : session.status === 'failed' ? 'Failed' : 'Done' },
  ];
  const activeKey = (() => {
    if (session.status === 'queued') return 'queued';
    if (session.status === 'paused') return 'waiting';
    if (session.status === 'completed' || session.status === 'failed' || session.status === 'cancelled') return 'terminal';
    if (session.execution_stage === 'preparing') return 'preparing';
    if (session.execution_stage === 'starting') return 'starting';
    return 'working';
  })();
  const activeIndex = items.findIndex((item) => item.key === activeKey);

  return (
    <div className="flex min-w-0 flex-wrap items-center gap-2">
      {items.map((item, index) => {
        const complete = index < activeIndex;
        const active = index === activeIndex;
        return (
          <div key={item.key} className="flex items-center gap-2">
            {index > 0 ? (
              <ArrowRight02Icon
                aria-hidden="true"
                className={cn('h-3 w-3', complete || active ? 'text-muted-foreground' : 'text-muted-foreground/40')}
              />
            ) : null}
            <span className={cn(
              'inline-flex h-5 items-center rounded-md px-1.5 text-[11px] font-medium',
              complete && 'text-muted-foreground',
              active && 'bg-muted text-foreground',
              !complete && !active && 'text-muted-foreground/70',
            )}>
              {item.label}
            </span>
          </div>
        );
      })}
    </div>
  );
}
