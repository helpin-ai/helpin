import { useCallback, useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  ArrowReloadHorizontalIcon,
  ArrowUpRight01Icon,
  BotIcon,
  Calendar03Icon,
  Loading01Icon,
  Search01Icon,
  ZapIcon,
} from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { AutomationShell } from '@/components/automation/AutomationShell';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus, isPausedAgentRun } from '@/components/pm/agentRunConstants';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Skeleton } from '@/components/ui/skeleton';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAgents, useAutomationActivity, useAutomationOverview, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { buildAutomationFlowsPath } from '@/lib/automationUi';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { automationService } from '@/lib/services/automationService';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import type {
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionListItem,
} from '@/lib/types';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';

export type AutomationActivitySearch = {
  page: number;
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  status?: string;
  source?: string;
  reference_id?: string;
  fired_after?: string;
  fired_before?: string;
};

const EXECUTIONS_PER_PAGE = 25;

const SHORT_STATUS_LABELS: Record<string, string> = {
  completed: 'Completed',
  failed: 'Failed',
  running: 'Running',
  queued: 'Queued',
  paused: 'Paused',
  awaiting_approval: 'Needs approval',
  awaiting_input: 'Needs input',
  awaiting_auth: 'Needs sign-in',
  cancelled: 'Cancelled',
  skipped: 'Skipped',
};

const STATUS_STYLES: Record<string, string> = {
  completed: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  failed: 'border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-400',
  running: 'border-sky-500/30 bg-sky-500/10 text-sky-700 dark:text-sky-400',
  queued: 'border-border/70 bg-muted/40 text-muted-foreground',
  paused: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  awaiting_approval: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  awaiting_input: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  awaiting_auth: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  cancelled: 'border-border/70 bg-muted/40 text-muted-foreground',
  skipped: 'border-border/70 bg-muted/40 text-muted-foreground',
};

function relativeTime(isoString?: string): string {
  if (!isoString) return '';
  const diff = Date.now() - new Date(isoString).getTime();
  if (Number.isNaN(diff) || diff < 0) return '';
  const mins = Math.floor(diff / 60_000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days === 1) return 'Yesterday';
  return `${days}d ago`;
}

