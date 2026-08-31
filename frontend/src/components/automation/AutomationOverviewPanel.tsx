import { useMemo, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { QuietEmptyState, QuietUnderlineInput } from '@/components/design-system/quiet';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Skeleton } from '@/components/ui/skeleton';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from '@/components/ui/tooltip';
import { useAgents, useAutomationActivity, useAutomationOverview, useAutomationTriggerCatalog } from '@/hooks/queries';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { ArrowRight01Icon, DashboardSpeed01Icon, SecurityCheckIcon, BotIcon, Search01Icon } from '@/lib/icons';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import { buildAutomationActivityPath, buildAutomationFlowsPath } from '@/lib/automationUi';
import type {
  AutomationHealthStatus,
  AutomationInventoryItem,
  AutomationTriggerCatalogEntry,
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionSearchPreset,
  AutomationTriggerExecutionListItem,
  WorkflowRuleSearchPreset,
} from '@/lib/types';

const EXECUTIONS_PER_PAGE = 25;

interface AutomationOverviewPanelProps {
  workspaceId: string;
  search: {
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
  onSearchChange: (updates: {
    page?: number;
    agent_id?: string;
    binding_id?: string;
    trigger_type?: string;
    status?: string;
    source?: string;
    reference_id?: string;
    fired_after?: string;
    fired_before?: string;
  }) => void;
  sectionVisibility?: {
    statusBar?: boolean;
    triggerCatalog?: boolean;
    triggerExecutions?: boolean;
    builtIns?: boolean;
    automationRules?: boolean;
  };
  pathOverrides?: {
    activityBasePath?: string;
    flowsBasePath?: string;
    agentRunsBasePath?: string;
  };
}

const HEALTH_DOT: Record<AutomationHealthStatus, string> = {
  healthy: 'bg-emerald-500',
  warning: 'bg-amber-500',
  error: 'bg-rose-500',
  inactive: 'bg-slate-300',
  unknown: 'bg-slate-300',
};

const EXECUTION_STATUS_BADGE: Record<string, string> = {
  queued: 'border-slate-300 text-slate-700 dark:text-slate-200',
  running: 'border-amber-400 text-amber-700 dark:text-amber-300',
  paused: 'border-amber-400 text-amber-700 dark:text-amber-300',
  completed: 'border-emerald-400 text-emerald-700 dark:text-emerald-300',
  failed: 'border-rose-400 text-rose-700 dark:text-rose-300',
  cancelled: 'border-slate-300 text-slate-600 dark:text-slate-300',
  skipped: 'border-slate-300 text-slate-600 dark:text-slate-300',
};

const SOURCE_OPTIONS = [
  { value: '__all__', label: 'All sources' },
  { value: 'manual', label: 'Manual' },
  { value: 'automation_rule', label: 'Automation rule' },
  { value: 'schedule', label: 'Schedule' },
  { value: 'support_widget', label: 'Support widget' },
  { value: 'task_assignment', label: 'Task assignment' },
];

const STATUS_OPTIONS = [
  { value: '__all__', label: 'All statuses' },
  { value: 'queued', label: 'Queued' },
  { value: 'running', label: 'Running' },
  { value: 'paused', label: 'Paused' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'cancelled', label: 'Cancelled' },
  { value: 'skipped', label: 'Skipped' },
];

function relativeTime(isoString?: string): string {
  if (!isoString) return '--';
  const diff = Date.now() - new Date(isoString).getTime();
  if (Number.isNaN(diff) || diff < 0) return '--';
  const mins = Math.floor(diff / 60_000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function formatTimestamp(isoString?: string): string {
  if (!isoString) return '--';
  const date = new Date(isoString);
  if (Number.isNaN(date.getTime())) return '--';
  return date.toLocaleString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  });
}

function displayPath(path: string | undefined, slug: string | undefined) {
  if (!path) return undefined;
  if (!slug) return path.replace('/w/$slug', '/w/<workspace>');
  return path.replace('$slug', slug);
}

function trimFilterValue(value?: string) {
  const trimmed = value?.trim();
  return trimmed ? trimmed : undefined;
}

function truncateMiddle(value?: string, start = 8, end = 6): string {
  if (!value) return '--';
  if (value.length <= start + end + 3) return value;
  return `${value.slice(0, start)}...${value.slice(-end)}`;
}

function getContextInfo(item: AutomationInventoryItem): string | null {
  if (item.module === 'crm' && item.kind === 'built_in_automation') {
    const fresh = item.health.freshness !== 'unknown' ? item.health.freshness : null;
    const lastSuccess = relativeTime(item.health.last_success_at);
    if (fresh && lastSuccess !== '--') return `${fresh} · ${lastSuccess}`;
    if (fresh) return fresh;
    if (lastSuccess !== '--') return lastSuccess;
    return null;
  }
  if (item.kind === 'automation_rule') {
    const status = item.health.status !== 'unknown' ? item.health.status : null;
    const lastSeen = relativeTime(item.health.last_seen_at);
    if (status && lastSeen !== '--') return `${status} · ${lastSeen}`;
    if (status) return status;
    if (lastSeen !== '--') return lastSeen;
    return null;
  }
  return null;
}

interface SubgroupDef {
  label: string;
  filter: (item: AutomationInventoryItem) => boolean;
}

interface TriggerGroupDef {
  id: 'flow' | 'manual' | 'built_in';
  label: string;
  helper: string;
  filter: (item: AutomationTriggerCatalogEntry) => boolean;
}

const SUBGROUPS: SubgroupDef[] = [
  { label: 'CRM system intelligence', filter: (i) => i.module === 'crm' && i.kind === 'built_in_automation' },
  { label: 'PM built-in rules', filter: (i) => i.module === 'pm' && i.kind === 'built_in_automation' },
  { label: 'Automation rules', filter: (i) => i.kind === 'automation_rule' },
];

const TRIGGER_GROUPS: TriggerGroupDef[] = [
  {
    id: 'flow',
    label: 'Flow',
    helper: 'Event and schedule triggers used by automation flows.',
    filter: (item) => item.category !== 'manual' && item.binding_kind === 'automation_rule',
  },
  {
    id: 'manual',
    label: 'Manual',
    helper: 'Human-started agent runs from product surfaces.',
    filter: (item) => item.category === 'manual',
  },
  {
    id: 'built_in',
    label: 'Built-in',
    helper: 'Product-owned triggers configured from feature settings.',
    filter: (item) => item.category !== 'manual' && item.binding_kind !== 'automation_rule',
  },
];

function buildExecutionHistoryHref(
  slug: string | undefined,
  filters: Partial<AutomationTriggerExecutionFilters> & AutomationTriggerExecutionSearchPreset,
  basePath?: string,
) {
  const base = basePath ?? buildAutomationActivityPath(slug);
  const params = new URLSearchParams();
  if (filters.agent_id) params.set('agent_id', filters.agent_id);
  if (filters.binding_id) params.set('binding_id', filters.binding_id);
  if (filters.trigger_type) params.set('trigger_type', filters.trigger_type);
  if (filters.source) params.set('source', filters.source);
  if (filters.reference_id) params.set('reference_id', filters.reference_id);
  if (filters.status) params.set('status', filters.status);
  if (filters.fired_after) params.set('fired_after', filters.fired_after);
  if (filters.fired_before) params.set('fired_before', filters.fired_before);
  const query = params.toString();
  return `${base}${query ? `?${query}` : ''}#trigger-executions`;
}

function buildWorkflowHref(slug: string | undefined, search?: WorkflowRuleSearchPreset, basePath?: string) {
  if (!search) {
    return undefined;
  }
  const workflowsBase = basePath ?? buildAutomationFlowsPath(slug);
  const params = new URLSearchParams();
  if (search.show_trigger) params.set('show_trigger', search.show_trigger);
  if (search.show_trigger_title) params.set('show_trigger_title', search.show_trigger_title);
  if (search.template) params.set('template', search.template);
  if (search.template_title) params.set('template_title', search.template_title);
  if (search.template_description) params.set('template_description', search.template_description);
  if (search.create_event_rule) params.set('create_event_rule', '1');
  if (search.trigger_type) params.set('trigger_type', search.trigger_type);
  if (search.agent_id) params.set('agent_id', search.agent_id);
  if (search.repo_full_name) params.set('repo_full_name', search.repo_full_name);
  if (search.branch) params.set('branch', search.branch);
  if (search.base_branch) params.set('base_branch', search.base_branch);
  if (search.tag_name) params.set('tag_name', search.tag_name);
  if (search.conclusion) params.set('conclusion', search.conclusion);
  if (search.target_mode) params.set('target_mode', search.target_mode);
  if (search.target_id) params.set('target_id', search.target_id);
  return `${workflowsBase}?${params.toString()}`;
}

function triggerCountTooltip(item: AutomationTriggerCatalogEntry) {
  if (item.category === 'manual') {
    return `${item.binding_count} agent${item.binding_count === 1 ? '' : 's'} can be started manually from this surface.`;
  }
  if (item.binding_kind === 'automation_rule') {
    return `${item.binding_count} active flow${item.binding_count === 1 ? '' : 's'} use this trigger.`;
  }
  if (item.binding_kind === 'support_widget') {
    return item.binding_count > 0
      ? 'Support AI auto-replies are enabled with an assigned agent.'
      : 'Support AI auto-replies are not currently enabled with an assigned agent.';
  }
  return `${item.binding_count} active setup${item.binding_count === 1 ? '' : 's'} use this trigger.`;
}

function triggerCountLabel(item: AutomationTriggerCatalogEntry) {
  if (item.category === 'manual') {
    return `${item.binding_count} runnable agent${item.binding_count === 1 ? '' : 's'}`;
  }
  if (item.binding_kind === 'automation_rule') {
    return `${item.binding_count} active flow${item.binding_count === 1 ? '' : 's'}`;
  }
  if (item.binding_kind === 'support_widget') {
    return item.binding_count > 0 ? 'Enabled' : 'Not enabled';
  }
  return `${item.binding_count} active setup${item.binding_count === 1 ? '' : 's'}`;
}

function AutomationRow({ item, slug }: { item: AutomationInventoryItem; slug?: string }) {
  const managePath = displayPath(item.current_write_path, slug);
  const contextInfo = getContextInfo(item);

  return (
    <div className={item.enabled ? '' : 'opacity-60'}>
      <div className="flex items-center gap-3 px-4 py-2.5">
        <span className={`h-2 w-2 shrink-0 rounded-full ${HEALTH_DOT[item.health.status]}`} />
        <span className="min-w-0 truncate text-sm font-medium">{item.title}</span>
        <Badge variant="outline" className="shrink-0 text-[10px]">
          {item.scope_label}
        </Badge>
        <span className="flex-1" />
        {contextInfo && (
          <span className="hidden shrink-0 text-xs text-muted-foreground sm:inline">
            {contextInfo}
          </span>
        )}
        <Badge
          variant="outline"
          className={`shrink-0 text-[10px] ${
            item.enabled
              ? 'border-emerald-500/30 bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400'
              : ''
          }`}
        >
          {item.enabled ? 'On' : 'Off'}
        </Badge>
        {managePath ? (
          <a href={managePath} className="shrink-0 text-muted-foreground hover:text-foreground">
            <ArrowRight01Icon className="h-4 w-4" />
          </a>
        ) : (
          <span className="w-4 shrink-0" />
        )}
      </div>
      {item.health.last_error_message && (
        <div className="mx-4 mb-2 rounded-sm border border-rose-200 bg-rose-50 px-3 py-1.5 text-xs text-rose-700 dark:border-rose-800 dark:bg-rose-950/30 dark:text-rose-400">
          {item.health.last_error_message}
        </div>
      )}
    </div>
  );
}

const TRIGGER_CATALOG_GRID_CLASS = 'lg:grid-cols-[minmax(18rem,1fr)_10rem_18rem]';

function TriggerUsageBadge({ item }: { item: AutomationTriggerCatalogEntry }) {
  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className="shrink-0 cursor-help text-[11.5px] text-quiet-text-tertiary">
            {triggerCountLabel(item)}
          </span>
        </TooltipTrigger>
        <TooltipContent side="top" className="max-w-64 text-xs leading-relaxed">
          {triggerCountTooltip(item)}
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  );
}

