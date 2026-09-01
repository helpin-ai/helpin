import { useState, type ReactNode } from 'react';

import {
  ArrowLeft02Icon,
  Cancel01Icon,
  FilterHorizontalIcon,
  Tick01Icon,
} from '@/lib/icons';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';

export interface PMFilterOption {
  value: string;
  label: string;
  icon?: ReactNode;
}

export interface PMFilterDefinition<K extends string> {
  key: K;
  label: string;
  options: PMFilterOption[];
  singleSelect?: boolean;
  searchableValues?: boolean;
}

export type PMFilterValues<K extends string> = Partial<Record<K, string[]>>;

function PMFilterValueSelect<K extends string>({
  definition,
  selected,
  onToggle,
}: {
  definition: PMFilterDefinition<K>;
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
        <button type="button" className="inline-flex items-center gap-1 rounded border border-border bg-background px-1.5 py-0.5 text-xs transition-colors hover:bg-accent">
          {selectedLabels.length === 0
            ? 'Choose value'
            : selectedLabels.length === 1
              ? selectedLabels[0]
              : `${selectedLabels.length} selected`}
        </button>
      </PopoverTrigger>
      <PopoverContent className={`${definition.searchableValues ? 'w-80' : 'w-52'} p-0`} align="start">
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
                  <CommandItem key={option.value} value={option.label} onSelect={() => onToggle(option.value)}>
                    <div className={`mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}>
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

function PMFilterPill<K extends string>({
  definition,
  selected,
  onToggle,
  onRemove,
}: {
  definition: PMFilterDefinition<K>;
  selected: string[];
  onToggle: (value: string) => void;
  onRemove: () => void;
}) {
  return (
    <div className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-1 text-xs">
      <span className="font-medium text-muted-foreground">{definition.label}</span>
      <span className="text-muted-foreground/60">is</span>
      <PMFilterValueSelect definition={definition} selected={selected} onToggle={onToggle} />
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

export function PMFilterTrigger<K extends string>({
  definitions,
  values,
  visibleKeys,
  activeCount,
  onAdd,
  onToggle,
}: {
  definitions: PMFilterDefinition<K>[];
  values: PMFilterValues<K>;
  visibleKeys: Set<K>;
  activeCount: number;
  onAdd: (key: K) => void;
  onToggle: (key: K, value: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [selectedKey, setSelectedKey] = useState<K | null>(null);
  const available = definitions.filter((definition) => !visibleKeys.has(definition.key) && definition.options.length > 0);
  const selectedDefinition = selectedKey
    ? definitions.find((definition) => definition.key === selectedKey)
    : undefined;
  const canChooseFilter = available.length > 0 || Boolean(selectedDefinition);

  const handleOpenChange = (nextOpen: boolean) => {
    setOpen(nextOpen);
    if (!nextOpen) setSelectedKey(null);
  };

  if (!canChooseFilter) {
    return (
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

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="h-7 min-w-[88px] justify-between gap-2 px-2 text-xs text-muted-foreground">
          <span className="inline-flex items-center gap-1">
            <FilterHorizontalIcon className="h-3.5 w-3.5" />
            Filters
          </span>
          <Badge
            variant="secondary"
            className={`rounded-full px-1.5 py-0 text-[10px] transition-opacity ${activeCount > 0 ? 'opacity-100' : 'opacity-0'}`}
          >
            {activeCount || 0}
          </Badge>
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className={`${selectedDefinition ? (selectedDefinition.searchableValues ? 'w-80' : 'w-52') : 'w-48'} p-0`}
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
                  const isSelected = values[selectedDefinition.key]?.includes(option.value) ?? false;
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
                      <div className={`mr-2 flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border ${isSelected ? 'border-primary bg-primary text-primary-foreground' : 'border-muted-foreground/40'}`}>
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
  );
}

export function PMFilterBar<K extends string>({
  definitions,
  values,
  visibleKeys,
  onToggle,
  onRemove,
  onClearAll,
  trailing,
}: {
  definitions: PMFilterDefinition<K>[];
  values: PMFilterValues<K>;
  visibleKeys: Set<K>;
  onToggle: (key: K, value: string) => void;
  onRemove: (key: K) => void;
  onClearAll: () => void;
  trailing?: ReactNode;
}) {
  if (visibleKeys.size === 0) return null;

  return (
    <div className="ui-divider-bottom-fade flex flex-wrap items-center gap-1.5 px-3 py-1.5">
      {definitions
        .filter((definition) => visibleKeys.has(definition.key))
        .map((definition) => (
          <PMFilterPill
            key={definition.key}
            definition={definition}
            selected={values[definition.key] ?? []}
            onToggle={(value) => onToggle(definition.key, value)}
            onRemove={() => onRemove(definition.key)}
          />
        ))}
      <Button variant="ghost" size="sm" className="h-6 px-2 text-[10px] text-muted-foreground" onClick={onClearAll}>
        Clear all
      </Button>
      {trailing}
    </div>
  );
}
