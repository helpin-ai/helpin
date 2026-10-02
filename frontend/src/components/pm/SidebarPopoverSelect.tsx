import { useState } from 'react';
import { ArrowDown01Icon } from '@/lib/icons';
import { QuietDropdown } from '@/components/design-system/quiet-dropdown';
import { cn } from '@/lib/utils';
import {
  pickerTriggerVariants,
  type PickerTriggerVariant,
} from '@/components/ui/picker-trigger';

interface SidebarPopoverSelectOption<T extends string = string> {
  value: T;
  label: string;
  className?: string;
  icon?: React.ReactNode;
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
  /** Popover width class (default: "w-52") */
  width?: string;
  /** Placeholder text for search input */
  searchPlaceholder?: string;
  /** Disable interaction */
  disabled?: boolean;
  /** Called when the popover opens or closes. */
  onOpenChange?: (open: boolean) => void;
  /** Content to show when there are no options. */
  emptyContent?: React.ReactNode;
  /** Optional trigger button class override */
  triggerClassName?: string;
  triggerLabel?: string;
  /** Optional visual treatment for the trigger. Omit to preserve the existing compact style. */
  triggerVariant?: PickerTriggerVariant;
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
  width = 'w-52',
  searchPlaceholder = 'Search...',
  disabled = false,
  onOpenChange,
  emptyContent,
  triggerClassName,
  triggerLabel,
  triggerVariant,
  showChevron = false,
}: SidebarPopoverSelectProps<T>) {
  const [open, setOpen] = useState(false);
  const updateOpen = (next: boolean) => {
    setOpen(next);
    onOpenChange?.(next);
  };
  return (
    <QuietDropdown
      label={searchPlaceholder.replace(/^Search\s*|\.\.\.$/g, '') || 'Options'}
      open={open}
      onOpenChange={updateOpen}
      disabled={disabled}
      selected={[value]}
      onSelect={(next) => onChange(next as T)}
      searchPlaceholder={searchPlaceholder}
      contentClassName={width}
      empty={emptyContent ?? 'No results'}
      groups={(groups ?? [{ options: options ?? [] }]).map((group, index) => ({
        id: String(index),
        label: group.label,
        options: group.options.map((option) => ({
          value: option.value,
          label: option.label,
          leading: option.icon,
          content: renderOption?.(option.value),
          className: option.className,
        })),
      }))}
      trigger={
        <button
          type="button"
          aria-label={triggerLabel}
          disabled={disabled}
          onClick={(event) => event.stopPropagation()}
          className={cn(
            'inline-flex min-w-0 max-w-full items-center gap-1.5 px-1.5 py-0.5 text-ui text-foreground cursor-pointer disabled:pointer-events-none',
            triggerVariant
              ? pickerTriggerVariants({ variant: triggerVariant })
              : 'rounded-md transition-colors hover:bg-accent disabled:opacity-50',
            triggerClassName,
          )}
        >
          {(groups?.flatMap(group => group.options) ?? options)?.find(option => option.value === value)?.icon}
          {renderTrigger()}
          {showChevron && (
            <ArrowDown01Icon className="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          )}
        </button>
      }
    />
  );
}