function TriggerRow({
  item,
  slug,
  activityBasePath,
  flowsBasePath,
}: {
  item: AutomationTriggerCatalogEntry;
  slug?: string;
  activityBasePath?: string;
  flowsBasePath?: string;
}) {
  const historyHref = item.execution_search ? buildExecutionHistoryHref(slug, item.execution_search, activityBasePath) : undefined;
  const createRuleHref = buildWorkflowHref(slug, item.create_rule_search, flowsBasePath);
  const showRulesHref = buildWorkflowHref(slug, item.show_rules_search, flowsBasePath);
  const actions = [
    createRuleHref ? { href: createRuleHref, label: 'Create flow' } : undefined,
    showRulesHref ? { href: showRulesHref, label: 'View flows' } : undefined,
    historyHref ? { href: historyHref, label: 'View history' } : undefined,
  ].filter((action): action is { href: string; label: string } => Boolean(action));

  return (
    <div
      className={`grid gap-3 border-b border-border/60 px-[14px] py-[13px] transition-colors hover:bg-muted/40 focus-within:bg-muted/40 lg:items-center lg:gap-4 ${TRIGGER_CATALOG_GRID_CLASS}`}
    >
      <div className="min-w-0">
        <div className="truncate text-sm font-medium">{item.title}</div>
        <div className="mt-0.5 text-xs leading-relaxed text-muted-foreground">{item.description}</div>
        <div className="mt-1.5 flex min-w-0 items-center gap-1 text-[11px] text-muted-foreground/90">
          <span className="shrink-0 font-medium">Source:</span>
          <span className="truncate">{item.source_surface}</span>
        </div>
      </div>

      <div className="hidden lg:flex">
        <TriggerUsageBadge item={item} />
      </div>

      <div className="hidden items-center justify-end gap-4 lg:flex">
        {actions.map((action) => (
          <a key={action.label} href={action.href} className="shrink-0 text-xs font-medium text-muted-foreground hover:text-foreground">
            {action.label}
          </a>
        ))}
      </div>

      <div className="flex flex-wrap items-center gap-x-4 gap-y-2 lg:hidden">
        <TriggerUsageBadge item={item} />
        {actions.map((action) => (
          <a key={action.label} href={action.href} className="shrink-0 text-xs font-medium text-muted-foreground hover:text-foreground">
            {action.label}
          </a>
        ))}
      </div>
    </div>
  );
}

