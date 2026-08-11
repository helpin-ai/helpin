import { useId, useMemo, useState } from 'react';
import {
  ArrowDown01Icon,
  ArrowRight01Icon,
  SourceCodeIcon,
  FolderOpenIcon,
  GitBranchIcon,
  GlobeIcon,
  Search01Icon,
  ChartIncreaseIcon,
  File01Icon,
  Tick01Icon,
  FileSearchIcon,
  Message01Icon,
  MessagePreview01Icon,
  Building03Icon,
  TerminalIcon,
  Wrench01Icon,
  BookOpen01Icon,
} from '@/lib/icons';

import { Badge } from '@/components/ui/badge';
import { Input } from '@/components/ui/input';
import { useAutomationToolCatalog } from '@/hooks/queries';
import { useTitle } from '@/hooks/useTitle';
import type { AgentPresetKey, ToolCatalogEntry } from '@/lib/pmTypes';
import { PRESET_STYLES } from '@/lib/presetStyles';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { cn } from '@/lib/utils';

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const CATEGORY_ICONS: Record<string, typeof Wrench01Icon> = {
  Filesystem: FolderOpenIcon,
  'Code Analysis': SourceCodeIcon,
  Skills: BookOpen01Icon,
  Commands: TerminalIcon,
  Security: Wrench01Icon,
  'Web Search': GlobeIcon,
  Browser: GlobeIcon,
  Git: GitBranchIcon,
  Interaction: MessagePreview01Icon,
  'PM / Tasks': FileSearchIcon,
  Workspace: Building03Icon,
  Support: Message01Icon,
  CRM: ChartIncreaseIcon,
  Docs: File01Icon,
  'External MCP': GlobeIcon,
};

// PRESET_STYLES imported from @/lib/presetStyles

// ---------------------------------------------------------------------------
// Sub-components
// ---------------------------------------------------------------------------

function PresetBadge({ preset }: { preset: AgentPresetKey }) {
  const style = PRESET_STYLES[preset];
  if (!style) return null;
  return (
    <span className={cn('inline-flex items-center rounded-md border px-1.5 py-0.5 text-[10px] font-medium', style.className)}>
      {style.label}
    </span>
  );
}

function ParamRow({
  name,
  prop,
  required,
}: {
  name: string;
  prop: { type: string; description?: string; items?: Record<string, unknown> };
  required: boolean;
}) {
  return (
    <tr className="border-b border-border/40 last:border-0">
      <td className="py-1.5 pr-3 font-mono text-xs text-foreground">{name}</td>
      <td className="py-1.5 pr-3 text-xs text-muted-foreground">{prop.type}</td>
      <td className="py-1.5 pr-3">
        {required ? (
          <span className="inline-flex items-center gap-0.5 text-[10px] font-medium text-emerald-600 dark:text-emerald-400">
            <Tick01Icon className="h-3 w-3" />
            required
          </span>
        ) : (
          <span className="text-[10px] text-muted-foreground/60">optional</span>
        )}
      </td>
      <td className="py-1.5 text-xs text-muted-foreground">{prop.description ?? ''}</td>
    </tr>
  );
}