function formatShortDate(isoString?: string) {
  if (!isoString) return '\u2014';
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return '\u2014';
  return date.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

function formatDuration(startIso?: string, endIso?: string) {
  if (!startIso || !endIso) return '\u2014';
  const ms = new Date(endIso).getTime() - new Date(startIso).getTime();
  if (Number.isNaN(ms) || ms < 0) return '\u2014';
  const totalSecs = Math.floor(ms / 1000);
  if (totalSecs < 60) return `${totalSecs}s`;
  const mins = Math.floor(totalSecs / 60);
  const secs = totalSecs % 60;
  if (mins < 60) return `${mins}m ${secs.toString().padStart(2, '0')}s`;
  const hours = Math.floor(mins / 60);
  const remMins = mins % 60;
  return `${hours}h ${remMins.toString().padStart(2, '0')}m`;
}

function truncateMiddle(value?: string, start = 8, end = 6) {
  if (!value) return '';
  if (value.length <= start + end + 3) return value;
  return `${value.slice(0, start)}\u2026${value.slice(-end)}`;
}

function trimFilterValue(value?: string) {
  const trimmed = value?.trim();
  return trimmed || undefined;
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function asNonEmptyString(value: unknown): string | undefined {
  return typeof value === 'string' && value.trim().length > 0 ? value.trim() : undefined;
}

function startOfDay(isoString?: string) {
  if (!isoString) return '';
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return '';
  date.setHours(0, 0, 0, 0);
  return date.toISOString();
}

function formatDayGroupLabel(isoString: string) {
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return 'Unknown day';
  const today = new Date();
  today.setHours(0, 0, 0, 0);
  const diffDays = Math.round((today.getTime() - date.getTime()) / 86_400_000);
  const absolute = date.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  if (diffDays === 0) return `Today · ${absolute}`;
  if (diffDays === 1) return `Yesterday · ${absolute}`;
  return date.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' });
}

function getTimeFilterDate(value?: string) {
  if (!value || value === '__all__') return undefined;
  const normalized = value.toLowerCase();
  const date = new Date();

  if (normalized.endsWith('h')) {
    const hours = Number.parseInt(normalized, 10);
    if (!Number.isNaN(hours) && hours > 0) {
      date.setHours(date.getHours() - hours);
      return date.toISOString().split('T')[0];
    }
  }

  if (normalized.endsWith('d')) {
    const days = Number.parseInt(normalized, 10);
    if (!Number.isNaN(days) && days > 0) {
      date.setDate(date.getDate() - days);
      return date.toISOString().split('T')[0];
    }
  }

  return undefined;
}

function sourceLabel(value?: string) {
  switch (value) {
    case 'manual':
      return 'Manual';
    case 'automation_rule':
      return 'Rule';
    case 'schedule':
      return 'Scheduled';
    case 'support_widget':
      return 'Support';
    case 'task_assignment':
      return 'Assignment';
    default:
      return value ? value.replace(/_/g, ' ') : 'Manual';
  }
}

function buildExecutionTriggerLabel(item: AutomationTriggerExecutionListItem) {
  if (item.binding_kind === 'manual') return 'Manual';
  return item.trigger_title || item.binding_title || sourceLabel(item.binding_kind);
}

function buildExecutionTargetLabel(item: AutomationTriggerExecutionListItem) {
  if (item.reference_title?.trim()) return item.reference_title.trim();
  if (item.target_type && item.target_id) return `${item.target_type.replace(/_/g, ' ')} · ${truncateMiddle(item.target_id, 8, 4)}`;
  if (item.reference_type && item.reference_id) return `${item.reference_type.replace(/_/g, ' ')} · ${truncateMiddle(item.reference_id, 8, 4)}`;
  return 'Workspace event';
}

function runTargetLabel(run: AgentRun) {
  const input = asRecord(run.input);
  const target = asRecord(input?.target);
  const title =
    asNonEmptyString(target?.title) ||
    asNonEmptyString(input?.reference_title) ||
    asNonEmptyString(input?.title);
  if (title) return title;
  return `${run.target_type.replace(/_/g, ' ')} · ${truncateMiddle(run.target_id, 8, 4)}`;
}

function runBlockingLabel(run: AgentRun) {
  const status = getAgentRunDisplayStatus(run);
  switch (status) {
    case 'awaiting_approval':
      return 'Needs approval';
    case 'awaiting_auth':
      return 'Needs sign-in';
    default:
      return 'Needs your input';
  }
}

function runBlockingCopy(run: AgentRun, agent?: Agent) {
  const status = getAgentRunDisplayStatus(run);
  const agentName = agent?.name ?? 'Agent';
  switch (status) {
    case 'awaiting_approval':
      return `${agentName} is waiting for approval to continue`;
    case 'awaiting_auth':
      return `${agentName} is waiting for authentication to continue`;
    default:
      return `${agentName} is waiting for more context to continue`;
  }
}

function parseSmartFilter(input: string, agents: Agent[]) {
  const tokens = input.split(/\s+/).filter(Boolean);
  const updates: Partial<AutomationActivitySearch> = {
    agent_id: undefined,
    status: undefined,
    source: undefined,
    fired_after: undefined,
    fired_before: undefined,
    page: 1,
  };

  for (const token of tokens) {
    const [rawKey, ...rest] = token.split(':');
    const key = rawKey.toLowerCase();
    const value = rest.join(':').trim();
    if (!value) continue;

    if (key === 'agent') {
      const match = agents.find((agent) => agent.name.toLowerCase() === value.toLowerCase())
        ?? agents.find((agent) => agent.name.toLowerCase().includes(value.toLowerCase()));
      if (match) updates.agent_id = match.id;
      continue;
    }

    if (key === 'status') {
      updates.status = value.toLowerCase().replace(/\s+/g, '_');
      continue;
    }

    if (key === 'trigger' || key === 'source') {
      const normalized = value.toLowerCase().replace(/\s+/g, '_');
      if (['manual', 'automation_rule', 'schedule', 'support_widget', 'task_assignment'].includes(normalized)) {
        updates.source = normalized;
      }
      continue;
    }

    if (key === 'last') {
      updates.fired_after = getTimeFilterDate(value);
    }
  }

  return updates;
}

function buildSmartFilterValue(search: AutomationActivitySearch, agents: Agent[]) {
  const tokens: string[] = [];
  if (search.agent_id) {
    const agentName = agents.find((agent) => agent.id === search.agent_id)?.name;
    if (agentName) tokens.push(`agent:${agentName}`);
  }
  if (search.status) tokens.push(`status:${search.status}`);
  if (search.source) tokens.push(`source:${search.source}`);
  if (search.fired_after) {
    const days = Math.round((Date.now() - new Date(search.fired_after).getTime()) / 86_400_000);
    if (days <= 1) {
      tokens.push('last:24h');
    } else {
      tokens.push(`last:${days}d`);
    }
  }
  return tokens.join(' ');
}

function SparkBars({ values, tone = 'neutral' }: { values: number[]; tone?: 'neutral' | 'good' | 'warn' | 'bad' }) {
  const hasData = values.length > 0 && values.some((value) => value > 0);
  if (!hasData) {
    return (
      <div className="flex h-8 items-center">
        <span className="h-[2px] w-14 rounded-full bg-muted-foreground/20" aria-hidden />
        <span className="sr-only">No activity yet</span>
      </div>
    );
  }

  const max = Math.max(...values, 1);
  const color = tone === 'good'
    ? 'bg-emerald-500'
    : tone === 'warn'
      ? 'bg-amber-500'
      : tone === 'bad'
        ? 'bg-rose-500'
        : 'bg-foreground/45';

  return (
    <div className="flex h-8 items-end gap-1">
      {values.map((value, index) => (
        <span
          key={`${tone}-${index}`}
          className={cn('w-1.5 rounded-sm opacity-80', color)}
          style={{ height: `${Math.max((value / max) * 100, 18)}%` }}
        />
      ))}
    </div>
  );
}

function SummaryCard({
  label,
  value,
  sublabel,
  tone = 'neutral',
  spark,
}: {
  label: string;
  value: string;
  sublabel: string;
  tone?: 'neutral' | 'good' | 'warn' | 'bad';
  spark?: number[];
}) {
  const valueClass = tone === 'good'
    ? 'text-emerald-600 dark:text-emerald-400'
    : tone === 'warn'
      ? 'text-amber-600 dark:text-amber-400'
      : tone === 'bad'
        ? 'text-rose-600 dark:text-rose-400'
        : 'text-foreground';

  return (
    <Card className="border-border/70 bg-card/80">
      <CardContent className="flex items-end justify-between gap-4 p-4">
        <div className="space-y-1">
          <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">{label}</p>
          <p className={cn('text-2xl font-semibold tracking-tight', valueClass)}>{value}</p>
          <p className="text-xs text-muted-foreground">{sublabel}</p>
        </div>
        {spark && <SparkBars values={spark} tone={tone} />}
      </CardContent>
    </Card>
  );
}

function StatusBadge({ status }: { status: string }) {
  return (
    <Badge variant="outline" className={cn('gap-1.5 rounded-full px-2.5 py-0.5 text-[11px] font-medium', STATUS_STYLES[status] ?? STATUS_STYLES.queued)}>
      <span className="h-1.5 w-1.5 rounded-full bg-current" />
      {SHORT_STATUS_LABELS[status] ?? status}
    </Badge>
  );
}

function TriggerKindChip({ kind }: { kind: string }) {
  const label = sourceLabel(kind);
  const icon = kind === 'automation_rule'
    ? <ZapIcon className="h-3 w-3" />
    : kind === 'schedule'
      ? <Calendar03Icon className="h-3 w-3" />
      : <BotIcon className="h-3 w-3" />;

  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border border-border/70 bg-muted/40 px-2 py-0.5 text-[11px] font-medium text-muted-foreground">
      {icon}
      {label}
    </span>
  );
}

function SmartFilterInput({
  value,
  onChange,
  onApply,
  onClear,
  disabled,
}: {
  value: string;
  onChange: (value: string) => void;
  onApply: () => void;
  onClear: () => void;
  disabled?: boolean;
}) {
  return (
    <div className="flex flex-col gap-2 md:flex-row md:items-center">
      <div className="relative flex-1">
        <Search01Icon className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
        <Input
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              onApply();
            }
          }}
          placeholder="agent:Lens status:failed last:24h"
          className="h-8 pl-8 font-mono text-xs"
          disabled={disabled}
        />
        <Tooltip>
          <TooltipTrigger asChild>
            <button
              type="button"
              tabIndex={-1}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-[10px] font-mono text-muted-foreground/60 hover:text-muted-foreground"
              aria-label="Filter syntax help"
            >
              ?
            </button>
          </TooltipTrigger>
          <TooltipContent side="top" align="end" className="max-w-xs font-mono text-[11px]">
            Supports <code>agent:</code>, <code>status:</code>, <code>source:</code>, <code>last:24h|7d|30d</code>.
          </TooltipContent>
        </Tooltip>
      </div>
      <div className="flex items-center gap-2">
        <Button type="button" size="sm" variant="ghost" onClick={onClear} disabled={disabled} className="h-8 px-2 text-xs text-muted-foreground">
          Clear
        </Button>
        <Button type="button" size="sm" onClick={onApply} disabled={disabled} className="h-8 px-3 text-xs">
          Apply
        </Button>
      </div>
    </div>
  );
}