function filterTriggerCatalog(items: AutomationTriggerCatalogEntry[], query: string) {
  const normalizedQuery = query.trim().toLocaleLowerCase();
  if (!normalizedQuery) return items;

  return items.filter((item) => (
    [item.title, item.description, item.source_surface, item.trigger_type]
      .some((value) => value.toLocaleLowerCase().includes(normalizedQuery))
  ));
}

export function TriggerCatalogList({
  items,
  slug,
  activityBasePath,
  flowsBasePath,
}: {
  items: AutomationTriggerCatalogEntry[];
  slug?: string;
  activityBasePath?: string;
  flowsBasePath?: string;
}) {
  const [activeTab, setActiveTab] = useState<TriggerGroupDef['id']>('flow');
  const [query, setQuery] = useState('');
  const groups = useMemo(() => TRIGGER_GROUPS.map((group) => ({
    ...group,
    items: items.filter(group.filter),
  })), [items]);

  const selectedGroup = groups.find((group) => group.id === activeTab) ?? groups[0];
  const visibleItems = selectedGroup ? filterTriggerCatalog(selectedGroup.items, query) : [];

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <Tabs value={activeTab} onValueChange={(value) => setActiveTab(value as TriggerGroupDef['id'])} className="max-w-full gap-0 overflow-x-auto">
          <TabsList variant="quiet" aria-label="Trigger category" className="border-b-0">
          {groups.map((group) => (
            <TabsTrigger key={group.id} value={group.id} title={group.helper}>
              {group.label}
              <span className="text-[12px] font-normal tabular-nums text-quiet-muted">{group.items.length}</span>
            </TabsTrigger>
          ))}
          </TabsList>
        </Tabs>

        <div className="relative w-full sm:w-64">
          <Search01Icon className="pointer-events-none absolute left-0.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-quiet-muted" />
          <QuietUnderlineInput
            type="search"
            aria-label="Search triggers"
            placeholder="Search triggers..."
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            className="w-full pl-6"
          />
        </div>
      </div>

      {selectedGroup ? (
        visibleItems.length > 0 ? (
          <div className="border-t border-quiet-divider-strong">
            <div className={`hidden gap-4 border-b border-quiet-divider-light px-[14px] py-2 text-[11px] font-medium uppercase tracking-[0.06em] text-quiet-muted lg:grid ${TRIGGER_CATALOG_GRID_CLASS}`}>
              <div>Trigger</div>
              <div>Usage</div>
              <div className="text-right">Actions</div>
            </div>
            {visibleItems.map((item) => (
              <TriggerRow
                key={item.id}
                item={item}
                slug={slug}
                activityBasePath={activityBasePath}
                flowsBasePath={flowsBasePath}
              />
            ))}
          </div>
        ) : (
          <QuietEmptyState
            title={query.trim() ? `No ${selectedGroup.label.toLocaleLowerCase()} triggers match “${query.trim()}”` : `No ${selectedGroup.label.toLocaleLowerCase()} triggers yet`}
            description={query.trim() ? 'Try a broader search or choose another trigger category.' : 'Triggers will appear here when this event surface becomes available.'}
          />
        )
      ) : null}
    </div>
  );
}

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
        · {items.length - errorCount - warningCount} healthy
      </span>
    </div>
  );
}

