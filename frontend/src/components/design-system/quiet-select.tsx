import { useId, useRef, useState, type ReactNode } from 'react';

import { QuietDropdown } from './quiet-dropdown';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './quiet-dropdown-select';
import { ArrowDown01Icon, Cancel01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

const quietSelectTriggerClassName = 'min-w-0 max-w-full rounded-md border-0 bg-transparent text-quiet-text-secondary hover:bg-quiet-hover focus-visible:ring-2 focus-visible:ring-quiet-field focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50';

export interface QuietSelectOption {
  value: string;
  label: string;
  leading?: ReactNode;
  labelClassName?: string;
  tooltip?: string;
  disabled?: boolean;
}

export interface QuietSelectProps {
  label: string;
  placeholder?: string;
  value: string;
  options: QuietSelectOption[];
  onChange: (value: string) => void;
  disabled?: boolean;
  id?: string;
}

/** Compact single-value form control. Use QuietFilterDropdown for searchable list filters. */
export function QuietSelect({ label, placeholder, value, options, onChange, disabled, id }: QuietSelectProps) {
  return (
    <Select value={value} onValueChange={onChange} disabled={disabled} size="ui">
      <SelectTrigger id={id} aria-label={label} variant="ghost" className={quietSelectTriggerClassName}>
        <SelectValue placeholder={placeholder} />
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
  /** Show the category above the trigger, or inline before the selected value. */
  showLabel?: boolean | 'inline';
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
      {showLabel === true && <label htmlFor={triggerId} className="text-xs font-medium text-quiet-text-tertiary">{label}</label>}
      <div className="flex min-w-0 items-center">
        <QuietDropdown label={label} open={open} onOpenChange={setOpen} disabled={disabled}
          selected={selected} options={options.map(option => ({ ...option, className: option.labelClassName }))}
          multiple={props.multiple} onSelect={select} searchMode={searchable ? 'auto' : 'off'}
          searchPlaceholder={`Search ${label.toLowerCase()}…`} contentClassName={popoverClassName}
          trigger={(
            <button
              ref={triggerRef}
              id={triggerId}
              type="button"
              disabled={disabled}
              aria-label={`${label}: ${summary}`}
              className={cn(quietSelectTriggerClassName, 'inline-flex h-8 items-center justify-between gap-1.5 px-2 text-ui')}
            >
              {icon && <span className="shrink-0" aria-hidden="true">{icon}</span>}
              {showLabel === 'inline' && <span className="shrink-0 text-quiet-text-tertiary">{label}:</span>}
              <span className="truncate">{props.multiple && selected.length > 1 && !showLabel ? `${label}: ` : ''}{summary}</span>
              <ArrowDown01Icon aria-hidden="true" className="size-3.5 shrink-0 text-quiet-text-tertiary" />
            </button>
          )} />
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
