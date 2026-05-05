import { useState } from 'react';
import { ArrowDown01Icon, Tick01Icon } from '@/lib/icons';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

interface SidebarPopoverSelectOption<T extends string = string> {
  value: T;
  label: string;
  className?: string;
}

export interface SidebarPopoverSelectGroup<T extends string = string> {
  /** Optional heading rendered above the group's options. Omit for un-headed groups (e.g. "None" at the top). */
  label?: string;
  options: SidebarPopoverSelectOption<T>[];
}

export interface SidebarPopoverSelectProps<T extends string = string> {
  value: T;
  /** Flat list of options. Mutually exclusive with `groups`. */
  options?: SidebarPopoverSelectOption<T>[];
  /** Grouped options with optional headings. Takes precedence over `options`. */
  groups?: SidebarPopoverSelectGroup<T>[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
  /** Custom renderer for each option. Receives the option value. Falls back to the label. */
  renderOption?: (value: T) => React.ReactNode;
  /** Show search input when total option count exceeds this (default: 8) */
  searchThreshold?: number;
  /** Popover width class (default: "w-52") */
  width?: string;
  /** Placeholder text for search input */
  searchPlaceholder?: string;
  /** Disable interaction */
  disabled?: boolean;
  /** Optional trigger button class override */
  triggerClassName?: string;
  /** Show a chevron icon on the trigger */
  showChevron?: boolean;
}

export function SidebarPopoverSelect<T extends string>({
  value,
  options,
  groups,
  onChange,
  renderTrigger,
  renderOption,
  searchThreshold = 8,
  width = 'w-52',
  searchPlaceholder = 'Search...',
  disabled = false,
  triggerClassName,
  showChevron = false,
}: SidebarPopoverSelectProps<T>) {
  const [open, setOpen] = useState(false);
  const resolvedGroups: SidebarPopoverSelectGroup<T>[] = groups ?? [{ options: options ?? [] }];
  const totalOptionCount = resolvedGroups.reduce((sum, group) => sum + group.options.length, 0);
  const showSearch = totalOptionCount > searchThreshold;

  if (showSearch) {
    return (
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            className={cn(
              'inline-flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-[12px] text-foreground transition-colors hover:bg-accent cursor-pointer disabled:pointer-events-none disabled:opacity-50',
              triggerClassName,
            )}
          >
            {renderTrigger()}
            {showChevron && <ArrowDown01Icon className="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground" />}
          </button>
        </PopoverTrigger>
        <PopoverContent className={cn(width, 'p-0')} align="start">
          <Command
            filter={(optionValue, search) => {
              const needle = search.trim().toLowerCase();
              if (!needle) return 1;
              const haystack = optionValue.toLowerCase();
              if (haystack.startsWith(needle)) return 2;
              if (haystack.includes(needle)) return 1;
              return 0;
            }}
          >
            <CommandInput placeholder={searchPlaceholder} className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No results</CommandEmpty>
              {resolvedGroups.map((group, groupIndex) => (
                group.options.length === 0 ? null : (
                  <CommandGroup key={groupIndex} heading={group.label}>
                    {group.options.map((option) => (
                      <CommandItem
                        key={option.value}
                        value={option.label}
                        onSelect={() => { onChange(option.value); setOpen(false); }}
                        className="flex items-center gap-2 text-xs"
                      >
                        {renderOption ? renderOption(option.value) : <span className={cn('truncate', option.className)}>{option.label}</span>}
                        {value === option.value && <Tick01Icon className="ml-auto h-3.5 w-3.5 shrink-0 text-primary" />}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                )
              ))}
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    );
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className={cn(
            'inline-flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-[12px] text-foreground transition-colors hover:bg-accent cursor-pointer disabled:pointer-events-none disabled:opacity-50',
            triggerClassName,
          )}
        >
          {renderTrigger()}
          {showChevron && <ArrowDown01Icon className="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground" />}
        </button>
      </PopoverTrigger>
      <PopoverContent className={cn(width, 'p-0.5')} align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {resolvedGroups.map((group, groupIndex) => (
            group.options.length === 0 ? null : (
              <div key={groupIndex} className="flex flex-col">
                {group.label && (
                  <div className="px-2 pt-1.5 pb-0.5 text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
                    {group.label}
                  </div>
                )}
                {group.options.map((option) => (
                  <button
                    key={option.value}
                    type="button"
                    className={cn(
                      'flex items-center gap-1.5 rounded-sm px-2 py-1.5 text-xs text-foreground transition-colors cursor-pointer',
                      value === option.value
                        ? 'bg-accent font-medium'
                        : 'hover:bg-accent',
                    )}
                    onClick={() => { onChange(option.value); setOpen(false); }}
                  >
                    {renderOption ? renderOption(option.value) : <span className={cn('truncate', option.className)}>{option.label}</span>}
                  </button>
                ))}
              </div>
            )
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}