function NeedActionCard({
  run,
  agent,
  onOpenRun,
  onApprove,
  approving,
}: {
  run: AgentRun;
  agent?: Agent;
  onOpenRun: (runId: string) => void;
  onApprove: (runId: string) => Promise<void>;
  approving: boolean;
}) {
  const displayStatus = getAgentRunDisplayStatus(run);
  const waitTime = formatDuration(run.created_at, new Date().toISOString());
  const needsApproval = displayStatus === 'awaiting_approval';
  const subtitle = runBlockingCopy(run, agent);

  return (
    <div className="grid gap-4 rounded-2xl border border-amber-500/30 bg-card/90 p-4 shadow-sm shadow-amber-500/5 md:grid-cols-[1fr_auto] md:items-center">
      <div className="min-w-0 space-y-2 border-l-4 border-amber-500 pl-4">
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="outline" className="rounded-full border-amber-500/40 bg-amber-500/10 text-[11px] font-medium text-amber-700 dark:text-amber-400">
            {runBlockingLabel(run)}
          </Badge>
          <span className="font-mono text-[11px] text-muted-foreground">blocked for {waitTime}</span>
        </div>
        <p className="text-sm font-medium">{subtitle}</p>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {agent ? <AgentAvatar agent={agent} className="h-6 w-6 rounded-none border-0 bg-transparent shadow-none" genericBare /> : null}
          <span>{agent?.name ?? 'Agent'}</span>
          <span className="text-muted-foreground/60">·</span>
          <span className="text-foreground">{runTargetLabel(run)}</span>
          <span className="text-muted-foreground/60">·</span>
          <span className="font-mono">{formatShortDate(run.created_at)}</span>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Button type="button" variant="outline" size="sm" onClick={() => onOpenRun(run.id)}>
          View run
        </Button>
        {needsApproval ? (
          <Button type="button" size="sm" onClick={() => void onApprove(run.id)} disabled={approving}>
            {approving ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : null}
            Approve
          </Button>
        ) : (
          <Button type="button" size="sm" onClick={() => onOpenRun(run.id)}>
            Respond
          </Button>
        )}
      </div>
    </div>
  );
}

