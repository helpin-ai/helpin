import {
  useState,
  type ComponentProps,
  type ReactElement,
  type ReactNode,
} from 'react';
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
  CommandSeparator,
} from '@/components/ui/command';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import { cn } from '@/lib/utils';

export type QuietDropdownSearchMode = 'auto' | 'always' | 'off';
export interface QuietDropdownOption {
  value: string;
  label: string;
  keywords?: string[];
  leading?: ReactNode;
  content?: ReactNode;
  trailing?: ReactNode;
  disabled?: boolean;
  partial?: boolean;
  className?: string;
}
export interface QuietDropdownOptionGroup {
  id: string;
  label?: ReactNode;
  options: QuietDropdownOption[];
}

/** Compound hosts for pickers with additional domain controls (e.g. filter navigation). */
export const QuietDropdownRoot = Popover;
export const QuietDropdownTrigger = PopoverTrigger;
export const QuietDropdownGroup = CommandGroup;
export const QuietDropdownEmpty = CommandEmpty;
export const QuietDropdownSeparator = CommandSeparator;

export function QuietDropdownContent({
  className,
  ...props
}: ComponentProps<typeof PopoverContent>) {
  return (
    <PopoverContent
      data-dropdown-content=""
      collisionPadding={8}
      align="start"
      className={cn(
        'flex flex-col w-60 max-w-[calc(100vw-2rem)] max-h-[var(--radix-popover-content-available-height)] overflow-hidden p-0',
        className,
      )}
      {...props}
    />
  );
}

export function QuietDropdownItem({
  value,
  keywords,
  children,
  fullText,
  onPointerEnter,
  ...props
}: ComponentProps<typeof CommandItem> & { value: string; fullText?: string }) {
  return (
    <CommandItem value={value} keywords={keywords} {...props}
      onPointerEnter={(event) => {
        onPointerEnter?.(event);
        const item = event.currentTarget;
        // Measure on hover so resizing and custom option content are covered without
        // an observer or a state update for every item in a large dropdown.
        const truncated = [item, ...item.querySelectorAll<HTMLElement>('*')].some(
          (element) => element.clientWidth > 0 && Boolean(element.textContent?.trim()) &&
            (element.scrollWidth > element.clientWidth || element.scrollHeight > element.clientHeight),
        );
        if (truncated) item.title = fullText ?? item.textContent?.trim() ?? '';
        else item.removeAttribute('title');
      }}
    >
      {children}
    </CommandItem>
  );
}

type OptionsProps = ComponentProps<typeof Command> & {
  searchMode?: QuietDropdownSearchMode;
  searchPlaceholder?: string;
  searchLabel?: string;
  query?: string;
  onQueryChange?: (query: string) => void;
  listClassName?: string;
  listProps?: ComponentProps<typeof CommandList>;
  inputProps?: ComponentProps<typeof CommandInput>;
  header?: ReactNode;
  footer?: ReactNode;
};

/** One search, scrolling and keyboard implementation for every option-selection surface. */
export function QuietDropdownOptions({
  searchMode = 'auto',
  searchPlaceholder = 'Search...',
  searchLabel,
  query,
  onQueryChange,
  listClassName,
  listProps,
  inputProps,
  header,
  footer,
  children,
  className,
  ...props
}: OptionsProps) {
  return (
    <Command
      tabIndex={searchMode === 'off' ? 0 : undefined}
      className={cn('min-h-0', className)}
      {...props}
    >
      {header}
      {searchMode !== 'off' && (
        <CommandInput
          presentation="quiet"
          placeholder={searchPlaceholder}
          aria-label={searchLabel ?? searchPlaceholder}
          value={query}
          onValueChange={onQueryChange}
          {...inputProps}
          hideWhenFits={searchMode === 'auto'}
        />
      )}
      <CommandList
        {...listProps}
        className={cn('min-h-0', listProps?.className, listClassName)}
      >
        {children}
      </CommandList>
      {footer}
    </Command>
  );
}

export interface QuietDropdownProps {
  label: string;
  trigger: ReactElement;
  triggerWrapper?: (trigger: ReactElement) => ReactNode;
  options?: QuietDropdownOption[];
  groups?: QuietDropdownOptionGroup[];
  selected?: string[];
  onSelect: (value: string) => void;
  multiple?: boolean;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  disabled?: boolean;
  searchMode?: QuietDropdownSearchMode;
  searchPlaceholder?: string;
  query?: string;
  onQueryChange?: (query: string) => void;
  empty?: ReactNode;
  footer?: ReactNode;
  contentClassName?: string;
  contentProps?: ComponentProps<typeof QuietDropdownContent>;
  listClassName?: string;
  loading?: boolean;
  error?: ReactNode;
}

