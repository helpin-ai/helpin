import { useEffect, useMemo, useState } from 'react';
import { ArrowDown01Icon, Cancel01Icon, Loading01Icon, Settings02Icon } from '@/lib/icons';
import { Checkbox } from '@/components/ui/checkbox';
import { Badge } from '@/components/ui/badge';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { commandBarService } from '@/lib/services/commandBarService';
import { cn } from '@/lib/utils';
import type { CommandBarToolCatalogEntry, CommandBarToolCatalogResponse } from '@/lib/pmTypes';

interface StepToolPickerProps {
  workspaceId: string | undefined;
  agentId: string;
  agentName: string;
  /** Fallback tool list (raw names) used until the backend catalog loads. */
  availableTools: string[];
  selectedTools: string[] | undefined;
  onChange: (next: string[] | undefined) => void;
  disabled?: boolean;
}

const COLLAPSE_KEY = (agentId: string) => `helpin:cmdk-tools:collapsed:${agentId}`;

function humanizeTool(name: string): string {
  return name
    .replace(/[_:]/g, ' ')
    .replace(/\b\w/g, (m) => m.toUpperCase());
}

function categoryOf(tool: CommandBarToolCatalogEntry): string {
  return tool.category?.trim() || 'Other';
}

export function StepToolPicker({
  workspaceId,
  agentId,
  agentName,
  availableTools,
  selectedTools,
  onChange,
  disabled,
}: StepToolPickerProps) {
  const [open, setOpen] = useState(false);
  const [filter, setFilter] = useState('');
  const [catalog, setCatalog] = useState<CommandBarToolCatalogResponse | null>(null);
  const [catalogLoading, setCatalogLoading] = useState(false);
  const [collapsedGroups, setCollapsedGroups] = useState<Set<string>>(() => {
    if (typeof window === 'undefined') return new Set();
    try {
      const raw = window.localStorage.getItem(COLLAPSE_KEY(agentId));
      return raw ? new Set(JSON.parse(raw) as string[]) : new Set();
    } catch {
      return new Set();
    }
  });
  const [showNotAvailable, setShowNotAvailable] = useState(false);

  // Lazy-load the catalog the first time the picker opens.
  useEffect(() => {
    if (!open || catalog || !workspaceId || !agentId) return;
    let cancelled = false;
    setCatalogLoading(true);
    void commandBarService
      .getAgentToolCatalog(workspaceId, agentId, selectedTools)
      .then((res) => {
        if (cancelled) return;
        if (res.data) setCatalog(res.data);
      })
      .finally(() => {
        if (!cancelled) setCatalogLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [agentId, catalog, open, selectedTools, workspaceId]);

  // Persist group collapse state per agent.
  useEffect(() => {
    if (typeof window === 'undefined') return;
    try {
      window.localStorage.setItem(COLLAPSE_KEY(agentId), JSON.stringify(Array.from(collapsedGroups)));
    } catch {
      /* ignore */
    }
  }, [agentId, collapsedGroups]);

  const allowedTools = catalog?.allowed_tools ?? availableTools;
  const total = allowedTools.length;

  // selectedTools === undefined means "use the agent's full allowlist".
  const effectiveSelected = useMemo(() => new Set(selectedTools ?? allowedTools), [selectedTools, allowedTools]);
  const selectedCount = effectiveSelected.size;
  const isNarrowed = selectedTools !== undefined && selectedTools.length !== allowedTools.length;
  const isEmpty = selectedTools !== undefined && selectedTools.length === 0;

  const allEntries: CommandBarToolCatalogEntry[] = useMemo(() => {
    if (catalog) return catalog.tools;
    return allowedTools.map((name) => ({
      id: name,
      name,
      description: '',
      category: '',
      allowed: true,
      selected: effectiveSelected.has(name),
    }));
  }, [allowedTools, catalog, effectiveSelected]);

  const filtered = useMemo(() => {
    const needle = filter.trim().toLowerCase();
    if (!needle) return allEntries;
    return allEntries.filter(
      (tool) =>
        tool.name.toLowerCase().includes(needle) ||
        tool.description.toLowerCase().includes(needle) ||
        (tool.disabled_reason ?? '').toLowerCase().includes(needle) ||
        tool.category.toLowerCase().includes(needle),
    );
  }, [allEntries, filter]);

  const allowedFiltered = useMemo(() => filtered.filter((t) => t.allowed), [filtered]);
  const notAllowedFiltered = useMemo(() => filtered.filter((t) => !t.allowed), [filtered]);

  const allowedByCategory = useMemo(() => groupByCategory(allowedFiltered), [allowedFiltered]);
  const notAllowedByCategory = useMemo(() => groupByCategory(notAllowedFiltered), [notAllowedFiltered]);

  const toggle = (tool: string) => {
    const next = new Set(effectiveSelected);
    if (next.has(tool)) next.delete(tool);
    else next.add(tool);
    if (next.size === allowedTools.length && allowedTools.every((t) => next.has(t))) {
      onChange(undefined);
      return;
    }
    onChange(Array.from(next));
  };

  const toggleGroup = (groupId: string) => {
    setCollapsedGroups((prev) => {
      const next = new Set(prev);
      if (next.has(groupId)) next.delete(groupId);
      else next.add(groupId);
      return next;
    });
  };

  const allOn = () => onChange(undefined);

  const showCount = filtered.length;
  const totalCount = allEntries.length;

  if (total === 0 && !catalogLoading) {
    return (
      <span className="text-[11px] text-muted-foreground">
        {agentName} inherits its default toolset.
      </span>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="ghost"
          size="sm"
          disabled={disabled}
          className={cn(
            'h-7 gap-1.5 px-2 text-[11px] text-muted-foreground hover:text-foreground',
            isNarrowed && !isEmpty && 'text-foreground',
            isEmpty && 'text-destructive hover:text-destructive',
          )}
        >
          <Settings02Icon className="h-3 w-3" />
          {isEmpty
            ? 'Pick at least 1 tool'
            : isNarrowed
              ? `Tools: ${selectedCount} of ${total}`
              : `Tools: all ${total}`}
          {isNarrowed && !disabled ? (
            <button
              type="button"
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                allOn();
              }}
              className="ml-0.5 rounded-full p-0.5 text-muted-foreground transition hover:bg-muted hover:text-foreground"
              title="Reset to all tools"
            >
              <Cancel01Icon className="h-2.5 w-2.5" />
            </button>
          ) : null}
          <ArrowDown01Icon className={cn('h-3 w-3 transition-transform', open && 'rotate-180')} />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="start"
        side="top"
        className="w-80 p-0"
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => e.stopPropagation()}
      >
        <div className="border-b px-3 py-2">
          <div className="flex items-center justify-between gap-2">
            <p className="text-xs font-medium">Tools for {agentName}</p>
            {isNarrowed ? <Badge variant="outline" className="text-[10px]">narrowed</Badge> : null}
          </div>
          <p className="mt-0.5 text-[11px] text-muted-foreground">
            Uncheck to restrict this step. Server validates the subset.
          </p>
        </div>
        <div className="border-b px-3 py-2">
          <div className="relative">
            <input
              type="text"
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              placeholder="Filter tools..."
              className="h-7 w-full rounded border border-border/60 bg-background px-2 pr-12 text-xs outline-none focus:border-ring"
              autoFocus
              onKeyDown={(e) => {
                if (e.key === 'Escape' && filter) {
                  e.stopPropagation();
                  setFilter('');
                }
              }}
            />
            {filter ? (
              <button
                type="button"
                onClick={() => setFilter('')}
                className="absolute right-1 top-1/2 -translate-y-1/2 rounded-full p-0.5 text-muted-foreground hover:bg-muted hover:text-foreground"
                title="Clear filter (Esc)"
              >
                <Cancel01Icon className="h-2.5 w-2.5" />
              </button>
            ) : null}
          </div>
          {filter ? (
            <p className="mt-1 text-[10px] text-muted-foreground">
              Showing {showCount} of {totalCount}
            </p>
          ) : null}
        </div>
        <div className="max-h-72 overflow-y-auto">
          {catalogLoading ? (
            <div className="flex items-center justify-center gap-2 px-3 py-4 text-xs text-muted-foreground">
              <Loading01Icon className="h-3 w-3 animate-spin" />
              Loading catalog...
            </div>
          ) : (
            <>
              <ToolGroupList
                title={`Available (${allowedFiltered.length})`}
                groups={allowedByCategory}
                collapsedGroups={collapsedGroups}
                toggleGroup={toggleGroup}
                effectiveSelected={effectiveSelected}
                onToggleTool={toggle}
                emptyHint={filter ? 'No allowed tools match the filter.' : 'No allowed tools.'}
              />
              {notAllowedFiltered.length > 0 ? (
                <div className="border-t border-border/60 py-1">
                  <button
                    type="button"
                    onClick={() => setShowNotAvailable((prev) => !prev)}
                    className="flex w-full items-center justify-between px-3 py-1.5 text-[11px] font-medium uppercase tracking-wider text-muted-foreground hover:bg-muted/40"
                  >
                    <span>Not available ({notAllowedFiltered.length})</span>
                    <ArrowDown01Icon className={cn('h-3 w-3 transition-transform', showNotAvailable && 'rotate-180')} />
                  </button>
                  {showNotAvailable ? (
                    <ToolGroupList
                      title=""
                      groups={notAllowedByCategory}
                      collapsedGroups={collapsedGroups}
                      toggleGroup={toggleGroup}
                      effectiveSelected={effectiveSelected}
                      onToggleTool={toggle}
                      readOnly
                    />
                  ) : null}
                </div>
              ) : null}
            </>
          )}
        </div>
        <div className="flex items-center justify-between border-t px-3 py-2">
          <span className={cn('text-[11px] text-muted-foreground', isEmpty && 'text-destructive')}>
            {isEmpty ? 'At least one tool is required' : `${selectedCount} of ${total} enabled`}
          </span>
          <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" onClick={allOn} disabled={!isNarrowed}>
            Reset to all
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}

function groupByCategory(tools: CommandBarToolCatalogEntry[]): Array<[string, CommandBarToolCatalogEntry[]]> {
  const map = new Map<string, CommandBarToolCatalogEntry[]>();
  for (const tool of tools) {
    const key = categoryOf(tool);
    const existing = map.get(key);
    if (existing) existing.push(tool);
    else map.set(key, [tool]);
  }
  return Array.from(map.entries()).sort(([a], [b]) => a.localeCompare(b));
}

interface ToolGroupListProps {
  title: string;
  groups: Array<[string, CommandBarToolCatalogEntry[]]>;
  collapsedGroups: Set<string>;
  toggleGroup: (groupId: string) => void;
  effectiveSelected: Set<string>;
  onToggleTool: (tool: string) => void;
  emptyHint?: string;
  readOnly?: boolean;
}

function ToolGroupList({
  title,
  groups,
  collapsedGroups,
  toggleGroup,
  effectiveSelected,
  onToggleTool,
  emptyHint,
  readOnly,
}: ToolGroupListProps) {
  if (groups.length === 0) {
    return emptyHint ? <p className="px-3 py-3 text-center text-xs text-muted-foreground">{emptyHint}</p> : null;
  }
  return (
    <div>
      {title ? (
        <div className="px-3 pt-2 pb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground">{title}</div>
      ) : null}
      {groups.map(([category, tools]) => {
        const groupId = `${title}:${category}`;
        const collapsed = collapsedGroups.has(groupId);
        return (
          <div key={groupId}>
            <button
              type="button"
              onClick={() => toggleGroup(groupId)}
              className="flex w-full items-center justify-between px-3 py-1 text-[11px] font-medium text-muted-foreground hover:bg-muted/40"
            >
              <span className="uppercase tracking-wider">{category}</span>
              <span className="flex items-center gap-1.5">
                <span className="text-[10px] text-muted-foreground/70">{tools.length}</span>
                <ArrowDown01Icon className={cn('h-3 w-3 transition-transform', collapsed && '-rotate-90')} />
              </span>
            </button>
            {collapsed
              ? null
              : tools.map((tool) => (
                  <ToolRow
                    key={tool.name}
                    tool={tool}
                    checked={tool.allowed && effectiveSelected.has(tool.name)}
                    readOnly={readOnly || !tool.allowed}
                    onToggle={() => onToggleTool(tool.name)}
                  />
                ))}
          </div>
        );
      })}
    </div>
  );
}

interface ToolRowProps {
  tool: CommandBarToolCatalogEntry;
  checked: boolean;
  readOnly?: boolean;
  onToggle: () => void;
}

function ToolRow({ tool, checked, readOnly, onToggle }: ToolRowProps) {
  const [expanded, setExpanded] = useState(false);
  return (
    <div className={cn('flex items-start gap-2 px-3 py-1.5', readOnly ? 'cursor-default opacity-60' : 'cursor-pointer hover:bg-muted/60')}>
      <Checkbox
        checked={checked}
        disabled={readOnly}
        onCheckedChange={() => {
          if (!readOnly) onToggle();
        }}
        className="mt-0.5"
      />
      <button
        type="button"
        onClick={() => setExpanded((prev) => !prev)}
        className="min-w-0 flex-1 text-left"
      >
        <div className="flex items-center gap-1.5">
          <span className="truncate text-xs font-medium" title={tool.name}>
            {tool.description ? humanizeTool(tool.name) : tool.name}
          </span>
        </div>
        {tool.description ? (
          <p className={cn('text-[11px] text-muted-foreground', expanded ? 'whitespace-pre-wrap' : 'line-clamp-1')}>
            {tool.description}
          </p>
        ) : null}
        {tool.disabled_reason && readOnly ? (
          <p className="text-[10px] italic text-muted-foreground/70">{tool.disabled_reason}</p>
        ) : null}
      </button>
    </div>
  );
}