function TimelineRow({
  item,
  agent,
  workspaceSlug,
  onOpenRun,
}: {
  item: AutomationTriggerExecutionListItem;
  agent?: Agent;
  workspaceSlug?: string;
  onOpenRun: (runId: string) => void;
}) {
  const duration = formatDuration(item.started_at, item.completed_at);
  const targetLabel = buildExecutionTargetLabel(item);
  const triggerLabel = buildExecutionTriggerLabel(item);
  const flowHref = item.manage_path || (item.reference_id ? buildAutomationFlowsPath(workspaceSlug, { target_id: item.reference_id }) : undefined);
  const canOpenRun = Boolean(item.run_id);

  return (
    <div className="grid grid-cols-[1.25rem_minmax(0,1fr)_auto] gap-3 px-4 py-3">
      <div className="relative flex justify-center">
        <span className={cn(
          'mt-1 h-2.5 w-2.5 rounded-full ring-4 ring-background',
          item.status === 'completed'
            ? 'bg-emerald-500'
            : item.status === 'failed'
              ? 'bg-rose-500'
              : 'bg-amber-500',
        )} />
      </div>

      <div className="min-w-0 space-y-2">
        <div className="flex flex-wrap items-center gap-2">
          <StatusBadge status={item.status} />
          <TriggerKindChip kind={item.binding_kind} />
          <span className="text-sm font-medium text-foreground">{targetLabel}</span>
        </div>

        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
          <span>{triggerLabel}</span>
          {item.binding_title && (
            <>
              <span className="text-muted-foreground/60">via</span>
              {flowHref ? (
                <a
                  href={flowHref}
                  className="inline-flex items-center gap-1 text-foreground underline decoration-border underline-offset-4 hover:text-primary"
                >
                  {item.binding_title}
                  <ArrowUpRight01Icon className="h-3 w-3" />
                </a>
              ) : (
                <span className="text-foreground">{item.binding_title}</span>
              )}
            </>
          )}
        </div>

        <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
          <div className="flex min-w-0 items-center gap-2">
            {agent?.is_system ? (
              <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
            ) : (
              <span className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-border/70 bg-muted/40">
                <BotIcon className="h-3 w-3" />
              </span>
            )}
            <span>{item.agent_name}</span>
          </div>
          <span className="font-mono">{relativeTime(item.fired_at)}</span>
          <span className="font-mono">{duration}</span>
          {item.error_message ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <span className="max-w-[28rem] truncate text-rose-600 dark:text-rose-400">{item.error_message}</span>
              </TooltipTrigger>
              <TooltipContent side="bottom" align="start" className="max-w-md whitespace-pre-wrap break-words text-xs">
                {item.error_message}
              </TooltipContent>
            </Tooltip>
          ) : null}
        </div>
      </div>

      <div className="flex items-start justify-end">
        {canOpenRun ? (
          <Button type="button" variant="ghost" size="sm" className="h-8 px-2.5 text-xs" onClick={() => onOpenRun(item.run_id!)}>
            Open
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function AutomationActivityPage({
  search,
  onSearchChange,
}: {
  search: AutomationActivitySearch;
  onSearchChange: (updates: Partial<AutomationActivitySearch>) => void;
}) {
  useTitle('Automation Activity');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);

  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [approvingRunId, setApprovingRunId] = useState<string | null>(null);
  const [smartFilter, setSmartFilter] = useState('');

  const openRun = useCallback((runId: string) => {
    setSelectedRunId(runId);
    setDrawerOpen(true);
  }, []);

  const { data: agents = [] } = useAgents(workspaceId);
  const overviewQuery = useAutomationOverview(workspaceId, true);

  useEffect(() => {
    setSmartFilter(buildSmartFilterValue(search, Array.isArray(agents) ? agents : []));
  }, [agents, search]);

  const executionFilters = useMemo<AutomationTriggerExecutionFilters>(
    () => ({
      page: search.page,
      per_page: EXECUTIONS_PER_PAGE,
      agent_id: trimFilterValue(search.agent_id),
      binding_id: trimFilterValue(search.binding_id),
      trigger_type: trimFilterValue(search.trigger_type),
      status: trimFilterValue(search.status),
      source: trimFilterValue(search.source),
      reference_id: trimFilterValue(search.reference_id),
      fired_after: trimFilterValue(search.fired_after),
      fired_before: trimFilterValue(search.fired_before),
    }),
    [search],
  );

  const executionsQuery = useAutomationActivity(workspaceId, executionFilters, permissions.canManageSettings);
  const runsQuery = useQuery({
    queryKey: queryKeys.automation.runs(workspaceId, 1, 100),
    queryFn: async () => {
      const payload = unwrap(await automationService.listWorkspaceRuns(workspaceId, 1, 100));
      return {
        data: Array.isArray(payload?.data) ? payload.data : [],
        total: payload?.total ?? 0,
        page: payload?.page ?? 1,
        per_page: payload?.per_page ?? 100,
        total_pages: payload?.total_pages ?? 0,
      };
    },
    enabled: !!workspaceId && permissions.canManageSettings,
    staleTime: 15_000,
    refetchInterval: 30_000,
  });

  useEffect(() => {
    const handler = () => {
      void runsQuery.refetch();
      void executionsQuery.refetch();
      void overviewQuery.refetch();
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [executionsQuery, overviewQuery, runsQuery]);

  const agentList = Array.isArray(agents) ? agents : [];
  const agentById = useMemo(() => new Map(agentList.map((agent) => [agent.id, agent])), [agentList]);
  const rawWorkspaceRuns = runsQuery.data?.data;
  const workspaceRuns = Array.isArray(rawWorkspaceRuns) ? rawWorkspaceRuns : [];
  const pausedRuns = useMemo(() => workspaceRuns.filter((run) => ACTIVE_RUN_STATUSES.has(run.status) && isPausedAgentRun(run)), [workspaceRuns]);
  const rawExecutions = executionsQuery.data?.data;
  const executions = Array.isArray(rawExecutions) ? rawExecutions : [];
  const executionPage = executionsQuery.data?.page ?? search.page;
  const executionTotalPages = executionsQuery.data?.total_pages ?? 0;
  const rawItems = overviewQuery.data?.items;
  const items = Array.isArray(rawItems) ? rawItems : [];

  const groupedExecutions = useMemo(() => {
    const groups = new Map<string, AutomationTriggerExecutionListItem[]>();
    for (const item of executions) {
      const key = startOfDay(item.fired_at) || 'unknown';
      const current = groups.get(key) ?? [];
      current.push(item);
      groups.set(key, current);
    }

    return Array.from(groups.entries()).map(([key, rows]) => ({
      key,
      label: key === 'unknown' ? 'Unknown day' : formatDayGroupLabel(key),
      rows,
    }));
  }, [executions]);

  const recent24hRuns = useMemo(() => {
    const threshold = Date.now() - 86_400_000;
    return workspaceRuns.filter((run) => {
      const createdAt = new Date(run.created_at).getTime();
      return !Number.isNaN(createdAt) && createdAt >= threshold;
    });
  }, [workspaceRuns]);

  const healthSummary = useMemo(() => {
    // Only count items the backend explicitly marked with each status.
    // Items that are `inactive` (disabled) or `unknown` (never ran) should not
    // be counted as healthy — that was the old bug that made an empty
    // workspace look like "11 healthy".
    const errorCount = items.filter((item) => item.health.status === 'error').length;
    const warningCount = items.filter((item) => item.health.status === 'warning').length;
    const healthyCount = items.filter((item) => item.health.status === 'healthy').length;
    const idleCount = items.length - errorCount - warningCount - healthyCount;
    return { errorCount, warningCount, healthyCount, idleCount, totalCount: items.length };
  }, [items]);

  const recentStatusBars = useMemo(
    () => workspaceRuns.slice(0, 7).reverse().map((run) => (
      run.status === 'completed' ? 4 : run.status === 'failed' ? 1 : 2
    )),
    [workspaceRuns],
  );

  const recentFailureBars = useMemo(() => {
    const perDay = new Map<string, number>();
    for (const run of recent24hRuns.filter((run) => run.status === 'failed')) {
      const key = startOfDay(run.created_at);
      perDay.set(key, (perDay.get(key) ?? 0) + 1);
    }
    return Array.from(perDay.values()).slice(-7);
  }, [recent24hRuns]);

  const needsYouBars = useMemo(
    () => pausedRuns.slice(0, 7).reverse().map((run) => (
      getAgentRunDisplayStatus(run) === 'awaiting_approval' ? 4 : 2
    )),
    [pausedRuns],
  );

  const oldestBlocked = useMemo(() => {
    if (pausedRuns.length === 0) return null;
    const oldest = [...pausedRuns].sort((a, b) => new Date(a.created_at).getTime() - new Date(b.created_at).getTime())[0];
    return formatDuration(oldest.created_at, new Date().toISOString());
  }, [pausedRuns]);

  const handleApplySmartFilter = useCallback(() => {
    onSearchChange(parseSmartFilter(smartFilter, Array.isArray(agents) ? agents : []));
  }, [agents, onSearchChange, smartFilter]);

  const handleClearSmartFilter = useCallback(() => {
    setSmartFilter('');
    onSearchChange({
      page: 1,
      agent_id: undefined,
      binding_id: undefined,
      trigger_type: undefined,
      status: undefined,
      source: undefined,
      reference_id: undefined,
      fired_after: undefined,
      fired_before: undefined,
    });
  }, [onSearchChange]);

  const handleApproveRun = useCallback(async (runId: string) => {
    if (!workspaceId) return;
    setApprovingRunId(runId);
    const res = await automationService.approveRun(workspaceId, runId);
    if (res.error) {
      toast.error('Failed to approve run', { description: res.error });
    } else {
      toast.success('Run approved');
      await Promise.all([
        runsQuery.refetch(),
        executionsQuery.refetch(),
      ]);
    }
    setApprovingRunId(null);
  }, [executionsQuery, runsQuery, workspaceId]);

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  if (!permissions.canManageSettings) {
    return (
      <AutomationShell
        title="Automation Activity"
        description="See what fired, what needs a human, and which flows are delivering value."
      >
        <Card className={LINEAR_CARD_CLASS}>
          <CardContent className="px-5 py-6 text-sm text-muted-foreground">
            You do not have permission to view workspace-wide automation activity.
          </CardContent>
        </Card>
      </AutomationShell>
    );
  }

  return (
    <AutomationShell
      title="Activity"
      description="Answer the operator question first: what needs a human, what is healthy, and where the failures are clustering."
      actions={(
        <Button variant="outline" size="sm" onClick={() => void Promise.all([runsQuery.refetch(), executionsQuery.refetch()])}>
          <ArrowReloadHorizontalIcon className="mr-1.5 h-3.5 w-3.5" />
          Refresh
        </Button>
      )}
    >
      <div className="space-y-5">
        <div className="grid gap-3 lg:grid-cols-4">
          <SummaryCard
            label="Needs You"
            value={String(pausedRuns.length)}
            sublabel={pausedRuns.length > 0 ? `Oldest blocked ${oldestBlocked ?? '\u2014'}` : 'No paused runs waiting on a human'}
            tone={pausedRuns.length > 0 ? 'warn' : 'neutral'}
            spark={needsYouBars}
          />
          <SummaryCard
            label="Runs · 24h"
            value={String(recent24hRuns.length)}
            sublabel={recent24hRuns.length > 0 ? `${recent24hRuns.filter((run) => run.status === 'completed').length} completed in the latest day` : 'No recent runs'}
            tone="neutral"
            spark={recentStatusBars}
          />
          <SummaryCard
            label="Failed · 24h"
            value={String(recent24hRuns.filter((run) => run.status === 'failed').length)}
            sublabel={recent24hRuns.filter((run) => run.status === 'failed').length > 0 ? 'Investigate repeated failures and flaky flows' : 'No recent failures'}
            tone={recent24hRuns.some((run) => run.status === 'failed') ? 'bad' : 'neutral'}
            spark={recentFailureBars}
          />
          <SummaryCard
            label="Fleet Health"
            value={String(
              healthSummary.errorCount > 0
                ? healthSummary.errorCount
                : healthSummary.warningCount > 0
                  ? healthSummary.warningCount
                  : healthSummary.healthyCount > 0
                    ? healthSummary.healthyCount
                    : healthSummary.idleCount,
            )}
            sublabel={
              healthSummary.errorCount > 0
                ? `${healthSummary.errorCount === 1 ? 'error' : 'errors'} reported`
                : healthSummary.warningCount > 0
                  ? `${healthSummary.warningCount === 1 ? 'warning' : 'warnings'} to review`
                  : healthSummary.healthyCount > 0
                    ? `${healthSummary.healthyCount === 1 ? 'automation' : 'automations'} healthy`
                    : healthSummary.idleCount > 0
                      ? `${healthSummary.idleCount === 1 ? 'automation' : 'automations'} idle — no runs yet`
                      : 'No automations configured'
            }
            tone={
              healthSummary.errorCount > 0
                ? 'bad'
                : healthSummary.warningCount > 0
                  ? 'warn'
                  : healthSummary.healthyCount > 0
                    ? 'good'
                    : 'neutral'
            }
          />
        </div>

        {pausedRuns.length > 0 && (
          <section className="space-y-3">
            <div className="space-y-1">
              <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Needs You</p>
              <p className="text-sm text-muted-foreground">Paused runs waiting on a human. Clear these to unblock the fleet.</p>
            </div>
            <div className="space-y-3">
              {pausedRuns.map((run) => (
                <NeedActionCard
                  key={run.id}
                  run={run}
                  agent={agentById.get(run.agent_id)}
                  onOpenRun={openRun}
                  onApprove={handleApproveRun}
                  approving={approvingRunId === run.id}
                />
              ))}
            </div>
          </section>
        )}

        <section className="space-y-3">
          <div className="space-y-3">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
              <div className="space-y-1">
                <p className="text-[11px] font-medium uppercase tracking-[0.14em] text-muted-foreground">Timeline</p>
                <p className="text-sm text-muted-foreground">
                  Grouped by day so clusters of failures and pauses are visible without reading raw trigger prose.
                </p>
              </div>
              <div className="lg:max-w-xl lg:flex-1">
                <SmartFilterInput
                  value={smartFilter}
                  onChange={setSmartFilter}
                  onApply={handleApplySmartFilter}
                  onClear={handleClearSmartFilter}
                  disabled={executionsQuery.isLoading}
                />
              </div>
            </div>

            {executionsQuery.isLoading ? (
              <div className="space-y-3">
                <Skeleton className="h-28 w-full rounded-2xl" />
                <Skeleton className="h-28 w-full rounded-2xl" />
              </div>
            ) : executionsQuery.isError ? (
              <div className="rounded-2xl border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
                Could not load trigger executions
                {executionsQuery.error instanceof Error ? `: ${executionsQuery.error.message}` : ''}
              </div>
            ) : groupedExecutions.length > 0 ? (
              <div className="space-y-4">
                {groupedExecutions.map((group) => (
                  <div key={group.key} className="space-y-2">
                    <div className="px-1">
                      <p className="font-mono text-[11px] uppercase tracking-[0.16em] text-muted-foreground">{group.label}</p>
                    </div>
                    <div className="rounded-2xl border border-border/70 bg-card/80">
                      <div className="relative pl-3">
                        <div className="absolute bottom-3 left-[1.08rem] top-3 border-l border-dashed border-border/80" />
                        <div className="relative divide-y divide-border/60">
                          {group.rows.map((item) => (
                            <TimelineRow
                              key={item.execution_id}
                              item={item}
                              agent={agentById.get(item.agent_id)}
                              workspaceSlug={workspace?.slug}
                              onOpenRun={openRun}
                            />
                          ))}
                        </div>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
                No activity matches the current filter.
              </div>
            )}

            {executionTotalPages > 1 && (
              <div className="flex items-center justify-between">
                <p className="text-xs text-muted-foreground">
                  Page {executionPage} of {executionTotalPages}
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={executionPage <= 1}
                    onClick={() => onSearchChange({ page: Math.max(1, executionPage - 1) })}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={executionPage >= executionTotalPages}
                    onClick={() => onSearchChange({ page: executionPage + 1 })}
                  >
                    Next
                  </Button>
                </div>
              </div>
            )}
          </div>
        </section>
      </div>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        title="Agent Run"
        description="Interactive transcript, approvals, artifacts, and session details."
      />
    </AutomationShell>
  );
}
