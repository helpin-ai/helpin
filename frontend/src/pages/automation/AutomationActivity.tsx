import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { useLocation, useNavigate } from '@tanstack/react-router';
import { useQuery } from '@tanstack/react-query';
import { toast } from 'sonner';
import {
  ArrowReloadHorizontalIcon,
  ArrowUpRight01Icon,
  ArrowLeft02Icon,
  BotIcon,
  Cancel01Icon,
  FilterHorizontalIcon,
  Loading01Icon,
  Tick01Icon,
} from '@/lib/icons';
import { AgentAvatar } from '@/components/agents/AgentAvatar';
import { AutomationShell } from '@/components/automation/AutomationShell';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { openEpicRoute } from '@/components/pm/epic-detail/epicRouteNavigation';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus, isPausedAgentRun } from '@/components/pm/agentRunConstants';
import { openTaskRoute } from '@/components/pm/task-detail/taskRouteNavigation';
import { LINEAR_CARD_CLASS } from '@/components/settings/settingsConstants';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useAgents, useAutomationActivity, useAutomationOverview, useAutomationTriggerCatalog, useWorkspaceAccess, usePermissions } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import { buildAutomationFlowsPath } from '@/lib/automationUi';
import { queryKeys } from '@/lib/queryKeys';
import { unwrap } from '@/lib/queryUtils';
import { automationService } from '@/lib/services/automationService';
import type { Agent, AgentRun } from '@/lib/pmTypes';
import type {
  AutomationInventoryItem,
  AutomationTriggerExecutionFilters,
  AutomationTriggerExecutionListItem,
} from '@/lib/types';
import { cn } from '@/lib/utils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  buildActivityTargetPresentation,
  buildExecutionMetadataPresentation,
  buildExecutionTargetPresentation,
  formatAttentionWaitDuration,
  normalizeActivityTargetType,
  type ActivityTargetPresentation,
} from './automationActivityRunPresentation';

