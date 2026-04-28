import { useEffect, useMemo, useState } from 'react';
import { ArrowDown01Icon, Loading01Icon, Settings02Icon } from '@/lib/icons';
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

function humanizeTool(name: string): string {
  return name
    .replace(/[_:]/g, ' ')
    .replace(/\b\w/g, (m) => m.toUpperCase());
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

  const allowedTools = catalog?.allowed_tools ?? availableTools;
  const total = allowedTools.length;

  // selectedTools === undefined means "use the agent's full allowlist".
  const effectiveSelected = useMemo(() => new Set(selectedTools ?? allowedTools), [selectedTools, allowedTools]);
  const selectedCount = effectiveSelected.size;
  const isNarrowed = selectedTools !== undefined && selectedTools.length !== allowedTools.length;
  const isEmpty = selectedTools !== undefined && selectedTools.length === 0;

  const displayEntries: CommandBarToolCatalogEntry[] = useMemo(() => {
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

  const filteredEntries = useMemo(() => {
    if (!filter.trim()) return displayEntries;
    const needle = filter.trim().toLowerCase();
    return displayEntries.filter(
      (tool) =>
        tool.name.toLowerCase().includes(needle) ||
        tool.description.toLowerCase().includes(needle) ||
        (tool.disabled_reason ?? '').toLowerCase().includes(needle) ||
        tool.category.toLowerCase().includes(needle),
    );
  }, [displayEntries, filter]);

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

  const allOn = () => onChange(undefined);

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
            isNarrowed && 'text-foreground',
            isEmpty && 'text-destructive hover:text-destructive',
          )}
        >
          <Settings02Icon className="h-3 w-3" />
          {isEmpty
            ? 'Pick at least 1 tool'
            : isNarrowed
              ? `Tools: ${selectedCount} of ${total}`
              : `Tools: all ${total}`}
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
            {isNarrowed ? (
              <Badge variant="outline" className="text-[10px]">narrowed</Badge>
            ) : null}
          </div>
          <p className="mt-0.5 text-[11px] text-muted-foreground">
            Uncheck to restrict this step. Server validates the subset.
          </p>
        </div>
        <div className="border-b px-3 py-2">
          <input
            type="text"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
            placeholder="Filter tools..."
            className="h-7 w-full rounded border border-border/60 bg-background px-2 text-xs outline-none focus:border-ring"
            autoFocus
          />
        </div>
        <div className="max-h-64 overflow-y-auto py-1">
          {catalogLoading ? (
            <div className="flex items-center justify-center gap-2 px-3 py-4 text-xs text-muted-foreground">
              <Loading01Icon className="h-3 w-3 animate-spin" />
              Loading catalog...
            </div>
          ) : filteredEntries.length === 0 ? (
            <p className="px-3 py-3 text-center text-xs text-muted-foreground">No tools match "{filter}"</p>
          ) : (
            filteredEntries.map((tool) => {
              const checked = tool.allowed && effectiveSelected.has(tool.name);
              return (
                <label
                  key={tool.name}
                  className={cn(
                    'flex items-start gap-2 px-3 py-1.5',
                    tool.allowed ? 'cursor-pointer hover:bg-muted/60' : 'cursor-not-allowed opacity-60',
                  )}
                >
                  <Checkbox
                    checked={checked}
                    disabled={!tool.allowed}
                    onCheckedChange={() => {
                      if (tool.allowed) toggle(tool.name);
                    }}
                    className="mt-0.5"
                  />
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <span className="truncate text-xs font-medium" title={tool.name}>
                        {tool.description ? humanizeTool(tool.name) : tool.name}
                      </span>
                      {tool.category ? (
                        <Badge variant="outline" className="text-[9px] uppercase tracking-wider">{tool.category}</Badge>
                      ) : null}
                    </div>
                    {tool.description ? (
                      <p className="line-clamp-2 text-[11px] text-muted-foreground">{tool.description}</p>
                    ) : null}
                    {!tool.allowed && tool.disabled_reason ? (
                      <p className="line-clamp-2 text-[11px] text-destructive/80">{tool.disabled_reason}</p>
                    ) : null}
                  </div>
                </label>
              );
            })
          )}
        </div>
        <div className="flex items-center justify-between border-t px-3 py-2">
          <span className={cn('text-[11px] text-muted-foreground', isEmpty && 'text-destructive')}>
            {isEmpty ? 'At least one tool is required' : `${selectedCount} of ${total} enabled`}
          </span>
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="sm" className="h-6 px-2 text-[11px]" onClick={allOn}>
              Reset to all
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
