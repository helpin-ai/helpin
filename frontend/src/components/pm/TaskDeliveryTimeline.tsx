import { useMemo, useState } from 'react';
import { formatDistanceToNow, parseISO } from 'date-fns';
import {
  ArrowUpRight01Icon,
  GitBranchIcon,
  GitCommitIcon,
  GitPullRequestIcon,
  Loading01Icon,
} from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { formatRunTokenUsageBreakdown, formatRunTokenUsageTotal } from '@/lib/agentTokenUsage';
import { gitBranchURL, gitCommitURL } from '@/lib/gitUrls';
import type { Agent, AgentRun, TaskDeliveryTarget, TaskGitLink } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';
import { getAgentRunDisplayStatus, STATUS_META } from './agentRunConstants';
import { buildTaskDeliveryTimelineEntries } from './taskDeliveryTimelineEntries';

const DELIVERY_PREVIEW_LIMIT = 5;

function runDotClass(run: AgentRun) {
  if (run.status === 'completed') return 'border-emerald-500 bg-emerald-500';
  if (run.status === 'failed' || run.status === 'cancelled') return 'border-destructive bg-destructive';
  if (run.status === 'queued' || run.status === 'running' || run.status === 'paused') {
    return 'border-[2.5px] border-foreground bg-background';
  }
  return 'border-border bg-muted';
}

function gitDotClass(link: TaskGitLink) {
  if (link.pr_status === 'closed') return 'border-border bg-muted';
  if (link.pr_status === 'merged' || link.commit_sha) return 'border-emerald-500 bg-emerald-500';
  return 'border-[2.5px] border-foreground bg-background';
}

function relativeTime(value: string) {
  return formatDistanceToNow(parseISO(value), { addSuffix: true });
}

function runDuration(run: AgentRun) {
  if (!run.started_at) return '';
  if (!run.completed_at) return run.status === 'running' ? 'In progress' : '';
  const milliseconds = Math.max(0, Date.parse(run.completed_at) - Date.parse(run.started_at));
  const seconds = Math.round(milliseconds / 1000);
  if (seconds < 60) return `${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m`;
  const hours = Math.floor(minutes / 60);
  return `${hours}h ${minutes % 60}m`;
}

function gitActivity(link: TaskGitLink, deliveryTarget?: TaskDeliveryTarget | null) {
  const workingBranch = link.branch ?? deliveryTarget?.working_branch ?? '';
  if (link.pr_url || link.pr_number) {
    return {
      icon: GitPullRequestIcon,
      title: `Pull request${link.pr_number ? ` #${link.pr_number}` : ''}${link.pr_status ? ` · ${link.pr_status}` : ''}`,
      detail: [link.pr_title, workingBranch].filter(Boolean).join(' · '),
      href: link.pr_url,
      action: 'View PR',
    };
  }
  if (link.commit_sha) {
    return {
      icon: GitCommitIcon,
      title: `Commit ${link.commit_sha.slice(0, 7)} linked`,
      detail: [link.repo, workingBranch].filter(Boolean).join(' · '),
      href: gitCommitURL(link.provider, link.repo, link.commit_sha, link.base_url),
      action: 'View commit',
    };
  }
  return {
    icon: GitBranchIcon,
    title: `Branch ${workingBranch || 'linked'}`,
    detail: link.repo,
    href: workingBranch ? gitBranchURL(link.provider, link.repo, workingBranch, link.base_url) : undefined,
    action: 'View branch',
  };
}

function RunTimelineItem({
  run,
  agent,
  isLast,
  onOpen,
}: {
  run: AgentRun;
  agent?: Agent;
  isLast: boolean;
  onOpen: () => void;
}) {
  const displayStatus = getAgentRunDisplayStatus(run);
  const status = STATUS_META[displayStatus] ?? STATUS_META.queued;
  const agentName = agent?.name ?? 'Agent';
  const tokenTotal = formatRunTokenUsageTotal(run, { includeUnit: true });
  const tokenBreakdown = formatRunTokenUsageBreakdown(run);
  const duration = runDuration(run);
  const detail = [
    run.execution_stage,
    tokenTotal !== '-' ? tokenTotal : null,
    duration,
  ].filter(Boolean).join(' · ');

  return (
    <div className="flex gap-3.5" data-testid="delivery-run-entry">
      <div className="flex w-3.5 shrink-0 flex-col items-center">
        <span className={cn('mt-1 h-2.5 w-2.5 shrink-0 rounded-full border', runDotClass(run))} />
        {!isLast ? <span className="mt-1 w-px flex-1 bg-border/70" /> : null}
      </div>
      <div className={cn('flex min-w-0 flex-1 items-start gap-3', !isLast && 'pb-5')}>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5 text-sm">
            <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
            <span className="font-medium text-foreground">{agentName}</span>
            <span className={cn('text-xs font-medium', displayStatus === 'failed' ? 'text-destructive' : 'text-foreground/75')}>
              {status.label.toLowerCase()}
            </span>
            <span className="text-xs text-muted-foreground">· {relativeTime(run.completed_at ?? run.updated_at ?? run.created_at)}</span>
          </div>
          <div className="mt-0.5 flex min-w-0 items-center gap-1.5 text-xs text-muted-foreground">
            <span className="truncate">{detail || 'No run details recorded'}</span>
            {tokenBreakdown.length > 1 ? (
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className="shrink-0 cursor-help text-muted-foreground/70">Details</span>
                </TooltipTrigger>
                <TooltipContent side="top" align="start" className="space-y-1 text-xs">
                  {tokenBreakdown.map((line) => <div key={line}>{line}</div>)}
                </TooltipContent>
              </Tooltip>
            ) : null}
          </div>
          {run.error_message ? <p className="mt-1 line-clamp-2 text-xs text-destructive/85">{run.error_message}</p> : null}
        </div>
        <Button type="button" variant="outline" size="sm" onClick={onOpen}>Open run</Button>
      </div>
    </div>
  );
}

