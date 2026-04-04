import { useState } from 'react';
import { Tick01Icon } from '@/lib/icons';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { cn } from '@/lib/utils';

interface SidebarPopoverSelectOption<T extends string = string> {
  value: T;
  label: string;
  className?: string;
}

export interface SidebarPopoverSelectProps<T extends string = string> {
  value: T;
  options: SidebarPopoverSelectOption<T>[];
  onChange: (value: T) => void;
  renderTrigger: () => React.ReactNode;
  /** Custom renderer for each option. Receives the option value. Falls back to the label. */
  renderOption?: (value: T) => React.ReactNode;
  /** Show search input when options exceed this count (default: 8) */
  searchThreshold?: number;
  /** Popover width class (default: "w-52") */
  width?: string;
  /** Placeholder text for search input */
  searchPlaceholder?: string;
}

export function SidebarPopoverSelect<T extends string>({
  value,
  options,
  onChange,
  renderTrigger,
  renderOption,
  searchThreshold = 8,
  width = 'w-52',
  searchPlaceholder = 'Search...',
}: SidebarPopoverSelectProps<T>) {
  const [open, setOpen] = useState(false);
  const showSearch = options.length > searchThreshold;

  if (showSearch) {
    return (
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            className="inline-flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs text-foreground transition-colors hover:bg-accent cursor-pointer"
          >
            {renderTrigger()}
          </button>
        </PopoverTrigger>
        <PopoverContent className={cn(width, 'p-0')} align="start">
          <Command>
            <CommandInput placeholder={searchPlaceholder} className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">No results</CommandEmpty>
              <CommandGroup>
                {options.map((option) => (
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
          className="inline-flex min-w-0 max-w-full items-center gap-1.5 rounded-md px-1.5 py-0.5 text-xs text-foreground transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className={cn(width, 'p-0.5')} align="start">
        <div className="flex max-h-60 flex-col overflow-y-auto">
          {options.map((option) => (
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
      </PopoverContent>
    </Popover>
  );
}