function TriggerExecutionTable({
  rows,
  slug,
  agentRunsBasePath,
}: {
  rows: AutomationTriggerExecutionListItem[];
  slug?: string;
  agentRunsBasePath?: string;
}) {
  return (
    <div className="overflow-x-auto">
      <Table>
        <TableHeader>
          <TableRow className="bg-muted/30 hover:bg-muted/30">
            <TableHead>Fired</TableHead>
            <TableHead>Status</TableHead>
            <TableHead>Trigger</TableHead>
            <TableHead>Agent</TableHead>
            <TableHead>Source</TableHead>
            <TableHead>Target</TableHead>
            <TableHead>Run</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => {
            const managePath = displayPath(row.manage_path, slug);
            const runHref = row.run_id ? (agentRunsBasePath ?? buildAutomationActivityPath(slug, { run_id: row.run_id })) : undefined;
            const triggerLabel = row.trigger_title || row.trigger_type || row.binding_title;
            return (
              <TableRow key={row.execution_id}>
                <TableCell className="min-w-[180px]">
                  <div className="text-sm">{formatTimestamp(row.fired_at)}</div>
                  <div className="text-[11px] text-muted-foreground">{relativeTime(row.fired_at)}</div>
                </TableCell>
                <TableCell className="min-w-[120px]">
                  <Badge variant="outline" className={`capitalize ${EXECUTION_STATUS_BADGE[row.status] ?? ''}`}>
                    {row.status}
                  </Badge>
                  {row.error_message && (
                    <div className="mt-1 max-w-[220px] truncate text-[11px] text-rose-600 dark:text-rose-400" title={row.error_message}>
                      {row.error_message}
                    </div>
                  )}
                </TableCell>
                <TableCell className="min-w-[180px]">
                  <div className="text-sm">{triggerLabel}</div>
                  {row.trigger_type && (
                    <div className="font-mono text-[11px] text-muted-foreground">{row.trigger_type}</div>
                  )}
                </TableCell>
                <TableCell className="min-w-[180px]">
                  <div className="text-sm font-medium">{row.agent_name}</div>
                  <div className="font-mono text-[11px] text-muted-foreground">{truncateMiddle(row.agent_id, 8, 6)}</div>
                </TableCell>
                <TableCell className="min-w-[220px]">
                  <div className="text-sm">{row.reference_title || row.binding_title}</div>
                  <div className="flex flex-wrap items-center gap-2 text-[11px] text-muted-foreground">
                    <span className="capitalize">{row.binding_kind.replace('_', ' ')}</span>
                    {row.reference_id && <span className="font-mono">{truncateMiddle(row.reference_id, 8, 6)}</span>}
                    {managePath && (
                      <a href={managePath} className="hover:text-foreground">
                        Manage
                      </a>
                    )}
                  </div>
                </TableCell>
                <TableCell className="min-w-[180px]">
                  <div className="text-sm">{row.target_type || '--'}</div>
                  <div className="font-mono text-[11px] text-muted-foreground">{truncateMiddle(row.target_id, 8, 6)}</div>
                </TableCell>
                <TableCell className="min-w-[120px]">
                  {row.run_id ? (
                    <div className="space-y-1">
                      <div className="font-mono text-[11px] text-muted-foreground">{truncateMiddle(row.run_id, 8, 6)}</div>
                      {runHref && (
                        <a href={runHref} className="text-[11px] text-muted-foreground hover:text-foreground">
                          Open run
                        </a>
                      )}
                    </div>
                  ) : (
                    <span className="text-muted-foreground">--</span>
                  )}
                </TableCell>
              </TableRow>
            );
          })}
        </TableBody>
      </Table>
    </div>
  );
}