function GitTimelineItem({
  link,
  deliveryTarget,
  isLast,
}: {
  link: TaskGitLink;
  deliveryTarget?: TaskDeliveryTarget | null;
  isLast: boolean;
}) {
  const activity = gitActivity(link, deliveryTarget);
  const ActivityIcon = activity.icon;

  return (
    <div className="flex gap-3.5" data-testid="delivery-git-entry">
      <div className="flex w-3.5 shrink-0 flex-col items-center">
        <span className={cn('mt-1 h-2.5 w-2.5 shrink-0 rounded-full border', gitDotClass(link))} />
        {!isLast ? <span className="mt-1 w-px flex-1 bg-border/70" /> : null}
      </div>
      <div className={cn('flex min-w-0 flex-1 items-start gap-3', !isLast && 'pb-5')}>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 flex-wrap items-center gap-1.5 text-sm">
            <ActivityIcon className="h-4 w-4 shrink-0 text-muted-foreground" />
            <span className="font-medium text-foreground">{activity.title}</span>
            <span className="text-xs text-muted-foreground">· {relativeTime(link.updated_at)}</span>
          </div>
          <p className="mt-0.5 truncate text-xs text-muted-foreground">{activity.detail || link.repo}</p>
        </div>
        {activity.href ? (
          <Button asChild variant="outline" size="sm">
            <a href={activity.href} target="_blank" rel="noopener noreferrer">
              {activity.action}
              <ArrowUpRight01Icon />
            </a>
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function TaskDeliveryTimeline({
  runs,
  agents,
  links,
  deliveryTarget,
  loading,
  error,
  onOpenRun,
}: {
  runs: AgentRun[];
  agents: Agent[];
  links: TaskGitLink[];
  deliveryTarget?: TaskDeliveryTarget | null;
  loading: boolean;
  error?: string | null;
  onOpenRun: (runId: string) => void;
}) {
  const [showAll, setShowAll] = useState(false);
  const agentsById = useMemo(() => new Map(agents.map((agent) => [agent.id, agent])), [agents]);
  const entries = useMemo(() => buildTaskDeliveryTimelineEntries(runs, links), [links, runs]);
  const visibleEntries = showAll ? entries : entries.slice(0, DELIVERY_PREVIEW_LIMIT);
  const hiddenCount = Math.max(entries.length - DELIVERY_PREVIEW_LIMIT, 0);

  return (
    <section aria-label="Delivery progress">
      <div className="mb-3 flex items-center justify-between gap-3">
        <h2 className="text-xs font-semibold uppercase tracking-wide text-foreground/70">Delivery progress</h2>
        {entries.length > 0 ? <span className="text-xs text-muted-foreground">{entries.length} events</span> : null}
      </div>

      {loading ? (
        <div className="flex items-center gap-2 border-y border-border/60 py-5 text-sm text-muted-foreground">
          <Loading01Icon className="h-4 w-4 animate-spin" />
          Loading delivery activity…
        </div>
      ) : error && visibleEntries.length === 0 ? (
        <p className="border-y border-border/60 py-5 text-sm text-destructive">{error}</p>
      ) : visibleEntries.length === 0 ? (
        <p className="border-y border-border/60 py-8 text-sm text-muted-foreground">No delivery activity yet.</p>
      ) : (
        <div>
          {visibleEntries.map((entry, index) => {
            const isLast = index === visibleEntries.length - 1;
            return entry.kind === 'run' ? (
              <RunTimelineItem
                key={entry.id}
                run={entry.run}
                agent={agentsById.get(entry.run.agent_id)}
                isLast={isLast}
                onOpen={() => onOpenRun(entry.run.id)}
              />
            ) : (
              <GitTimelineItem
                key={entry.id}
                link={entry.link}
                deliveryTarget={deliveryTarget}
                isLast={isLast}
              />
            );
          })}
        </div>
      )}

      {hiddenCount > 0 || showAll ? (
        <div className="mt-3 flex items-center justify-between border-t border-border/60 pt-3 text-sm text-muted-foreground">
          <span>{showAll ? `${entries.length} delivery events` : `Previous activity · ${hiddenCount}`}</span>
          <Button type="button" variant="ghost" size="sm" onClick={() => setShowAll((current) => !current)}>
            {showAll ? 'Show recent' : 'Show all'}
          </Button>
        </div>
      ) : null}
    </section>
  );
}
