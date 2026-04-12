import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useQuery } from '@tanstack/react-query';
import {
  BotIcon,
  Clock01Icon,
  Loading01Icon,
  PauseIcon,
} from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { AutomationShell } from '@/components/automation/AutomationShell';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { ACTIVE_RUN_STATUSES } from '@/components/pm/agentRunConstants';
import { Badge } from '@/components/ui/badge';
import { Card, CardContent } from '@/components/ui/card';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import { useAgents, useAutomationActivity, useAutomationOverview } from '@/hooks/queries';
import { useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { automationService } from '@/lib/services/automationService';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import type {
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionListItem,
  AutomationInventoryItem,
} from '@/lib/types';
import { useWorkspaceStore } from '@/stores/workspaceStore';

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

const EXECUTIONS_PER_PAGE = 25;

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

function formatShortDate(isoString?: string): string {
  if (!isoString) return '\u2014';
  const d = new Date(isoString);
  if (Number.isNaN(d.getTime())) return '\u2014';
  return d.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit' });
}

function formatDuration(startIso?: string, endIso?: string): string {
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

function truncateMiddle(value?: string, start = 8, end = 6): string {
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

function formatBindingKind(kind: string): string {
  switch (kind) {
    case 'manual': return 'Manual';
    case 'automation_rule': return 'Automation rule';
    case 'schedule': return 'Scheduled';
    case 'support_widget': return 'Support';
    case 'task_assignment': return 'Assignment';
    default: return kind.replace(/_/g, ' ').replace(/\b\w/g, (c) => c.toUpperCase());
  }
}

function buildTriggerLabel(item: AutomationTriggerExecutionListItem): string {
  const kind = formatBindingKind(item.binding_kind);
  const name = item.trigger_title || item.binding_title || item.trigger_type || '';
  if (!name) return kind;
  if (kind === 'Manual') return `Manual ${name}`;
  return `${kind} \u00b7 ${name}`;
}

function formatRunTriggerLabel(input: Record<string, unknown> | undefined): string {
  const trigger = asRecord(input?.trigger);
  if (!trigger) return 'Manual';

  const source = asNonEmptyString(trigger.source);
  const triggerType = asNonEmptyString(trigger.trigger_type);

  switch (source) {
    case 'manual':
      return 'Manual';
    case 'automation_rule':
      return triggerType ? `Automation rule · ${triggerType}` : 'Automation rule';
    case 'schedule':
      return triggerType ? `Scheduled · ${triggerType}` : 'Scheduled';
    case 'support_widget':
      return triggerType ? `Support widget · ${triggerType}` : 'Support widget';
    case 'task_assignment':
      return triggerType ? `Task assignment · ${triggerType}` : 'Task assignment';
    default:
      return triggerType ?? source ?? 'Manual';
  }
}

// ---------------------------------------------------------------------------
// Status bar
// ---------------------------------------------------------------------------

function StatusBar({ items }: { items: AutomationInventoryItem[] }) {
  const errorCount = items.filter((i) => i.health.status === 'error').length;
  const warningCount = items.filter((i) => i.health.status === 'warning').length;

  if (errorCount === 0 && warningCount === 0) {
    return (
      <div className="flex items-center gap-2 text-sm text-muted-foreground">
        <span className="h-2 w-2 rounded-full bg-emerald-500" />
        All {items.length} automations operational
      </div>
    );
  }

  return (
    <div className="flex flex-wrap items-center gap-3 text-sm">
      {errorCount > 0 && (
        <span className="flex items-center gap-1.5 text-rose-700 dark:text-rose-400">
          <span className="h-2 w-2 rounded-full bg-rose-500" />
          {errorCount} error{errorCount !== 1 ? 's' : ''}
        </span>
      )}
      {warningCount > 0 && (
        <span className="flex items-center gap-1.5 text-amber-700 dark:text-amber-400">
          <span className="h-2 w-2 rounded-full bg-amber-500" />
          {warningCount} warning{warningCount !== 1 ? 's' : ''}
        </span>
      )}
      <span className="text-muted-foreground">
        &middot; {items.length - errorCount - warningCount} healthy
      </span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Active run card
// ---------------------------------------------------------------------------

const PAUSE_LABELS: Record<string, string> = {
  human_approval: 'Paused \u00b7 awaiting approval',
  human_input: 'Paused \u00b7 awaiting input',
  authentication: 'Paused \u00b7 awaiting sign-in',
};

const ACTIVE_RUN_GRID = 'grid grid-cols-[minmax(160px,auto)_minmax(100px,1fr)_100px_120px_90px_110px] items-center gap-x-4';

function ActiveRunCard({
  run,
  agent,
  onOpenRun,
}: {
  run: AgentRun;
  agent?: Agent;
  onOpenRun: (runId: string) => void;
}) {
  const pauseLabel =
    run.status === 'paused'
      ? PAUSE_LABELS[run.pause_reason] ?? 'Paused'
      : run.status === 'running'
        ? 'Running'
        : run.status === 'queued'
          ? 'Queued'
          : run.status;

  const isRunning = run.status === 'running';
  const isPaused = run.status === 'paused';

  const elapsed = run.created_at
    ? formatDuration(run.created_at, new Date().toISOString())
    : '\u2014';

  return (
    <button
      type="button"
      className="w-full rounded-lg border border-border/60 bg-card px-4 py-3 text-left transition-colors hover:bg-muted/50"
      onClick={() => onOpenRun(run.id)}
    >
      <div className={ACTIVE_RUN_GRID}>
        <Badge
          variant="outline"
          className="w-fit gap-1.5 border-amber-500/40 bg-amber-500/10 text-amber-700 dark:text-amber-400"
        >
          {isPaused ? (
            <PauseIcon className="h-3 w-3" />
          ) : isRunning ? (
            <Loading01Icon className="h-3 w-3 animate-spin" />
          ) : (
            <Clock01Icon className="h-3 w-3" />
          )}
          {pauseLabel}
        </Badge>

        <p className="truncate text-sm font-medium">{agent?.name ?? 'Agent'}</p>
        <p className="text-sm text-muted-foreground">{formatRunTriggerLabel(run.input)}</p>
        <p className="text-sm font-mono text-muted-foreground">{run.target_id ? truncateMiddle(run.target_id, 8, 4) : '\u2014'}</p>
        <p className="text-sm tabular-nums">{elapsed}</p>
        <p className="text-xs text-muted-foreground">{formatShortDate(run.created_at)}</p>
      </div>
    </button>
  );
}

function ActiveRunLabels() {
  return (
    <div className={`${ACTIVE_RUN_GRID} px-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground`}>
      <span>Status</span>
      <span>Agent</span>
      <span>Trigger</span>
      <span>Target</span>
      <span>Duration</span>
      <span>Started</span>
    </div>
  );
}

function HeaderFilterSelect({
  value,
  onValueChange,
  placeholder,
  children,
  className,
}: {
  value: string;
  onValueChange: (value: string) => void;
  placeholder: string;
  children: ReactNode;
  className?: string;
}) {
  return (
    <Select value={value} onValueChange={onValueChange}>
      <SelectTrigger className={`h-7 text-[11px] font-normal normal-case tracking-normal ${className ?? ''}`}>
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent>
        {children}
      </SelectContent>
    </Select>
  );
}

const STATUS_VARIANT: Record<string, string> = {
  completed: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-700 dark:text-emerald-400',
  failed: 'border-rose-500/30 bg-rose-500/10 text-rose-700 dark:text-rose-400',
  running: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  paused: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  queued: '',
  cancelled: '',
  skipped: '',
};

function ExecutionRow({
  item,
  agent,
  onOpenRun,
}: {
  item: AutomationTriggerExecutionListItem;
  agent?: Agent;
  onOpenRun: (runId: string) => void;
}) {
  const duration = formatDuration(item.started_at, item.completed_at);
  const triggerLabel = buildTriggerLabel(item);
  const rel = relativeTime(item.fired_at);
  const canOpenRun = Boolean(item.run_id);

  return (
    <div className={`rounded-lg border bg-card transition-colors ${item.status === 'failed' ? 'border-rose-500/20' : 'border-border'}`}>
      <div className="flex w-full items-center gap-3 px-4 py-2.5 text-left transition-colors hover:bg-muted">
        <div className={EXECUTION_COLUMNS.fired}>
          <p className="text-sm">{formatShortDate(item.fired_at)}</p>
          {rel && <p className="text-[11px] text-muted-foreground">{rel}</p>}
        </div>

        <div className={EXECUTION_COLUMNS.status}>
          {item.error_message ? (
            <Tooltip>
              <TooltipTrigger asChild>
                <Badge variant="outline" className={`cursor-help capitalize ${STATUS_VARIANT[item.status] ?? ''}`}>
                  {item.status}
                </Badge>
              </TooltipTrigger>
              <TooltipContent side="top" align="start" className="max-w-[360px] whitespace-pre-wrap break-words px-3 py-2 text-xs leading-relaxed">
                {item.error_message}
              </TooltipContent>
            </Tooltip>
          ) : (
            <Badge variant="outline" className={`capitalize ${STATUS_VARIANT[item.status] ?? ''}`}>
              {item.status}
            </Badge>
          )}
        </div>

        <div className={EXECUTION_COLUMNS.trigger}>
          <p className="truncate text-sm font-medium">{triggerLabel}</p>
          <p className="truncate text-[11px] text-muted-foreground">
            {formatBindingKind(item.binding_kind)}
          </p>
        </div>

        <div className={EXECUTION_COLUMNS.agent}>
          {canOpenRun ? (
            <button
              type="button"
              className="flex w-full items-center gap-2 rounded-md px-1.5 py-1 text-left transition-colors hover:bg-accent"
              onClick={(event) => {
                event.stopPropagation();
                onOpenRun(item.run_id!);
              }}
            >
              {agent?.is_system ? (
                <AgentAvatar agent={agent} className="h-6 w-6 rounded-xl border-border/60" />
              ) : (
                <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-xl border border-border/60 bg-muted/40 text-muted-foreground">
                  <BotIcon className="h-3.5 w-3.5" />
                </span>
              )}
              <span className="min-w-0">
                <span className="block truncate text-sm">{item.agent_name}</span>
                <span className="block truncate font-mono text-[11px] text-muted-foreground" title={item.agent_id}>
                  {truncateMiddle(item.agent_id, 8, 6)}
                </span>
              </span>
            </button>
          ) : (
            <div className="flex items-center gap-2 px-1.5 py-1">
              {agent?.is_system ? (
                <AgentAvatar agent={agent} className="h-6 w-6 rounded-xl border-border/60" />
              ) : (
                <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-xl border border-border/60 bg-muted/40 text-muted-foreground">
                  <BotIcon className="h-3.5 w-3.5" />
                </span>
              )}
              <span className="min-w-0">
                <span className="block truncate text-sm">{item.agent_name}</span>
                <span className="block truncate font-mono text-[11px] text-muted-foreground" title={item.agent_id}>
                  {truncateMiddle(item.agent_id, 8, 6)}
                </span>
              </span>
            </div>
          )}
        </div>

        <p className={`${EXECUTION_COLUMNS.duration} text-sm text-muted-foreground`}>{duration}</p>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Filters
// ---------------------------------------------------------------------------

const SOURCE_OPTIONS = [
  { value: '__all__', label: 'All triggers' },
  { value: 'manual', label: 'Manual' },
  { value: 'automation_rule', label: 'Automation rule' },
  { value: 'schedule', label: 'Schedule' },
  { value: 'support_widget', label: 'Support widget' },
  { value: 'task_assignment', label: 'Task assignment' },
];

const STATUS_OPTIONS = [
  { value: '__all__', label: 'All statuses' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'running', label: 'Running' },
  { value: 'paused', label: 'Paused' },
  { value: 'queued', label: 'Queued' },
  { value: 'cancelled', label: 'Cancelled' },
  { value: 'skipped', label: 'Skipped' },
];

const TIME_OPTIONS = [
  { value: '__all__', label: 'All time' },
  { value: '1d', label: 'Last 24 hours' },
  { value: '7d', label: 'Last 7 days' },
  { value: '30d', label: 'Last 30 days' },
];

const EXECUTION_COLUMNS = {
  fired: 'min-w-[140px] shrink-0',
  status: 'w-[120px] shrink-0',
  trigger: 'min-w-[220px] flex-1',
  agent: 'w-[190px] shrink-0',
  duration: 'w-[88px] shrink-0',
} as const;

function getTimeFilterDate(value: string): string | undefined {
  if (value === '__all__') return undefined;
  const days = Number.parseInt(value, 10);
  if (Number.isNaN(days)) return undefined;
  const d = new Date();
  d.setDate(d.getDate() - days);
  return d.toISOString().split('T')[0];
}

// ---------------------------------------------------------------------------
// Main page
// ---------------------------------------------------------------------------

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

  // Drawer state
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const openRun = useCallback((runId: string) => {
    setSelectedRunId(runId);
    setDrawerOpen(true);
  }, []);

  // Time filter state (maps to fired_after)
  const [timeFilter, setTimeFilter] = useState('__all__');

  const handleTimeFilter = useCallback(
    (value: string) => {
      setTimeFilter(value);
      onSearchChange({ fired_after: getTimeFilterDate(value), fired_before: undefined, page: 1 });
    },
    [onSearchChange],
  );

  // Data queries
  const { data: agents = [] } = useAgents(workspaceId);
  const overviewQuery = useAutomationOverview(workspaceId, true);

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

  // Active runs
  const runsQuery = useQuery({
    queryKey: queryKeys.automation.runs(workspaceId, 1, 50),
    queryFn: async () => unwrap(await automationService.listWorkspaceRuns(workspaceId, 1, 50)),
    enabled: !!workspaceId && permissions.canManageSettings,
    staleTime: 15_000,
    refetchInterval: 30_000,
  });

  useEffect(() => {
    const handler = () => {
      void runsQuery.refetch();
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [runsQuery.refetch]);

  const agentById = useMemo(
    () => new Map(agents.map((a) => [a.id, a])),
    [agents],
  );

  const activeRuns = useMemo(
    () => (runsQuery.data?.data ?? []).filter((r) => ACTIVE_RUN_STATUSES.has(r.status)),
    [runsQuery.data],
  );

  const items = overviewQuery.data?.items ?? [];
  const executions = executionsQuery.data?.data ?? [];
  const executionPage = executionsQuery.data?.page ?? search.page;
  const executionTotalPages = executionsQuery.data?.total_pages ?? 0;

  const updateFilters = useCallback(
    (updates: Partial<AutomationActivitySearch>) => {
      onSearchChange({ ...updates, page: updates.page ?? 1 });
    },
    [onSearchChange],
  );

  if (!workspaceId) {
    return <p className="text-sm text-muted-foreground">Workspace not found.</p>;
  }

  if (!permissions.canManageSettings) {
    return (
      <AutomationShell
        title="Automation Activity"
        description="See what triggered, what matched, what launched, and what each agent run did."
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
      title="Automation Activity"
      description="See what triggered, what matched, what launched, and what each agent run did."
    >
      <div className="space-y-5">
        {/* Status bar */}
        {items.length > 0 && (
          <div className="px-1">
            <StatusBar items={items} />
          </div>
        )}

        {/* Active runs */}
        {activeRuns.length > 0 && (
          <section>
            <p className="mb-2 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              Active Runs
            </p>
            <div className="space-y-2">
              <ActiveRunLabels />
              {activeRuns.map((run) => (
                <ActiveRunCard
                  key={run.id}
                  run={run}
                  agent={agentById.get(run.agent_id)}
                  onOpenRun={openRun}
                />
              ))}
            </div>
          </section>
        )}

        {/* History */}
        <section id="trigger-executions" className="space-y-3">
          <div className="space-y-3">
            <div className="flex items-start gap-3 px-4 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
              <div className={`${EXECUTION_COLUMNS.fired} space-y-1`}>
                <span className="block">Fired</span>
                <HeaderFilterSelect
                  value={timeFilter}
                  onValueChange={handleTimeFilter}
                  placeholder="All time"
                  className="w-[132px]"
                >
                  {TIME_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                  ))}
                </HeaderFilterSelect>
              </div>

              <div className={`${EXECUTION_COLUMNS.status} space-y-1`}>
                <span className="block">Status</span>
                <HeaderFilterSelect
                  value={search.status || '__all__'}
                  onValueChange={(v) => updateFilters({ status: v === '__all__' ? undefined : v })}
                  placeholder="All statuses"
                  className="w-full"
                >
                  {STATUS_OPTIONS.map((o) => (
                    <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                  ))}
                </HeaderFilterSelect>
              </div>

              <div className={`${EXECUTION_COLUMNS.trigger} space-y-1`}>
                <span className="block">Trigger</span>
                <div className="flex flex-wrap gap-2">
                  <HeaderFilterSelect
                    value={search.source || '__all__'}
                    onValueChange={(v) => updateFilters({ source: v === '__all__' ? undefined : v })}
                    placeholder="All triggers"
                    className="w-[150px]"
                  >
                    {SOURCE_OPTIONS.map((o) => (
                      <SelectItem key={o.value} value={o.value}>{o.label}</SelectItem>
                    ))}
                  </HeaderFilterSelect>
                </div>
              </div>

              <div className={`${EXECUTION_COLUMNS.agent} space-y-1`}>
                <span className="block">Agent</span>
                <HeaderFilterSelect
                  value={search.agent_id || '__all__'}
                  onValueChange={(v) => updateFilters({ agent_id: v === '__all__' ? undefined : v })}
                  placeholder="All agents"
                  className="w-full"
                >
                  <SelectItem value="__all__">All agents</SelectItem>
                  {agents.map((a) => (
                    <SelectItem key={a.id} value={a.id}>{a.name}</SelectItem>
                  ))}
                </HeaderFilterSelect>
              </div>

              <div className={EXECUTION_COLUMNS.duration}>
                <span className="block">Duration</span>
              </div>
            </div>

            {executionsQuery.isLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-14 w-full rounded-lg" />
                <Skeleton className="h-14 w-full rounded-lg" />
                <Skeleton className="h-14 w-full rounded-lg" />
              </div>
            ) : executionsQuery.isError ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/5 px-4 py-3 text-sm text-destructive">
                Could not load trigger executions
                {executionsQuery.error instanceof Error ? `: ${executionsQuery.error.message}` : ''}
              </div>
            ) : executions.length > 0 ? (
              <div className="space-y-2">
                {executions.map((item) => (
                  <ExecutionRow
                    key={item.execution_id}
                    item={item}
                    agent={agentById.get(item.agent_id)}
                    onOpenRun={openRun}
                  />
                ))}
              </div>
            ) : (
              <div className="rounded-lg border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
                No trigger executions match the current filters.
              </div>
            )}

            {/* Pagination */}
            {executionTotalPages > 1 && (
              <div className="flex items-center justify-between pt-1">
                <p className="text-xs text-muted-foreground">
                  Page {executionPage} of {executionTotalPages}
                </p>
                <div className="flex items-center gap-2">
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
                    disabled={executionPage <= 1}
                    onClick={() => onSearchChange({ page: Math.max(1, executionPage - 1) })}
                  >
                    Previous
                  </Button>
                  <Button
                    variant="outline"
                    size="sm"
                    className="h-7 text-xs"
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

      {/* Agent run drawer */}
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