/** Domain adapters own values and saves; this component owns the dropdown interaction. */
export function QuietDropdown({
  label,
  trigger,
  triggerWrapper,
  options = [],
  groups,
  selected = [],
  onSelect,
  multiple = false,
  open: controlledOpen,
  onOpenChange,
  disabled = false,
  searchMode = 'auto',
  searchPlaceholder = `Search ${label.toLowerCase()}...`,
  query,
  onQueryChange,
  empty = 'No results',
  footer,
  contentClassName,
  contentProps,
  listClassName,
  loading,
  error,
}: QuietDropdownProps) {
  const [localOpen, setLocalOpen] = useState(false);
  const open = controlledOpen ?? localOpen;
  const setOpen = (next: boolean) => {
    if (disabled && next) return;
    if (controlledOpen === undefined) setLocalOpen(next);
    onOpenChange?.(next);
  };
  const resolvedGroups = groups ?? [{ id: 'options', options }];
  return (
    <QuietDropdownRoot open={open && !disabled} onOpenChange={setOpen}>
      {triggerWrapper ? (
        triggerWrapper(
          <QuietDropdownTrigger asChild>{trigger}</QuietDropdownTrigger>,
        )
      ) : (
        <QuietDropdownTrigger asChild>{trigger}</QuietDropdownTrigger>
      )}
      <QuietDropdownContent
        aria-label={`${label} options`}
        className={contentClassName}
        onClick={(event) => event.stopPropagation()}
        onKeyDown={(event) => event.stopPropagation()}
        {...contentProps}
      >
        <QuietDropdownOptions
          label={`${label} options`}
          defaultValue={selected[0] === '' ? '__quiet_empty__' : selected[0]}
          searchMode={searchMode}
          searchPlaceholder={searchPlaceholder}
          query={query}
          onQueryChange={onQueryChange}
          listClassName={listClassName}
          filter={(_value, search, keywords) =>
            keywords?.some((keyword) =>
              keyword.toLowerCase().includes(search.trim().toLowerCase()),
            )
              ? 1
              : 0
          }
        >
          {loading ? (
            <div role="status" className="px-3 py-4 text-muted-foreground">
              Loading...
            </div>
          ) : null}
          {error ? (
            <div role="alert" className="px-3 py-4">
              {error}
            </div>
          ) : null}
          {!loading && !error && (
            <QuietDropdownEmpty className="px-3 py-3 text-center">
              {empty}
            </QuietDropdownEmpty>
          )}
          {resolvedGroups
            .filter((group) => group.options.length > 0)
            .map((group) => (
              <QuietDropdownGroup key={group.id} heading={group.label}>
                {group.options.map((option) => {
                  const checked = selected.includes(option.value);
                  return (
                    <QuietDropdownItem
                      key={option.value}
                      fullText={option.label}
                      value={option.value || '__quiet_empty__'}
                      keywords={[
                        option.label,
                        ...(typeof group.label === 'string'
                          ? [group.label]
                          : []),
                        ...(option.keywords ?? []),
                      ]}
                      disabled={loading || Boolean(error) || option.disabled}
                      data-checked={checked}
                      data-partial={option.partial || undefined}
                      aria-label={`${option.label}${checked ? ', selected' : option.partial ? ', partially selected' : ''}`}
                      className={cn(
                        option.partial && 'italic text-muted-foreground',
                        option.className,
                      )}
                      onSelect={() => {
                        onSelect(option.value);
                        if (!multiple) setOpen(false);
                      }}
                    >
                      {option.leading && (
                        <span
                          className="flex shrink-0 items-center"
                          aria-hidden="true"
                        >
                          {option.leading}
                        </span>
                      )}
                      {option.content ?? (
                        <span className="min-w-0 flex-1 truncate">
                          {option.label}
                        </span>
                      )}
                      {option.trailing}
                    </QuietDropdownItem>
                  );
                })}
              </QuietDropdownGroup>
            ))}
        </QuietDropdownOptions>
        {footer && (
          <div className="shrink-0 border-t border-border/60 p-1">{footer}</div>
        )}
      </QuietDropdownContent>
    </QuietDropdownRoot>
  );
}