export type AutomationActivitySearch = {
  page?: number;
  execution_id?: string;
  agent_id?: string;
  binding_id?: string;
  trigger_type?: string;
  status?: string;
  source?: string;
  reference_id?: string;
  run_id?: string;
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

const STATUS_DOT_STYLES: Record<string, string> = {
  completed: 'text-emerald-700 dark:text-emerald-400',
  failed: 'text-rose-700 dark:text-rose-400',
  running: 'text-sky-700 dark:text-sky-400',
  queued: 'text-muted-foreground',
  paused: 'text-amber-700 dark:text-amber-400',
  awaiting_approval: 'text-amber-700 dark:text-amber-400',
  awaiting_input: 'text-amber-700 dark:text-amber-400',
  awaiting_auth: 'text-amber-700 dark:text-amber-400',
  cancelled: 'text-muted-foreground',
  skipped: 'text-muted-foreground',
};

const STATUS_FILTER_OPTIONS = [
  { value: 'queued', label: 'Queued' },
  { value: 'running', label: 'Running' },
  { value: 'paused', label: 'Paused' },
  { value: 'completed', label: 'Completed' },
  { value: 'failed', label: 'Failed' },
  { value: 'cancelled', label: 'Cancelled' },
  { value: 'skipped', label: 'Skipped' },
];

const SOURCE_FILTER_OPTIONS = [
  { value: 'automation_rule', label: 'Flow' },
  { value: 'manual', label: 'Manual' },
  { value: 'schedule', label: 'Schedule' },
  { value: 'support_widget', label: 'Support' },
  { value: 'task_assignment', label: 'Task assignment' },
];

const DATE_FILTER_OPTIONS = [
  { value: 'all', label: 'All time' },
  { value: '24h', label: 'Last 24h' },
  { value: '7d', label: 'Last 7d' },
  { value: '30d', label: 'Last 30d' },
  { value: '90d', label: 'Last 90d' },
  { value: '180d', label: 'Last 6 months' },
  { value: '365d', label: 'Last 12 months' },
];

type ActivityFilterKey = 'status' | 'reference_id' | 'agent_id' | 'source' | 'trigger_type' | 'binding_id' | 'date';
type ActivityFilterState = Partial<Record<ActivityFilterKey, string[]>>;

interface ActivityFilterOption {
  value: string;
  label: string;
  icon?: ReactNode;
}

interface ActivityFilterDefinition {
  key: ActivityFilterKey;
  label: string;
  options: ActivityFilterOption[];
  searchableValues?: boolean;
  singleSelect?: boolean;
}

function ActivityFilterValueSelect({
  definition,
  selected,
  onToggle,
}: {
  definition: ActivityFilterDefinition;
  selected: string[];
  onToggle: (value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const selectedLabels = selected.map((value) =>
    definition.options.find((option) => option.value === value)?.label ?? value,
  );

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button className="inline-flex max-w-[180px] items-center gap-1 rounded border border-border bg-background px-1.5 py-0.5 text-xs transition-colors hover:bg-accent">
          <span className="truncate">
            {selectedLabels.length === 0
              ? 'Choose value'
              : selectedLabels.length === 1
                ? selectedLabels[0]
                : `${selectedLabels.length} selected`}
          </span>
        </button>
      </PopoverTrigger>
      <PopoverContent className={`${definition.searchableValues ? 'w-80' : 'w-56'} p-0`} align="start">
        <Command>
          {definition.searchableValues ? (
            <CommandInput placeholder={`Search ${definition.label.toLowerCase()}...`} />
          ) : null}
          <CommandList>
            <CommandEmpty>No results.</CommandEmpty>
            <CommandGroup>
              {definition.options.map((option) => {
                const isSelected = selected.includes(option.value);
                return (
                  <CommandItem
                    key={option.value}
                    value={option.label}
                    onSelect={() => {
                      onToggle(option.value);
                      if (definition.singleSelect) setOpen(false);
                    }}
                  >
                    <div className={cn(
                      'mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border',
                      isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40',
                    )}>
                      {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                    </div>
                    {option.icon ? <span className="mr-1.5 shrink-0">{option.icon}</span> : null}
                    <span className="min-w-0 flex-1 truncate">{option.label}</span>
                  </CommandItem>
                );
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  );
}

function ActivityFilterPill({
  definition,
  selected,
  onToggle,
  onRemove,
}: {
  definition: ActivityFilterDefinition;
  selected: string[];
  onToggle: (value: string) => void;
  onRemove: () => void;
}) {
  return (
    <div className="inline-flex max-w-full items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
      <span className="font-medium text-muted-foreground">{definition.label}</span>
      <span className="text-muted-foreground/60">is</span>
      <ActivityFilterValueSelect definition={definition} selected={selected} onToggle={onToggle} />
      <button
        type="button"
        onClick={onRemove}
        className="ml-0.5 rounded p-0.5 text-muted-foreground/60 transition-colors hover:bg-accent hover:text-foreground"
        aria-label={`Remove ${definition.label} filter`}
      >
        <Cancel01Icon className="h-3 w-3" />
      </button>
    </div>
  );
}

function ActivityFilterTrigger({
  definitions,
  filterState,
  visibleKeys,
  activeCount,
  onAdd,
  onToggle,
}: {
  definitions: ActivityFilterDefinition[];
  filterState: ActivityFilterState;
  visibleKeys: ActivityFilterKey[];
  activeCount: number;
  onAdd: (key: ActivityFilterKey) => void;
  onToggle: (key: ActivityFilterKey, value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<ActivityFilterKey | null>(null);
  const visible = new Set(visibleKeys);
  const available = definitions.filter((definition) => !visible.has(definition.key) && definition.options.length > 0);
  const selectedDefinition = selectedKey
    ? definitions.find((definition) => definition.key === selectedKey)
    : undefined;
  const canChooseFilter = available.length > 0 || Boolean(selectedDefinition);

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen);
    if (!nextOpen) setSelectedKey(null);
  };

  return canChooseFilter ? (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <FilterHorizontalIcon className="h-3.5 w-3.5" />
            Filters
          </span>
          <Badge
            variant="secondary"
            className={cn('rounded-full px-1.5 py-0 text-[10px] transition-opacity', activeCount > 0 ? 'opacity-100' : 'opacity-0')}
          >
            {activeCount || 0}
          </Badge>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className={`${selectedDefinition ? (selectedDefinition.searchableValues ? 'w-80' : 'w-56') : 'w-48'} p-0`}
        align="start"
      >
        {selectedDefinition ? (
          <Command>
            <div className="flex items-center gap-1 border-b border-border/70 px-1.5 py-1">
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="h-6 w-6"
                aria-label="Back to filter fields"
                onClick={() => setSelectedKey(null)}
              >
                <ArrowLeft02Icon className="h-3.5 w-3.5" />
              </Button>
              <span className="truncate text-xs font-medium">{selectedDefinition.label}</span>
            </div>
            {selectedDefinition.searchableValues ? (
              <CommandInput placeholder={`Search ${selectedDefinition.label.toLowerCase()}...`} />
            ) : null}
            <CommandList>
              <CommandEmpty>No results.</CommandEmpty>
              <CommandGroup>
                {selectedDefinition.options.map((option) => {
                  const isSelected = filterState[selectedDefinition.key]?.includes(option.value) ?? false;
                  return (
                    <CommandItem
                      key={option.value}
                      value={option.label}
                      onSelect={() => {
                        onToggle(selectedDefinition.key, option.value);
                        if (selectedDefinition.singleSelect) {
                          setOpen(false);
                          setSelectedKey(null);
                        }
                      }}
                    >
                      <div className={cn(
                        'mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border',
                        isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40',
                      )}>
                        {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                      </div>
                      {option.icon ? <span className="mr-1.5 shrink-0">{option.icon}</span> : null}
                      <span className="min-w-0 flex-1 truncate">{option.label}</span>
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        ) : (
          <Command>
            <CommandInput placeholder="Filter by..." />
            <CommandList>
              <CommandEmpty>No filters.</CommandEmpty>
              <CommandGroup>
                {available.map((definition) => (
                  <CommandItem
                    key={definition.key}
                    value={definition.label}
                    onSelect={() => {
                      onAdd(definition.key);
                      setSelectedKey(definition.key);
                    }}
                  >
                    {definition.label}
                  </CommandItem>
                ))}
              </CommandGroup>
            </CommandList>
          </Command>
        )}
      </PopoverContent>
    </Popover>
  ) : (
    <Button variant="ghost" size="sm" className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground" disabled>
      <span className="inline-flex items-center gap-1">
        <FilterHorizontalIcon className="h-3.5 w-3.5" />
        Filters
      </span>
      <Badge variant="secondary" className="ml-0.5 rounded-full px-1.5 py-0 text-[10px]">
        {activeCount}
      </Badge>
    </Button>
  );
}

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

function trimFilterValue(value?: string) {
  const trimmed = value?.trim();
  return trimmed || undefined;
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

function singleFilterValue(values: string[]) {
  return values.length > 0 ? values[values.length - 1] : undefined;
}

function multiFilterValues(value?: string) {
  return value?.split(',').map((item) => item.trim()).filter(Boolean) ?? [];
}

function multiFilterValue(values: string[]) {
  const unique = Array.from(new Set(values.map((item) => item.trim()).filter(Boolean)));
  return unique.length > 0 ? unique.join(',') : undefined;
}

function dateFilterValue(search: AutomationActivitySearch) {
  if (!search.fired_after || search.fired_before) return undefined;
  const firedAfter = search.fired_after.trim();
  for (const option of DATE_FILTER_OPTIONS) {
    if (option.value !== 'all' && firedAfter === getTimeFilterDate(option.value)) {
      return option.value;
    }
  }
  return undefined;
}

function flowIdFromInventoryItem(item: AutomationInventoryItem) {
  if (item.kind !== 'automation_rule') return null;
  const marker = 'automation_rule:rule:';
  if (item.inventory_id.startsWith(marker)) return item.inventory_id.slice(marker.length);
  return item.scope_type === 'workspace' ? item.scope_id : null;
}

function sourceLabel(value?: string) {
  switch (value) {
    case 'manual':
      return 'Manual';
    case 'automation_rule':
      return 'Flow';
    case 'schedule':
      return 'Scheduled';
    case 'support_widget':
      return 'Support';
    case 'command_bar':
      return 'Command bar';
    case 'agent_run':
      return 'Agent run';
    case 'task_assignment':
      return 'Assignment';
    default:
      return value ? value.replace(/_/g, ' ') : 'Manual';
  }
}

function buildExecutionTriggerLabel(item: AutomationTriggerExecutionListItem) {
  if (item.binding_kind === 'automation_rule' && item.trigger_type === 'manual') return 'Manual run';
  if (item.trigger_type === 'cron' || item.binding_id === 'automation_rule.cron') return 'Cron';
  if (item.binding_kind === 'manual') return 'Manual';
  if (item.binding_kind === 'automation_rule') return item.trigger_title || sourceLabel(item.binding_kind);
  return item.trigger_title || item.binding_title || sourceLabel(item.binding_kind);
}

function buildExecutionFlowHref(item: AutomationTriggerExecutionListItem, workspaceSlug?: string) {
  if (item.reference_type === 'automation_rule' && item.reference_id) {
    return buildAutomationFlowsPath(workspaceSlug, {
      show_rule: item.reference_id,
      show_rule_title: item.reference_title || item.binding_title || item.trigger_title || 'Flow',
    });
  }
  if (item.manage_path) return item.manage_path;
  if (item.reference_id) {
    return buildAutomationFlowsPath(workspaceSlug, { target_id: item.reference_id });
  }
  return undefined;
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

function TargetSummaryButton({
  target,
  onOpen,
  className,
}: {
  target: ActivityTargetPresentation;
  onOpen: () => void;
  className?: string;
}) {
  const openTitle = `Open ${target.typeLabel.toLowerCase()}`;
  const showTypePill = Boolean(target.targetType) && target.targetType !== 'workspace';
  const titleClasses = cn(
    'min-w-0 truncate text-left text-sm font-medium text-foreground',
    target.clickable && 'underline decoration-border underline-offset-4 hover:text-primary',
  );
  const content = (
    <>
      {showTypePill ? (
        <span className="inline-flex h-5 shrink-0 items-center rounded-full border border-border/70 bg-muted/40 px-2 text-[10px] font-medium uppercase leading-none tracking-normal text-muted-foreground">
          {target.typeLabel}
        </span>
      ) : null}
      {target.clickable ? (
        <Tooltip delayDuration={0}>
          <TooltipTrigger asChild>
            <button
              type="button"
              onClick={(event) => {
                event.stopPropagation();
                onOpen();
              }}
              className={titleClasses}
              aria-label={openTitle}
            >
              {target.primary}
            </button>
          </TooltipTrigger>
          <TooltipContent side="top">{openTitle}</TooltipContent>
        </Tooltip>
      ) : (
        <span className={titleClasses}>{target.primary}</span>
      )}
    </>
  );
  return (
    <span className={cn('inline-flex min-w-0 items-center gap-2', className)}>
      {content}
    </span>
  );
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
  onClick,
}: {
  label: string;
  value: string;
  sublabel: string;
  tone?: 'neutral' | 'good' | 'warn' | 'bad';
  spark?: number[];
  onClick?: () => void;
}) {
  const valueClass = tone === 'good'
    ? 'text-emerald-600 dark:text-emerald-400'
    : tone === 'warn'
      ? 'text-amber-600 dark:text-amber-400'
      : tone === 'bad'
        ? 'text-rose-600 dark:text-rose-400'
        : 'text-foreground';

  const interactive = Boolean(onClick);
  return (
    <Card
      className={cn(
        'border-border/70 bg-card/80 transition',
        interactive && 'cursor-pointer hover:border-border hover:bg-card focus-within:ring-2 focus-within:ring-ring/60',
      )}
    >
      <CardContent
        className={cn(
          'flex items-end justify-between gap-4 p-4',
          interactive && 'outline-none',
        )}
        role={interactive ? 'button' : undefined}
        tabIndex={interactive ? 0 : undefined}
        onClick={onClick}
        onKeyDown={
          interactive
            ? (event) => {
                if (event.key === 'Enter' || event.key === ' ') {
                  event.preventDefault();
                  onClick?.();
                }
              }
            : undefined
        }
      >
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
    <Badge variant="outline" className={cn('rounded-full px-2.5 py-0.5 text-[11px] font-medium', STATUS_STYLES[status] ?? STATUS_STYLES.queued)}>
      {SHORT_STATUS_LABELS[status] ?? status}
    </Badge>
  );
}

function NeedActionCard({
  run,
  agent,
  onOpenRun,
  onOpenTarget,
  onApprove,
  approving,
}: {
  run: AgentRun;
  agent?: Agent;
  onOpenRun: (runId: string) => void;
  onOpenTarget: (targetType: string, targetId: string) => void;
  onApprove: (runId: string) => Promise<void>;
  approving: boolean;
}) {
  const displayStatus = getAgentRunDisplayStatus(run);
  const waitTime = formatAttentionWaitDuration(run.created_at, new Date().toISOString());
  const needsApproval = displayStatus === 'awaiting_approval';
  const subtitle = runBlockingCopy(run, agent);
  const target = buildActivityTargetPresentation({ run });

  return (
    <div className="grid gap-4 rounded-2xl border border-border/70 bg-card/80 p-4 md:grid-cols-[1fr_auto] md:items-center">
      <div className="min-w-0 space-y-2 border-l-2 border-amber-500 pl-3">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <Badge variant="outline" className="rounded-full border-amber-500/40 bg-amber-500/10 text-[11px] font-medium text-amber-700 dark:text-amber-400">
            {runBlockingLabel(run)}
          </Badge>
          <TargetSummaryButton
            target={target}
            onOpen={() => onOpenTarget(target.targetType, target.targetId)}
            className="flex-1 text-[15px]"
          />
        </div>
        <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground">
          {agent ? <AgentAvatar agent={agent} className="h-6 w-6 rounded-none border-0 bg-transparent shadow-none" genericBare /> : null}
          <span>{agent?.name ?? 'Agent'}</span>
          <span className="text-muted-foreground/60">·</span>
          <span>blocked for <span className="font-mono text-foreground/80">{waitTime}</span></span>
          <span className="text-muted-foreground/60">·</span>
          <span className="font-mono">{formatShortDate(run.created_at)}</span>
        </div>
        <p className="text-xs text-muted-foreground">{subtitle}</p>
        {run.error_message ? (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-2.5 py-1.5 font-mono text-[11px] leading-relaxed text-destructive">
            {run.error_message}
          </div>
        ) : null}
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
  run,
  workspaceSlug,
  onOpenRun,
  onOpenFlow,
  onOpenTarget,
}: {
  item: AutomationTriggerExecutionListItem;
  agent?: Agent;
  run?: AgentRun;
  workspaceSlug?: string;
  onOpenRun: (runId: string) => void;
  onOpenFlow: (href: string) => void;
  onOpenTarget: (item: AutomationTriggerExecutionListItem) => void;
}) {
  const duration = formatDuration(item.started_at, item.completed_at);
  const target = buildExecutionTargetPresentation(item, run);
  const metadata = buildExecutionMetadataPresentation({
    ...item,
    trigger_title: buildExecutionTriggerLabel(item),
  });
  const flowHref = buildExecutionFlowHref(item, workspaceSlug);
  const canOpenRun = Boolean(item.run_id);

  const handleRowActivate = canOpenRun ? () => onOpenRun(item.run_id!) : undefined;

  return (
    <div
      className={cn(
        'group grid grid-cols-[1.25rem_minmax(0,1fr)_auto] gap-3 px-4 py-3 transition-colors',
        canOpenRun && 'cursor-pointer hover:bg-muted/40 focus-visible:bg-muted/40 focus-visible:outline-none',
      )}
      role={canOpenRun ? 'button' : undefined}
      tabIndex={canOpenRun ? 0 : undefined}
      onClick={handleRowActivate}
      onKeyDown={
        handleRowActivate
          ? (event) => {
              if (event.key === 'Enter' || event.key === ' ') {
                event.preventDefault();
                handleRowActivate();
              }
            }
          : undefined
      }
    >
      <div className="relative flex justify-center">
        <span className={cn(
          'relative z-10 mt-1 block h-[10px] w-[10px] min-w-[10px] shrink-0 rounded-full bg-current leading-none ring-4 ring-background',
          STATUS_DOT_STYLES[item.status] ?? STATUS_DOT_STYLES.queued,
          item.status === 'running' && 'animate-pulse',
        )} />
      </div>

      <div className="min-w-0 space-y-2">
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <StatusBadge status={item.status} />
          <TargetSummaryButton
            target={target}
            onOpen={() => onOpenTarget(item)}
            className="flex-1 text-[15px]"
          />
        </div>

        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
          <div className="flex min-w-0 items-center gap-1.5">
            {agent?.is_system ? (
              <AgentAvatar agent={agent} className="h-5 w-5 rounded-none border-0 bg-transparent shadow-none" genericBare />
            ) : (
              <span className="inline-flex h-5 w-5 items-center justify-center rounded-full border border-border/70 bg-muted/40">
                <BotIcon className="h-3 w-3" />
              </span>
            )}
            <span className="text-foreground">{metadata.subject}</span>
          </div>
          {metadata.sourceLabel ? (
            <>
              <span className="text-muted-foreground/40">·</span>
              {flowHref && metadata.sourceIsFlow ? (
                <Tooltip delayDuration={0}>
                  <TooltipTrigger asChild>
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation();
                        onOpenFlow(flowHref);
                      }}
                      className="inline-flex min-w-0 items-center gap-1 text-foreground underline decoration-border underline-offset-4 hover:text-primary"
                      aria-label="Open flow"
                    >
                      <ArrowReloadHorizontalIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                      <span className="truncate">{metadata.sourceLabel}</span>
                      <ArrowUpRight01Icon className="h-3 w-3 shrink-0" />
                    </button>
                  </TooltipTrigger>
                  <TooltipContent side="top">Open flow</TooltipContent>
                </Tooltip>
              ) : (
                <span className="inline-flex min-w-0 items-center gap-1 text-foreground">
                  {metadata.sourceIsFlow ? <ArrowReloadHorizontalIcon className="h-3.5 w-3.5 shrink-0 text-muted-foreground" /> : null}
                  <span className="truncate">{metadata.sourceLabel}</span>
                </span>
              )}
            </>
          ) : null}
          {metadata.triggerLabel ? (
            <>
              <span className="text-muted-foreground/40">·</span>
              <span className="text-foreground">{metadata.triggerLabel}</span>
            </>
          ) : null}
        </div>

        <div className="flex flex-wrap items-center gap-x-2 gap-y-1 text-[11px] text-muted-foreground">
          <span className="font-mono text-foreground/80">{relativeTime(item.fired_at)}</span>
          <span className="text-muted-foreground/40">·</span>
          <span>Duration <span className="font-mono text-foreground/80">{duration}</span></span>
          {item.actor_name ? (
            <>
              <span className="text-muted-foreground/40">·</span>
              <span>By <span className="font-medium text-foreground/80">{item.actor_name}</span></span>
            </>
          ) : null}
        </div>

        {item.error_message ? (
          <div className="rounded-md border border-destructive/30 bg-destructive/5 px-2.5 py-1.5 font-mono text-[11px] leading-relaxed text-destructive">
            ⚠ {item.error_message}
          </div>
        ) : null}
      </div>

      <div className="flex h-full flex-col items-end justify-center gap-2">
        {canOpenRun ? (
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="pointer-events-none opacity-0 transition-opacity group-hover:pointer-events-auto group-hover:opacity-100 group-focus-within:pointer-events-auto group-focus-within:opacity-100"
            onClick={(event) => {
              event.stopPropagation();
              onOpenRun(item.run_id!);
            }}
          >
            View run
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
  onSearchChange: (updates: Partial<AutomationActivitySearch>, options?: { preserveScroll?: boolean }) => void;
}) {
  useTitle('Automation Activity');
  const workspace = useWorkspaceStore((state) => state.currentWorkspace);
  const workspaceId = workspace?.id ?? '';
  const navigate = useNavigate();
  const location = useLocation();
  const { data: access } = useWorkspaceAccess(workspaceId);
  const permissions = usePermissions(access);

  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [approvingRunId, setApprovingRunId] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'timeline' | 'needs_you' | null>(null);
  const [visibleActivityFilterKeys, setVisibleActivityFilterKeys] = useState<ActivityFilterKey[]>([]);
  const [loadedExecutionPages, setLoadedExecutionPages] = useState(() => Math.max(1, search.page ?? 1));
  const hasActiveFilter = Boolean(
    search.execution_id
      || search.agent_id
      || search.binding_id
      || search.trigger_type
      || search.status
      || search.source
      || search.reference_id
      || search.run_id
      || search.fired_after
      || search.fired_before,
  );

  const openRun = useCallback((runId: string) => {
    setSelectedRunId(runId);
    setDrawerOpen(true);
  }, []);

  useEffect(() => {
    const runId = trimFilterValue(search.run_id);
    if (!runId) return;
    setSelectedRunId(runId);
    setDrawerOpen(true);
  }, [search.run_id]);

  const openFlow = useCallback((href: string) => {
    void navigate({ to: href });
  }, [navigate]);

  const openTargetByType = useCallback((targetTypeValue?: string | null, targetIdValue?: string | null) => {
    const slug = workspace?.slug;
    const targetID = targetIdValue?.trim();
    if (!slug || !targetID) return;

    switch (normalizeActivityTargetType(targetTypeValue)) {
      case 'task':
        openTaskRoute(navigate as never, { pathname: location.pathname } as never, slug, targetID);
        return;
      case 'epic':
        openEpicRoute(navigate as never, { pathname: location.pathname } as never, slug, targetID);
        return;
      case 'document':
        void navigate({
          to: '/w/$slug/docs/documents/$docId' as string,
          params: { slug, docId: targetID },
        });
        return;
      case 'support_conversation':
        void navigate({
          to: '/w/$slug/support/$conversationId' as string,
          params: { slug, conversationId: targetID },
        });
        return;
      case 'crm_contact':
        void navigate({
          to: '/w/$slug/crm/contacts/$contactId' as string,
          params: { slug, contactId: targetID },
        });
        return;
      case 'crm_deal':
        void navigate({
          to: '/w/$slug/crm/deals/$dealId' as string,
          params: { slug, dealId: targetID },
        });
        return;
      default:
        return;
    }
  }, [location.pathname, navigate, workspace?.slug]);

  const openTarget = useCallback((item: AutomationTriggerExecutionListItem) => {
    openTargetByType(item.target_type, item.target_id);
  }, [openTargetByType]);

  const { data: agents = [] } = useAgents(workspaceId);
  const overviewQuery = useAutomationOverview(workspaceId, true);
  const triggerCatalogQuery = useAutomationTriggerCatalog(workspaceId, permissions.canManageSettings);
  const activityFilterSignature = useMemo(
    () => JSON.stringify({
      execution_id: trimFilterValue(search.execution_id),
      agent_id: trimFilterValue(search.agent_id),
      binding_id: trimFilterValue(search.binding_id),
      trigger_type: trimFilterValue(search.trigger_type),
      status: trimFilterValue(search.status),
      source: trimFilterValue(search.source),
      reference_id: trimFilterValue(search.reference_id),
      run_id: trimFilterValue(search.run_id),
      fired_after: trimFilterValue(search.fired_after),
      fired_before: trimFilterValue(search.fired_before),
    }),
    [
      search.agent_id,
      search.binding_id,
      search.execution_id,
      search.fired_after,
      search.fired_before,
      search.reference_id,
      search.run_id,
      search.source,
      search.status,
      search.trigger_type,
    ],
  );

  useEffect(() => {
    setLoadedExecutionPages(Math.max(1, search.page ?? 1));
  }, [activityFilterSignature, search.page]);

  const executionFilters = useMemo<AutomationTriggerExecutionFilters>(
    () => ({
      page: 1,
      per_page: EXECUTIONS_PER_PAGE * loadedExecutionPages,
      execution_id: trimFilterValue(search.execution_id),
      agent_id: trimFilterValue(search.agent_id),
      binding_id: trimFilterValue(search.binding_id),
      trigger_type: trimFilterValue(search.trigger_type),
      status: trimFilterValue(search.status),
      source: trimFilterValue(search.source),
      reference_id: trimFilterValue(search.reference_id),
      run_id: trimFilterValue(search.run_id),
      fired_after: trimFilterValue(search.fired_after),
      fired_before: trimFilterValue(search.fired_before),
    }),
    [loadedExecutionPages, search],
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
  const isRefreshing = runsQuery.isFetching || executionsQuery.isFetching || overviewQuery.isFetching;

  const agentList = Array.isArray(agents) ? agents : [];
  const agentById = useMemo(() => new Map(agentList.map((agent) => [agent.id, agent])), [agentList]);
  const rawWorkspaceRuns = runsQuery.data?.data;
  const workspaceRuns = Array.isArray(rawWorkspaceRuns) ? rawWorkspaceRuns : [];
  const runById = useMemo(() => new Map(workspaceRuns.map((run) => [run.id, run])), [workspaceRuns]);
  const pausedRuns = useMemo(() => workspaceRuns.filter((run) => ACTIVE_RUN_STATUSES.has(run.status) && isPausedAgentRun(run)), [workspaceRuns]);
  const rawExecutions = executionsQuery.data?.data;
  const executions = Array.isArray(rawExecutions) ? rawExecutions : [];
  const executionTotal = executionsQuery.data?.total ?? 0;
  const hasMoreExecutions = executions.length < executionTotal;
  const rawItems = overviewQuery.data?.items;
  const items = Array.isArray(rawItems) ? rawItems : [];
  const agentFilterOptions = useMemo(
    () => agentList.map((agent) => ({
      value: agent.id,
      label: agent.name,
      icon: <AgentAvatar agent={agent} className="h-4 w-4" />,
    })),
    [agentList],
  );
  const triggerTypeFilterOptions = useMemo(() => {
    const seen = new Set<string>();
    return (triggerCatalogQuery.data ?? [])
      .filter((trigger) => {
        if (!trigger.trigger_type || seen.has(trigger.trigger_type)) return false;
        seen.add(trigger.trigger_type);
        return true;
      })
      .map((trigger) => ({
        value: trigger.trigger_type,
        label: trigger.title || trigger.trigger_type,
      }));
  }, [triggerCatalogQuery.data]);
  const triggerSurfaceFilterOptions = useMemo(
    () => (triggerCatalogQuery.data ?? []).map((trigger) => ({
      value: trigger.id,
      label: trigger.title || trigger.id,
    })),
    [triggerCatalogQuery.data],
  );
  const flowFilterOptions = useMemo(
    () => items
      .map((item) => {
        const id = flowIdFromInventoryItem(item);
        return id ? { value: id, label: item.title || id } : null;
      })
      .filter((item): item is { value: string; label: string } => Boolean(item)),
    [items],
  );
  const selectedDateFilter = dateFilterValue(search);
  const activityFilterDefinitions = useMemo<ActivityFilterDefinition[]>(
    () => [
      { key: 'status', label: 'Status', options: STATUS_FILTER_OPTIONS },
      { key: 'reference_id', label: 'Flow', options: flowFilterOptions, searchableValues: true },
      { key: 'agent_id', label: 'Agent', options: agentFilterOptions, searchableValues: true },
      { key: 'trigger_type', label: 'Trigger', options: triggerTypeFilterOptions, searchableValues: true },
      { key: 'binding_id', label: 'Surface', options: triggerSurfaceFilterOptions, searchableValues: true },
      { key: 'source', label: 'Source', options: SOURCE_FILTER_OPTIONS },
      { key: 'date', label: 'Date', options: DATE_FILTER_OPTIONS, singleSelect: true },
    ],
    [agentFilterOptions, flowFilterOptions, triggerSurfaceFilterOptions, triggerTypeFilterOptions],
  );
  const activityFilterState = useMemo<ActivityFilterState>(() => ({
    status: multiFilterValues(search.status),
    reference_id: multiFilterValues(search.reference_id),
    agent_id: multiFilterValues(search.agent_id),
    trigger_type: multiFilterValues(search.trigger_type),
    binding_id: multiFilterValues(search.binding_id),
    source: search.source && !(multiFilterValues(search.source).includes('automation_rule') && search.reference_id)
      ? multiFilterValues(search.source)
      : [],
    date: selectedDateFilter ? [selectedDateFilter] : [],
  }), [search.agent_id, search.binding_id, search.reference_id, search.source, search.status, search.trigger_type, selectedDateFilter]);
  const activeActivityFilterKeys = useMemo<ActivityFilterKey[]>(
    () => activityFilterDefinitions
      .map((definition) => definition.key)
      .filter((key) => (activityFilterState[key]?.length ?? 0) > 0),
    [activityFilterDefinitions, activityFilterState],
  );

  useEffect(() => {
    if (activeActivityFilterKeys.length === 0) return;
    setVisibleActivityFilterKeys((current) => {
      const next = [...current];
      for (const key of activeActivityFilterKeys) {
        if (!next.includes(key)) next.push(key);
      }
      return next.length === current.length ? current : next;
    });
  }, [activeActivityFilterKeys]);

  const setMultiFilter = useCallback((key: 'status' | 'agent_id' | 'source' | 'reference_id' | 'trigger_type' | 'binding_id', values: string[]) => {
    const value = multiFilterValue(values);
    if (key === 'reference_id') {
      onSearchChange({
        reference_id: value,
        source: value ? 'automation_rule' : undefined,
        execution_id: undefined,
        run_id: undefined,
        page: undefined,
      });
      return;
    }
    if (key === 'source') {
      const sourceValues = multiFilterValues(value);
      onSearchChange({
        source: value,
        reference_id: sourceValues.includes('automation_rule') ? search.reference_id : undefined,
        execution_id: undefined,
        run_id: undefined,
        page: undefined,
      });
      return;
    }
    onSearchChange({
      [key]: value,
      execution_id: undefined,
      run_id: undefined,
      page: undefined,
    });
  }, [onSearchChange, search.reference_id]);

  const setDateFilter = useCallback((values: string[]) => {
    const value = singleFilterValue(values);
    onSearchChange({
      fired_after: value && value !== 'all' ? getTimeFilterDate(value) : undefined,
      fired_before: undefined,
      execution_id: undefined,
      run_id: undefined,
      page: undefined,
    });
  }, [onSearchChange]);

  const handleActivityFilterAdd = useCallback((key: ActivityFilterKey) => {
    setVisibleActivityFilterKeys((current) => current.includes(key) ? current : [...current, key]);
  }, []);

  const handleActivityFilterToggle = useCallback((key: ActivityFilterKey, value: string) => {
    const current = activityFilterState[key] ?? [];
    const next = current.includes(value)
      ? current.filter((item) => item !== value)
      : [...current, value];
    if (key === 'date') {
      setDateFilter(current.includes(value) ? [] : [value]);
      return;
    }
    setMultiFilter(key, next);
  }, [activityFilterState, setDateFilter, setMultiFilter]);

  const handleActivityFilterRemove = useCallback((key: ActivityFilterKey) => {
    setVisibleActivityFilterKeys((current) => current.filter((item) => item !== key));
    if (key === 'date') {
      setDateFilter([]);
      return;
    }
    setMultiFilter(key, []);
  }, [setDateFilter, setMultiFilter]);

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

  const handleClearFilters = useCallback(() => {
    onSearchChange({
      page: undefined,
      execution_id: undefined,
      agent_id: undefined,
      binding_id: undefined,
      trigger_type: undefined,
      status: undefined,
      source: undefined,
      reference_id: undefined,
      run_id: undefined,
      fired_after: undefined,
      fired_before: undefined,
    });
  }, [onSearchChange]);

  const handleActivityClearFilters = useCallback(() => {
    setVisibleActivityFilterKeys([]);
    handleClearFilters();
  }, [handleClearFilters]);

  const applyAllRunsShortcut = useCallback((updates: Partial<AutomationActivitySearch>, visibleKeys: ActivityFilterKey[]) => {
    setActiveTab('timeline');
    setVisibleActivityFilterKeys(visibleKeys);
    onSearchChange({
      page: undefined,
      execution_id: undefined,
      agent_id: undefined,
      binding_id: undefined,
      trigger_type: undefined,
      status: undefined,
      source: undefined,
      reference_id: undefined,
      run_id: undefined,
      fired_after: undefined,
      fired_before: undefined,
      ...updates,
    });
  }, [onSearchChange]);

  const openNeedsYouShortcut = useCallback(() => {
    setActiveTab('needs_you');
    setVisibleActivityFilterKeys([]);
    handleClearFilters();
  }, [handleClearFilters]);

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

  const selectedActivityTab = activeTab ?? (pausedRuns.length > 0 && !hasActiveFilter ? 'needs_you' : 'timeline');

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
        <Button
          variant="outline"
          size="sm"
          disabled={isRefreshing}
          onClick={() => void Promise.all([runsQuery.refetch(), executionsQuery.refetch(), overviewQuery.refetch()])}
        >
          {isRefreshing
            ? <Loading01Icon className="mr-1.5 h-3.5 w-3.5 animate-spin" />
            : <ArrowReloadHorizontalIcon className="mr-1.5 h-3.5 w-3.5" />}
          Refresh
        </Button>
      )}
    >
      <div className="space-y-5 pb-20">
        <div className="grid gap-3 lg:grid-cols-4">
          <SummaryCard
            label="Needs You"
            value={String(pausedRuns.length)}
            sublabel={pausedRuns.length > 0 ? `Oldest blocked ${oldestBlocked ?? '\u2014'}` : 'No paused runs waiting on a human'}
            tone={pausedRuns.length > 0 ? 'warn' : 'neutral'}
            spark={needsYouBars}
            onClick={openNeedsYouShortcut}
          />
          <SummaryCard
            label="Runs · 24h"
            value={String(recent24hRuns.length)}
            sublabel={recent24hRuns.length > 0 ? `${recent24hRuns.filter((run) => run.status === 'completed').length} completed in the latest day` : 'No recent runs'}
            tone="neutral"
            spark={recentStatusBars}
            onClick={() => applyAllRunsShortcut({ fired_after: getTimeFilterDate('24h') }, ['date'])}
          />
          <SummaryCard
            label="Failed · 24h"
            value={String(recent24hRuns.filter((run) => run.status === 'failed').length)}
            sublabel={recent24hRuns.filter((run) => run.status === 'failed').length > 0 ? 'Investigate repeated failures and flaky flows' : 'No recent failures'}
            tone={recent24hRuns.some((run) => run.status === 'failed') ? 'bad' : 'neutral'}
            spark={recentFailureBars}
            onClick={() => applyAllRunsShortcut({ status: 'failed', fired_after: getTimeFilterDate('24h') }, ['status', 'date'])}
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
            onClick={
              workspace?.slug
                ? () => void navigate({ to: buildAutomationFlowsPath(workspace.slug) })
                : undefined
            }
          />
        </div>

        <Tabs value={selectedActivityTab} onValueChange={(value) => setActiveTab(value as 'timeline' | 'needs_you')} className="space-y-3">
          <div className="flex items-center justify-between">
            <TabsList variant="line">
              <TabsTrigger value="timeline">
                All runs
              </TabsTrigger>
              <TabsTrigger value="needs_you">
                Needs you
                {pausedRuns.length > 0 ? (
                  <Badge className="ml-1 h-5 rounded-full border-amber-500/30 bg-amber-500/15 px-1.5 text-[10px] font-semibold text-amber-700 dark:text-amber-300">
                    {pausedRuns.length}
                  </Badge>
                ) : null}
              </TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value="needs_you" className="mt-0 space-y-3">
            {pausedRuns.length > 0 ? (
              <div className="space-y-3">
                {pausedRuns.map((run) => (
                  <NeedActionCard
                    key={run.id}
                    run={run}
                    agent={agentById.get(run.agent_id)}
                    onOpenRun={openRun}
                    onOpenTarget={openTargetByType}
                    onApprove={handleApproveRun}
                    approving={approvingRunId === run.id}
                  />
                ))}
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-border/70 px-6 py-10 text-center text-sm text-muted-foreground">
                No runs need your attention.
              </div>
            )}
          </TabsContent>

          <TabsContent value="timeline" className="mt-0 space-y-3">
            <div className="flex flex-col gap-2">
              <div className="flex justify-start">
                <ActivityFilterTrigger
                  definitions={activityFilterDefinitions}
                  filterState={activityFilterState}
                  visibleKeys={visibleActivityFilterKeys}
                  activeCount={activeActivityFilterKeys.length}
                  onAdd={handleActivityFilterAdd}
                  onToggle={handleActivityFilterToggle}
                />
              </div>
              {visibleActivityFilterKeys.length > 0 ? (
                <div className="ui-divider-bottom-fade flex flex-wrap items-center justify-start gap-1.5 pb-2">
                  {activityFilterDefinitions
                    .filter((definition) => visibleActivityFilterKeys.includes(definition.key))
                    .map((definition) => (
                      <ActivityFilterPill
                        key={definition.key}
                        definition={definition}
                        selected={activityFilterState[definition.key] ?? []}
                        onToggle={(value) => handleActivityFilterToggle(definition.key, value)}
                        onRemove={() => handleActivityFilterRemove(definition.key)}
                      />
                    ))}
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-[10px] text-muted-foreground"
                    onClick={handleActivityClearFilters}
                  >
                    Clear all
                  </Button>
                </div>
              ) : null}
              {search.execution_id || search.run_id ? (
                <div className="flex flex-wrap items-center justify-end gap-2">
                  <p className="text-xs text-muted-foreground">
                    Showing a linked run. Clear filters to return to the full timeline.
                  </p>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    className="h-6 px-2 text-[10px] text-muted-foreground"
                    onClick={handleActivityClearFilters}
                  >
                    Clear all
                  </Button>
                </div>
              ) : null}
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
                      <div className="pl-3">
                        <div className="divide-y divide-border/60">
                          {group.rows.map((item) => (
                            <TimelineRow
                              key={item.execution_id}
                              item={item}
                              agent={agentById.get(item.agent_id)}
                              run={item.run_id ? runById.get(item.run_id) : undefined}
                              workspaceSlug={workspace?.slug}
                              onOpenRun={openRun}
                              onOpenFlow={openFlow}
                              onOpenTarget={openTarget}
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

            {executionTotal > 0 && (
              <div className="flex flex-col items-center gap-2 pt-1">
                <p className="text-xs text-muted-foreground">
                  Showing {executions.length} of {executionTotal} runs
                </p>
                {hasMoreExecutions ? (
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={executionsQuery.isFetching}
                    onClick={() => setLoadedExecutionPages((current) => current + 1)}
                  >
                    {executionsQuery.isFetching ? 'Loading…' : 'Load older runs'}
                  </Button>
                ) : null}
              </div>
            )}
          </TabsContent>
        </Tabs>
      </div>

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={(open) => {
          setDrawerOpen(open);
          if (!open && search.run_id) {
            onSearchChange({ run_id: undefined }, { preserveScroll: true });
          }
        }}
        title="Agent Run"
        description="Interactive transcript, approvals, artifacts, and session details."
      />
    </AutomationShell>
  );
}
