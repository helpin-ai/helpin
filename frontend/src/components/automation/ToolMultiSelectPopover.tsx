import { useEffect, useMemo, useState } from 'react';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { PlusSignIcon } from '@/lib/icons';
import type { ToolCatalogEntry } from '@/lib/pmTypes';
import { cn } from '@/lib/utils';

type ToolMultiSelectPopoverProps = {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  tools: ToolCatalogEntry[];
  selectedTools: string[];
  disabled?: boolean;
  onToggleTool: (toolName: string) => void;
};

export function ToolMultiSelectPopover({
  open,
  onOpenChange,
  tools,
  selectedTools,
  disabled,
  onToggleTool,
}: ToolMultiSelectPopoverProps) {
  const selected = new Set(selectedTools);
  const [search, setSearch] = useState('');
  const [category, setCategory] = useState('All');
  const categories = useMemo(
    () => ['All', ...Array.from(new Set(tools.map((tool) => tool.category).filter(Boolean))).sort()],
    [tools],
  );
  const filteredTools = useMemo(() => {
    const query = search.trim().toLowerCase();
    return tools.filter((tool) => {
      if (category !== 'All' && tool.category !== category) return false;
      if (!query) return true;
      return `${tool.name} ${tool.category} ${tool.description}`.toLowerCase().includes(query);
    });
  }, [category, search, tools]);

  useEffect(() => {
    if (!categories.includes(category)) setCategory('All');
  }, [categories, category]);

  const categoryCount = (value: string) => (value === 'All' ? tools.length : tools.filter((tool) => tool.category === value).length);

  return (
    <Popover open={open} onOpenChange={onOpenChange}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-8 gap-1.5 px-2 text-[11px]"
          disabled={tools.length === 0 || disabled}
        >
          <PlusSignIcon className="h-3.5 w-3.5" />
          Select tools
          {selectedTools.length > 0 && (
            <span className="rounded-full bg-muted px-1.5 py-0 text-[10px] text-muted-foreground">
              {selectedTools.length}
            </span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        align="end"
        className="w-[min(42rem,calc(100vw-3rem))] overflow-hidden p-0"
        onWheelCapture={(event) => event.stopPropagation()}
      >
        <div className="border-b border-border p-3">
          <Input
            className="h-9 rounded-md border-input bg-background text-sm"
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder="Search tools..."
          />
        </div>
        <div className="grid h-80 min-h-0 grid-cols-[11rem_minmax(0,1fr)]">
          <div className="min-h-0 overflow-y-auto overscroll-contain border-r border-border bg-muted/20 p-2">
            {categories.map((item) => (
              <button
                key={item}
                type="button"
                className={cn(
                  'flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-xs transition-colors',
                  category === item
                    ? 'bg-background text-foreground shadow-sm'
                    : 'text-muted-foreground hover:bg-background/70 hover:text-foreground',
                )}
                onClick={() => setCategory(item)}
              >
                <span className="truncate">{item}</span>
                <span className="text-[10px]">{categoryCount(item)}</span>
              </button>
            ))}
          </div>
          <div className="min-h-0 overflow-y-auto overscroll-contain p-2">
            {filteredTools.length > 0 ? (
              <div className="space-y-1.5">
                {filteredTools.map((tool) => {
                  const isSelected = selected.has(tool.name);
                  return (
                    <button
                      key={tool.name}
                      type="button"
                      className={cn(
                        'w-full rounded-md border px-2 py-2 text-left transition-colors',
                        isSelected ? 'border-primary/30 bg-primary/10' : 'border-transparent hover:bg-muted/50',
                      )}
                      onClick={() => onToggleTool(tool.name)}
                      aria-label={`${isSelected ? 'Remove' : 'Add'} ${tool.name}`}
                    >
                      <span className="flex min-w-0 items-center gap-2">
                        <span className="truncate font-mono text-xs text-foreground">{tool.name}</span>
                        <Badge variant="outline" className="shrink-0 text-[10px]">
                          {tool.category}
                        </Badge>
                        {isSelected && <span className="ml-auto text-[10px] font-medium text-primary">Selected</span>}
                      </span>
                      <span className="mt-1 block text-xs leading-relaxed text-muted-foreground">{tool.description}</span>
                    </button>
                  );
                })}
              </div>
            ) : (
              <p className="px-2 py-8 text-center text-sm text-muted-foreground">
                {tools.length === 0 ? 'Tool catalog unavailable.' : 'No tools found.'}
              </p>
            )}
          </div>
        </div>
        <div className="flex items-center justify-end border-t border-border/60 px-2 py-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 px-2 text-[11px]"
            onClick={() => onOpenChange(false)}
          >
            Done
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  );
}
