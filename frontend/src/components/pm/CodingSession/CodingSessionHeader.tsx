import { useEffect, useMemo, useState, type ReactNode } from 'react';
import { ArrowReloadHorizontalIcon, CancelCircleIcon, Folder01Icon, GitBranchIcon, InformationCircleIcon, Loading01Icon } from '@/lib/icons';

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

function normalizedLifecycleStage(session: CodingSession) {
  const stage = session.execution_stage?.trim() ?? '';
  if (session.status === 'queued') return 'queued';
  if (session.status === 'paused') return 'waiting';
  if (session.status === 'completed' || session.status === 'failed' || session.status === 'cancelled') return 'terminal';
  if (stage === 'preparing' || stage === 'continuing') return 'preparing';
  if (stage === 'starting' || stage.endsWith('_starting')) return 'starting';
  if (
    stage === 'resuming'
    || stage === 'approved'
    || stage === 'feedback_received'
    || stage === 'input_received'
    || stage === 'auth_completed'
  ) {
    return 'resuming';
  }
  return 'working';
}

function currentLifecycleStagePresentation(session: CodingSession) {
  const activeKey = normalizedLifecycleStage(session);
  if (activeKey === 'queued') return { label: 'Queued', detail: 'Waiting to start', tone: 'working' };
  if (activeKey === 'preparing') return { label: 'Preparing', detail: 'Loading run context', tone: 'working' };
  if (activeKey === 'starting') return { label: 'Starting agent', detail: 'Connecting runtime', tone: 'working' };
  if (activeKey === 'resuming') return { label: 'Resuming', detail: 'Continuing after your response', tone: 'working' };
  if (activeKey === 'working') return { label: 'Agent working', detail: 'Runtime is active', tone: 'working' };
  if (activeKey === 'waiting') {
    if (session.execution_stage === 'awaiting_review') return { label: 'Review', detail: 'Waiting for your review', tone: 'waiting' };
    if (session.pause_reason === 'human_approval') return { label: 'Approval', detail: 'Waiting for your decision', tone: 'waiting' };
    if (session.pause_reason === 'human_input') return { label: 'Input', detail: 'Waiting for your reply', tone: 'waiting' };
    if (session.pause_reason === 'authentication') return { label: 'Sign-in', detail: 'Waiting for sign-in', tone: 'waiting' };
    return { label: 'Waiting', detail: 'Waiting for you', tone: 'waiting' };
  }
  if (session.status === 'completed') return { label: 'Completed', detail: 'Run finished', tone: 'success' };
  if (session.status === 'failed') return { label: 'Failed', detail: 'Stopped with an error', tone: 'failed' };
  if (session.status === 'cancelled') return { label: 'Cancelled', detail: 'Run stopped', tone: 'cancelled' };
  return { label: 'Done', detail: 'Run ended', tone: 'terminal' };
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
  const elapsedLabel = session?.status === 'paused'
    ? `Ran ${formatCodingSessionElapsed(elapsedMs)}`
    : formatCodingSessionElapsed(elapsedMs);
  const repoHref = session?.repo.repo_name ? `https://github.com/${session.repo.repo_name}` : undefined;
  const branchHref = session?.repo.repo_name && session?.repo.branch
    ? `https://github.com/${session.repo.repo_name}/tree/${session.repo.branch}`
    : undefined;

  return (
    <div className="space-y-2.5">
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

      {session ? (
        <div className="space-y-2 border-t border-border/60 pt-2.5">
          <div className="flex flex-wrap items-center justify-between gap-3" data-coding-session-detail-row>
            {session ? <CodingSessionLifecycleStrip session={session} /> : null}
            <div className="flex min-w-0 items-center gap-1.5">
              {session?.tokens_used ? (
                <CodingSessionTokenSummary session={session} />
              ) : null}
              {session?.tokens_used && session ? (
                <span className="h-3 w-px bg-muted-foreground/25" aria-hidden="true" />
              ) : null}
              {session ? (
                <span
                  className="inline-flex h-5 items-center rounded-md px-1.5 text-[11px] font-medium text-muted-foreground"
                  data-coding-session-runtime-pill
                >
                  {AGENT_RUNTIME_LABELS[session.runtime_kind] ?? capitalize(session.runtime_kind)}
                </span>
              ) : null}
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
  const stage = currentLifecycleStagePresentation(session);
  const activeWorkingStep = session.status === 'queued' || session.status === 'running';

  return (
    <div className="flex min-w-0 items-center">
      <span
        className={cn(
          'inline-flex min-h-5 max-w-full items-center rounded-md px-1.5 py-0 text-[11px] text-muted-foreground',
          stage.tone === 'working' && 'text-muted-foreground',
          stage.tone === 'waiting' && 'text-amber-700/80 dark:text-amber-400/80',
          stage.tone === 'success' && 'text-emerald-700/80 dark:text-emerald-400/80',
          stage.tone === 'failed' && 'text-destructive/85',
          stage.tone === 'cancelled' && 'text-muted-foreground',
          stage.tone === 'terminal' && 'text-muted-foreground',
        )}
        data-coding-session-lifecycle-stage
      >
        {activeWorkingStep ? (
          <span
            aria-hidden="true"
            className="mr-1.5 h-1.5 w-1.5 rounded-full bg-primary/70 animate-pulse"
          />
        ) : null}
        <span className="font-medium text-foreground/80">{stage.label}</span>
        <span className="mx-1.5 h-3 w-px bg-current opacity-20" />
        <span className="truncate opacity-70">{stage.detail}</span>
      </span>
    </div>
  );
}