export function ToolRow({ tool }: { tool: ToolCatalogEntry }) {
  const [open, setOpen] = useState(false);
  const paramsId = useId();
  const properties = tool.input_schema?.properties ?? {};
  const requiredSet = new Set(tool.input_schema?.required ?? []);
  const paramNames = Object.keys(properties);
  const hasParams = paramNames.length > 0;
  const rowContent = (
    <>
      <div className="min-w-0 space-y-1">
        <div className="flex flex-wrap items-center gap-2">
          <code className="text-sm font-semibold">{tool.name}</code>
          {tool.presets.map((preset) => (
            <PresetBadge key={preset} preset={preset} />
          ))}
        </div>
        <p className="text-xs leading-relaxed text-muted-foreground">{tool.description}</p>
      </div>

      {hasParams ? (
        <span className="flex shrink-0 items-center gap-1 text-xs text-muted-foreground">
          {paramNames.length} param{paramNames.length !== 1 ? 's' : ''}
          {open ? <ArrowDown01Icon className="h-3.5 w-3.5" /> : <ArrowRight01Icon className="h-3.5 w-3.5" />}
        </span>
      ) : null}
    </>
  );

  return (
    <div className="border-b border-border/60">
      {hasParams ? (
        <button
          type="button"
          className="grid w-full grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-[14px] py-[13px] text-left outline-none transition-colors duration-100 hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
          aria-expanded={open}
          aria-controls={paramsId}
          onClick={() => setOpen((current) => !current)}
        >
          {rowContent}
        </button>
      ) : (
        <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-4 px-[14px] py-[13px]">
          {rowContent}
        </div>
      )}

      {open && hasParams && (
        <div
          id={paramsId}
          role="region"
          aria-label={`${tool.name} parameters`}
          className="overflow-x-auto border-t border-border/40 px-[14px] py-3"
        >
          <table className="w-full min-w-[36rem]">
            <thead>
              <tr className="border-b border-border/40 text-left">
                <th className="pb-1 pr-3 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Name</th>
                <th className="pb-1 pr-3 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Type</th>
                <th className="pb-1 pr-3 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Status</th>
                <th className="pb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">Description</th>
              </tr>
            </thead>
            <tbody>
              {paramNames.map((name) => (
                <ParamRow
                  key={name}
                  name={name}
                  prop={properties[name]}
                  required={requiredSet.has(name)}
                />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

function CategorySection({
  category,
  tools,
  defaultOpen,
}: {
  category: string;
  tools: ToolCatalogEntry[];
  defaultOpen: boolean;
}) {
  const [open, setOpen] = useState(defaultOpen);
  const toolsId = useId();
  const Icon = CATEGORY_ICONS[category] ?? Wrench01Icon;

  return (
    <section>
      <button
        type="button"
        className="flex w-full items-center gap-2 border-b border-border px-[14px] py-2.5 text-left outline-none transition-colors duration-100 hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
        aria-expanded={open}
        aria-controls={toolsId}
        onClick={() => setOpen((current) => !current)}
      >
        {open ? <ArrowDown01Icon className="h-4 w-4 text-muted-foreground" /> : <ArrowRight01Icon className="h-4 w-4 text-muted-foreground" />}
        <Icon className="h-4 w-4 text-muted-foreground" />
        <span className="text-xs font-medium uppercase tracking-wide">{category}</span>
        <Badge variant="secondary" className="ml-1 px-1.5 py-0 text-[10px]">
          {tools.length}
        </Badge>
      </button>

      {open && (
        <div id={toolsId}>
          {tools.map((tool) => (
            <ToolRow key={tool.name} tool={tool} />
          ))}
        </div>
      )}
    </section>
  );
}

// ---------------------------------------------------------------------------
// Page
// ---------------------------------------------------------------------------

export function ToolCatalogContent({
  workspaceId,
  embedded = false,
}: {
  workspaceId: string;
  embedded?: boolean;
}) {
  const [search, setSearch] = useState('');
  const [activeCategory, setActiveCategory] = useState<string | null>(null);
  const { data: catalog, isLoading: loading } = useAutomationToolCatalog(workspaceId);

  const filtered = useMemo(() => {
    if (!catalog) return [];
    const q = search.toLowerCase().trim();
    return catalog.tools.filter((tool) => {
      if (activeCategory && tool.category !== activeCategory) return false;
      if (q && !tool.name.toLowerCase().includes(q) && !tool.description.toLowerCase().includes(q)) return false;
      return true;
    });
  }, [catalog, search, activeCategory]);

  const grouped = useMemo(() => {
    const order = catalog?.categories ?? [];
    const map = new Map<string, ToolCatalogEntry[]>();
    for (const tool of filtered) {
      const list = map.get(tool.category) ?? [];
      list.push(tool);
      map.set(tool.category, list);
    }
    const result: { category: string; tools: ToolCatalogEntry[] }[] = [];
    for (const cat of order) {
      const tools = map.get(cat);
      if (tools?.length) result.push({ category: cat, tools });
    }
    // Catch any "Other" category not in the ordered list.
    for (const [cat, tools] of map) {
      if (!order.includes(cat)) result.push({ category: cat, tools });
    }
    return result;
  }, [catalog, filtered]);

  if (loading) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        Loading tool catalog...
      </div>
    );
  }

  if (!catalog || catalog.tools.length === 0) {
    return (
      <div className="flex h-48 items-center justify-center text-sm text-muted-foreground">
        No tools available.
      </div>
    );
  }

  return (
    <div className={cn('space-y-6', !embedded && 'mx-auto max-w-5xl')}>
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        {!embedded && (
          <div>
            <h1 className="text-xl font-semibold">Tool Catalog</h1>
            <p className="text-sm text-muted-foreground">
              {catalog.tools.length} tools across {catalog.categories.length} categories
            </p>
          </div>
        )}
        <div className={cn('relative w-full', embedded ? 'sm:max-w-sm' : 'sm:w-64')}>
          <Search01Icon className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder="Search tools..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
      </div>

      {/* Category filter pills */}
      <div className="flex flex-wrap gap-1.5">
        <button
          type="button"
          className={cn(
            'rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
            activeCategory === null
              ? 'border-primary/30 bg-primary/10 text-primary'
              : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground',
          )}
          onClick={() => setActiveCategory(null)}
        >
          All
        </button>
        {catalog.categories.map((cat) => {
          const Icon = CATEGORY_ICONS[cat] ?? Wrench01Icon;
          return (
            <button
              key={cat}
              type="button"
              className={cn(
                'flex items-center gap-1 rounded-md border px-2.5 py-1 text-xs font-medium transition-colors',
                activeCategory === cat
                  ? 'border-primary/30 bg-primary/10 text-primary'
                  : 'border-border text-muted-foreground hover:bg-accent hover:text-foreground',
              )}
              onClick={() => setActiveCategory(activeCategory === cat ? null : cat)}
            >
              <Icon className="h-3 w-3" />
              {cat}
            </button>
          );
        })}
      </div>

      {/* Tool list */}
      {grouped.length === 0 ? (
        <div className="flex h-32 items-center justify-center text-sm text-muted-foreground">
          No tools match your search.
        </div>
      ) : (
        <div>
          {grouped.map(({ category, tools }) => (
            <CategorySection
              key={category}
              category={category}
              tools={tools}
              defaultOpen={grouped.length === 1 || activeCategory !== null}
            />
          ))}
        </div>
      )}
    </div>
  );
}

export function ToolCatalogPage({ embedded = false }: { embedded?: boolean }) {
  useTitle('Tool Catalog');
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id) ?? '';

  return <ToolCatalogContent workspaceId={workspaceId} embedded={embedded} />;
}