export function AutomationOverviewPanel({
  workspaceId,
  search,
  onSearchChange,
  sectionVisibility,
  pathOverrides,
}: AutomationOverviewPanelProps) {
  const { currentWorkspace } = useWorkspaceStore();
  const { data: agents = [] } = useAgents(workspaceId);
  const showStatusBar = sectionVisibility?.statusBar !== false;
  const showTriggerCatalog = sectionVisibility?.triggerCatalog !== false;
  const showTriggerExecutions = sectionVisibility?.triggerExecutions !== false;
  const showBuiltIns = sectionVisibility?.builtIns !== false;
  const showAutomationRules = sectionVisibility?.automationRules !== false;
  const needsOverview = showStatusBar || showBuiltIns || showAutomationRules;
  const overviewQuery = useAutomationOverview(workspaceId, needsOverview);
  const triggerCatalogQuery = useAutomationTriggerCatalog(workspaceId, showTriggerCatalog);

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

  const executionsQuery = useAutomationActivity(workspaceId, executionFilters, showTriggerExecutions);

  const slug = currentWorkspace?.slug;
  const items = needsOverview ? (overviewQuery.data?.items ?? []) : [];
  const triggerCatalog = triggerCatalogQuery.data ?? [];

  const triggerTypeOptions = useMemo(() => {
    const seen = new Set<string>();
    return triggerCatalog
      .filter((item) => item.supports_agent_runs)
      .filter((item) => {
        if (seen.has(item.trigger_type)) return false;
        seen.add(item.trigger_type);
        return true;
      })
      .map((item) => ({
        value: item.trigger_type,
        label: item.title,
      }));
  }, [triggerCatalog]);

  const subgroups = useMemo(() => {
    return SUBGROUPS.map((sg) => ({
      ...sg,
      items: items.filter(sg.filter),
    })).filter((sg) => sg.items.length > 0);
  }, [items]);

  const executions = executionsQuery.data?.data ?? [];
  const executionTotal = executionsQuery.data?.total ?? 0;
  const executionPage = executionsQuery.data?.page ?? search.page;
  const executionTotalPages = executionsQuery.data?.total_pages ?? 0;

  const updateExecutionFilters = (updates: Partial<AutomationOverviewPanelProps['search']>, preservePage = false) => {
    onSearchChange({
      ...updates,
      page: preservePage ? (updates.page ?? search.page) : (updates.page ?? 1),
    });
  };

  const resetExecutionFilters = () => {
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
  };

  if ((needsOverview && overviewQuery.isLoading) || (showTriggerCatalog && triggerCatalogQuery.isLoading)) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-10 w-full rounded-xl" />
        <Skeleton className="h-48 w-full rounded-xl" />
      </div>
    );
  }

  if ((needsOverview && overviewQuery.isError) || (showTriggerCatalog && triggerCatalogQuery.isError)) {
    const queryError = overviewQuery.error ?? triggerCatalogQuery.error;
    return (
      <QuietEmptyState
        title="Could not load automation data"
        description={queryError instanceof Error ? queryError.message : 'The automation catalog is temporarily unavailable.'}
      />
    );
  }

  return (
    <div className="space-y-4">
      {showStatusBar && items.length > 0 && (
        <div className="px-1">
          <StatusBar items={items} />
        </div>
      )}

      {showTriggerCatalog && triggerCatalog.length > 0 && (
        <TriggerCatalogList
          items={triggerCatalog}
          slug={slug}
          activityBasePath={pathOverrides?.activityBasePath}
          flowsBasePath={pathOverrides?.flowsBasePath}
        />
      )}

      {showTriggerExecutions && (
      <Card className={LINEAR_CARD_CLASS} id="trigger-executions">
        <CardHeader className="space-y-3 pb-2">
          <div className="flex items-center gap-2">
            <DashboardSpeed01Icon className="h-4 w-4 text-muted-foreground" />
            <CardTitle className="text-base">Trigger Executions</CardTitle>
            <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
              {executionTotal}
            </span>
          </div>
          <p className="text-sm text-muted-foreground">
            Workspace-wide trigger firing history for agents. Use filters to isolate noisy rules, GitHub webhooks, skipped launches, and failures.
          </p>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-6">
            <Select
              value={search.agent_id || '__all__'}
              onValueChange={(value) => updateExecutionFilters({ agent_id: value === '__all__' ? undefined : value })}
            >
              <SelectTrigger className="h-9 text-xs">
                <SelectValue placeholder="All agents" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All agents</SelectItem>
                {agents.map((agent) => (
                  <SelectItem key={agent.id} value={agent.id}>{agent.name}</SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Select
              value={search.trigger_type || '__all__'}
              onValueChange={(value) => updateExecutionFilters({ trigger_type: value === '__all__' ? undefined : value, binding_id: undefined })}
            >
              <SelectTrigger className="h-9 text-xs">
                <SelectValue placeholder="All trigger types" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="__all__">All trigger types</SelectItem>
                {triggerTypeOptions.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Select
              value={search.source || '__all__'}
              onValueChange={(value) => updateExecutionFilters({ source: value === '__all__' ? undefined : value })}
            >
              <SelectTrigger className="h-9 text-xs">
                <SelectValue placeholder="All sources" />
              </SelectTrigger>
              <SelectContent>
                {SOURCE_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Select
              value={search.status || '__all__'}
              onValueChange={(value) => updateExecutionFilters({ status: value === '__all__' ? undefined : value })}
            >
              <SelectTrigger className="h-9 text-xs">
                <SelectValue placeholder="All statuses" />
              </SelectTrigger>
              <SelectContent>
                {STATUS_OPTIONS.map((option) => (
                  <SelectItem key={option.value} value={option.value}>{option.label}</SelectItem>
                ))}
              </SelectContent>
            </Select>

            <Input
              type="date"
              value={search.fired_after || ''}
              onChange={(event) => updateExecutionFilters({ fired_after: trimFilterValue(event.target.value) })}
              className="h-9 text-xs"
            />

            <Input
              type="date"
              value={search.fired_before || ''}
              onChange={(event) => updateExecutionFilters({ fired_before: trimFilterValue(event.target.value) })}
              className="h-9 text-xs"
            />
          </div>

          {(search.binding_id || search.reference_id) && (
            <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
              {search.binding_id && (
                <Badge variant="outline" className="font-mono text-[10px]">
                  binding {search.binding_id}
                </Badge>
              )}
              {search.reference_id && (
                <Badge variant="outline" className="font-mono text-[10px]">
                  reference {search.reference_id}
                </Badge>
              )}
            </div>
          )}

          <div className="flex flex-wrap items-center justify-between gap-2">
            <p className="text-xs text-muted-foreground">
              Page {executionPage} of {Math.max(executionTotalPages, 1)}
            </p>
            <div className="flex items-center gap-2">
              <Button type="button" variant="ghost" size="sm" className="h-8 px-3 text-xs" onClick={resetExecutionFilters}>
                Reset filters
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 px-3 text-xs"
                disabled={executionPage <= 1 || executionsQuery.isLoading}
                onClick={() => updateExecutionFilters({ page: Math.max(1, executionPage - 1) }, true)}
              >
                Previous
              </Button>
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-8 px-3 text-xs"
                disabled={executionTotalPages === 0 || executionPage >= executionTotalPages || executionsQuery.isLoading}
                onClick={() => updateExecutionFilters({ page: executionPage + 1 }, true)}
              >
                Next
              </Button>
            </div>
          </div>

          {executionsQuery.isLoading ? (
            <Skeleton className="h-48 w-full rounded-xl" />
          ) : executionsQuery.isError ? (
            <div className="rounded-md border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              Could not load trigger executions
              {executionsQuery.error instanceof Error ? `: ${executionsQuery.error.message}` : ''}
            </div>
          ) : executions.length > 0 ? (
            <TriggerExecutionTable rows={executions} slug={slug} agentRunsBasePath={pathOverrides?.agentRunsBasePath} />
          ) : (
            <div className="rounded-md border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
              No trigger executions match the current filters.
            </div>
          )}
        </CardContent>
      </Card>
      )}

      {showBuiltIns && subgroups.length > 0 && (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-2">
            <div className="flex items-center gap-2">
              <SecurityCheckIcon className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Built-in Automations</CardTitle>
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {items.filter((i) => i.kind === 'built_in_automation').length}
              </span>
            </div>
          </CardHeader>
          <CardContent className="space-y-4 px-0 pb-2">
            {subgroups
              .filter((sg) => sg.items.some((i) => i.kind === 'built_in_automation'))
              .map((sg) => (
                <div key={sg.label}>
                  <div className="px-6 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    {sg.label}
                  </div>
                  <div className="divide-y divide-border/50">
                    {sg.items.map((item) => (
                      <AutomationRow key={item.inventory_id} item={item} slug={slug} />
                    ))}
                  </div>
                </div>
              ))}
          </CardContent>
        </Card>
      )}

      {showAutomationRules && subgroups.some((sg) => sg.items.some((i) => i.kind === 'automation_rule')) && (
        <Card className={LINEAR_CARD_CLASS}>
          <CardHeader className="pb-2">
            <div className="flex items-center gap-2">
              <BotIcon className="h-4 w-4 text-muted-foreground" />
              <CardTitle className="text-base">Automation Rules</CardTitle>
              <span className="rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-medium text-muted-foreground">
                {items.filter((i) => i.kind === 'automation_rule').length}
              </span>
            </div>
          </CardHeader>
          <CardContent className="px-0 pb-2">
            {subgroups
              .filter((sg) => sg.items.some((i) => i.kind === 'automation_rule'))
              .map((sg) => (
                <div key={sg.label}>
                  <div className="px-6 pb-1 text-[11px] font-medium uppercase tracking-wide text-muted-foreground">
                    {sg.label}
                  </div>
                  <div className="divide-y divide-border/50">
                    {sg.items
                      .filter((i) => i.kind === 'automation_rule')
                      .map((item) => (
                        <AutomationRow key={item.inventory_id} item={item} slug={slug} />
                      ))}
                  </div>
                </div>
              ))}
          </CardContent>
        </Card>
      )}
    </div>
  );
}

export const AIAutomationsTab = AutomationOverviewPanel;
