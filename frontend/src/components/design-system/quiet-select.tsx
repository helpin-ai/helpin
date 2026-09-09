import { useId, useRef, useState, type ReactNode } from 'react';

import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ArrowDown01Icon, Cancel01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

const quietSelectTriggerClassName = 'min-w-0 max-w-full rounded-md border-0 bg-transparent text-quiet-text-secondary hover:bg-quiet-hover focus-visible:ring-2 focus-visible:ring-quiet-field focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50';

export interface QuietSelectOption {
  value: string;
  label: string;
  leading?: ReactNode;
  labelClassName?: string;
  disabled?: boolean;
}

export interface QuietSelectProps {
  label: string;
  value: string;
  options: QuietSelectOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
  id?: string;
}

/** Compact single-value form control. Use QuietFilterDropdown for searchable list filters. */
export function QuietSelect({ label, value, options, onChange, disabled, id }: QuietSelectProps) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled} size="ui">
      <SelectTrigger id={id} aria-label={label} variant="ghost" className={quietSelectTriggerClassName}>
        <SelectValue />
      </SelectTrigger>
      <SelectContent>
        {options.map((option) => (
          <SelectItem key={option.value} value={option.value} disabled={option.disabled}>
            {option.leading}
            <span className={cn('truncate', option.labelClassName)}>{option.label}</span>
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}

interface QuietFilterDropdownBaseProps {
  label: string;
  options: QuietSelectOption[];
  icon?: ReactNode;
  disabled?: boolean;
  id?: string;
  /** Keep a visible category label above the trigger in dense filter bars. */
  showLabel?: boolean;
  /** Context shown when no values are selected, e.g. "All states". */
  emptyLabel?: string;
  /** Search matches option labels; enabled by default in both selection modes. */
  searchable?: boolean;
  popoverClassName?: string;
}

export type QuietFilterDropdownProps = QuietFilterDropdownBaseProps & (
  | { multiple?: false; value: string; onChange: (value: string) => void }
  | { multiple: true; selected: string[]; onChange: (values: string[]) => void }
);

/**
 * Shared list-filter picker. Callers own values, defaults, URL state and filtering.
 * Single selections close the menu; multiple selections toggle and keep it open.
 */
export function QuietFilterDropdown(props: QuietFilterDropdownProps) {
  const { label, options, icon, disabled = false, id, showLabel = false, emptyLabel = 'All', searchable = true, popoverClassName } = props;
  const generatedId = useId();
  const triggerId = id ?? generatedId;
  const triggerRef = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const selected = props.multiple ? props.selected : [props.value];
  const summary = selected.length === 0
    ? emptyLabel
    : selected.length === 1
      ? (options.find((option) => option.value === selected[0])?.label ?? selected[0]) || emptyLabel
      : `${selected.length} selected`;

  const select = (value: string) => {
    if (props.multiple) {
      props.onChange(selected.includes(value) ? selected.filter((entry) => entry !== value) : [...selected, value]);
    } else {
      props.onChange(value);
      setOpen(false);
    }
  };

  return (
    <div className="flex min-w-0 max-w-full flex-col gap-0.5">
      {showLabel && <label htmlFor={triggerId} className="text-xs font-medium text-quiet-text-tertiary">{label}</label>}
      <div className="flex min-w-0 items-center">
        <Popover open={open && !disabled} onOpenChange={setOpen}>
          <PopoverTrigger asChild>
            <button
              ref={triggerRef}
              id={triggerId}
              type="button"
              disabled={disabled}
              aria-label={`${label}: ${summary}`}
              className={cn(quietSelectTriggerClassName, 'inline-flex h-8 items-center justify-between gap-1.5 px-2 text-ui')}
            >
              {icon && <span className="shrink-0" aria-hidden="true">{icon}</span>}
              <span className="truncate">{props.multiple && selected.length > 1 && !showLabel ? `${label}: ` : ''}{summary}</span>
              <ArrowDown01Icon aria-hidden="true" className="size-3.5 shrink-0 text-quiet-text-tertiary" />
            </button>
          </PopoverTrigger>
          <PopoverContent align="start" collisionPadding={8} aria-label={`${label} options`} className={cn('w-60 max-w-[calc(100vw-2rem)] p-0', popoverClassName)}>
            <Command
              label={`${label} options`}
              tabIndex={searchable ? undefined : 0}
              filter={(_value, search, keywords) => keywords?.some((keyword) => keyword.toLowerCase().includes(search.trim().toLowerCase())) ? 1 : 0}
            >
              {searchable && <CommandInput presentation="quiet" aria-label={`Search ${label.toLowerCase()}`} placeholder={`Search ${label.toLowerCase()}…`} />}
              <CommandList>
                <CommandEmpty>No results</CommandEmpty>
                <CommandGroup>
                  {options.map((option) => {
                    const checked = selected.includes(option.value);
                    return (
                      <CommandItem
                        key={option.value}
                        value={option.value}
                        keywords={[option.label]}
                        disabled={option.disabled}
                        data-checked={checked}
                        aria-label={`${option.label}${checked ? ', selected' : ''}`}
                        onSelect={() => select(option.value)}
                      >
                        {option.leading && <span className="flex shrink-0 items-center" aria-hidden="true">{option.leading}</span>}
                        <span className={cn('min-w-0 flex-1 truncate', option.labelClassName)}>{option.label}</span>
                      </CommandItem>
                    );
                  })}
                </CommandGroup>
              </CommandList>
            </Command>
          </PopoverContent>
        </Popover>
        {props.multiple && selected.length > 0 && (
          <button
            type="button"
            disabled={disabled}
            aria-label={`Clear ${label} filter`}
            className={cn(quietSelectTriggerClassName, 'inline-flex size-8 shrink-0 items-center justify-center')}
            onClick={() => { props.onChange([]); triggerRef.current?.focus(); }}
          >
            <Cancel01Icon aria-hidden="true" className="size-3.5" />
          </button>
        )}
      </div>
    </div>
  );
}
