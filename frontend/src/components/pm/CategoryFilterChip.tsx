import { useState, type ReactNode } from 'react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command';
import { ArrowDown01Icon, Cancel01Icon, Tick01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

export interface FilterChipOption {
  value: string;
  label: string;
}

interface CategoryFilterChipProps {
  label: string;
  options: FilterChipOption[];
  selected: string[];
  onChange: (next: string[]) => void;
  icon?: ReactNode;
  /** Disables the chip; renders as a non-interactive label. */
  disabled?: boolean;
  /** Override popover width — defaults to 240px. */
  popoverClassName?: string;
}

/**
 * A single filter category chip for use in a horizontal filter bar.
 *
 * Renders as `[icon] Label ▾ [active-pill ×]` (one chip per category).
 * Clicking the trigger opens a multi-select popover. The active selection
 * appears inline so the user can see what's filtered without scrolling
 * a separate row of pills.
 */
export function CategoryFilterChip({
  label,
  options,
  selected,
  onChange,
  icon,
  disabled = false,
  popoverClassName = 'w-60 p-0',
}: CategoryFilterChipProps) {
  const [open, setOpen] = useState(false);

  const selectedLabels = selected
    .map((value) => options.find((option) => option.value === value)?.label ?? value)
    .filter(Boolean);

  const summary = selected.length === 0
    ? 'All'
    : selected.length === 1
      ? selectedLabels[0]
      : `${selected.length} selected`;

  const toggle = (value: string) => {
    if (selected.includes(value)) onChange(selected.filter((v) => v !== value));
    else onChange([...selected, value]);
  };

  const clear = (event: React.MouseEvent) => {
    event.stopPropagation();
    onChange([]);
  };

  return (
    <div className="flex flex-col gap-0.5">
      <span className="text-[10px] font-medium uppercase tracking-wide text-muted-foreground">
        {label}
      </span>
      <Popover open={open} onOpenChange={disabled ? undefined : setOpen}>
        <PopoverTrigger asChild>
          <button
            type="button"
            disabled={disabled}
            className={cn(
              'inline-flex h-7 items-center gap-1 rounded-md border border-input bg-transparent px-2 text-xs transition-colors',
              disabled
                ? 'cursor-not-allowed opacity-60'
                : 'hover:bg-accent',
              selected.length > 0 && 'border-primary/40 bg-primary/5',
            )}
          >
            {icon ? <span className="text-muted-foreground">{icon}</span> : null}
            <span className={cn(selected.length > 0 ? 'text-foreground' : 'text-muted-foreground')}>
              {summary}
            </span>
            {selected.length > 0 ? (
              <span
                role="button"
                tabIndex={-1}
                onClick={clear}
                className="ml-1 inline-flex h-4 w-4 items-center justify-center rounded-sm text-muted-foreground/70 hover:bg-accent hover:text-foreground"
                aria-label={`Clear ${label} filter`}
              >
                <Cancel01Icon className="h-3 w-3" />
              </span>
            ) : (
              <ArrowDown01Icon className="h-3 w-3 opacity-60" />
            )}
          </button>
        </PopoverTrigger>
        <PopoverContent align="start" className={popoverClassName}>
          <Command>
            <CommandInput placeholder={`Search ${label.toLowerCase()}…`} className="h-8 text-xs" />
            <CommandList>
              <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">
                No results
              </CommandEmpty>
              <CommandGroup>
                {options.map((option) => {
                  const isSelected = selected.includes(option.value);
                  return (
                    <CommandItem
                      key={option.value}
                      value={option.label}
                      className="flex items-center gap-2 text-xs"
                      onSelect={() => toggle(option.value)}
                    >
                      <div
                        className={cn(
                          'flex h-4 w-4 items-center justify-center rounded-sm border',
                          isSelected
                            ? 'border-primary bg-primary text-primary-foreground'
                            : 'border-muted-foreground/40',
                        )}
                      >
                        {isSelected ? <Tick01Icon className="h-3 w-3" /> : null}
                      </div>
                      <span className="truncate">{option.label}</span>
                    </CommandItem>
                  );
                })}
              </CommandGroup>
            </CommandList>
          </Command>
        </PopoverContent>
      </Popover>
    </div>
  );
}
