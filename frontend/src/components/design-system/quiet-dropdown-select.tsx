import {
  Children,
  Fragment,
  createContext,
  isValidElement,
  useContext,
  useState,
  type ComponentProps,
  type ReactNode,
} from 'react';
import type * as Legacy from '@/components/ui/select';
import { QuietDropdown, type QuietDropdownOptionGroup } from './quiet-dropdown';
import { pickerTriggerVariants } from '@/components/ui/picker-trigger';
import { ArrowDown01Icon } from '@/lib/icons';
import { cn } from '@/lib/utils';

type Size = 'sm' | 'ui' | 'default';
const Context = createContext<{
  selected?: ReactNode;
  value?: string;
  disabled?: boolean;
  size: Size;
  setOpen?: (open: boolean) => void;
}>({ size: 'default' });

function text(node: ReactNode): string {
  return Children.toArray(node)
    .map((child) =>
      typeof child === 'string' || typeof child === 'number'
        ? String(child)
        : isValidElement<{ children?: ReactNode }>(child)
          ? text(child.props.children)
          : '',
    )
    .join(' ');
}

/** Declarative Select syntax adapter; all options render through QuietDropdown. */
export function Select(props: ComponentProps<typeof Legacy.Select>) {
  const {
    children,
    value: controlledValue,
    defaultValue,
    onValueChange,
    size = 'default',
    disabled,
    open,
    defaultOpen,
    onOpenChange,
    name,
    required,
  } = props;
  // An explicit undefined value represents a mixed/unchanged field in bulk edit.
  const controlled = Object.hasOwn(props, 'value');
  const [localValue, setLocalValue] = useState(defaultValue);
  const [localOpen, setLocalOpen] = useState(defaultOpen ?? false);
  const value = controlled ? controlledValue : localValue;
  const setOpen = (next: boolean) => {
    setLocalOpen(next);
    onOpenChange?.(next);
  };
  const nodes = Children.toArray(children);
  const trigger = nodes.find(
    (child) => isValidElement(child) && child.type === SelectTrigger,
  );
  const content = nodes.find(
    (child) => isValidElement(child) && child.type === SelectContent,
  );
  if (
    !isValidElement<ComponentProps<typeof SelectTrigger>>(trigger) ||
    !isValidElement<ComponentProps<typeof SelectContent>>(content)
  )
    return null;
  const groups: QuietDropdownOptionGroup[] = [];
  let current: QuietDropdownOptionGroup = { id: '0', options: [] };
  groups.push(current);
  let selected: ReactNode;
  const collect = (items: ReactNode) =>
    Children.forEach(items, (child) => {
      if (
        !isValidElement<{
          children?: ReactNode;
          value?: string;
          textValue?: string;
          disabled?: boolean;
          className?: string;
        }>(child)
      )
        return;
      if (child.type === SelectItem && child.props.value !== undefined) {
        const option = {
          value: child.props.value,
          label: child.props.textValue ?? text(child.props.children),
          content: (
            <span className="flex min-w-0 flex-1 items-center gap-2">
              {child.props.children}
            </span>
          ),
          disabled: child.props.disabled,
          className: child.props.className,
        };
        current.options.push(option);
        if (option.value === value) selected = child.props.children;
      } else if (child.type === SelectLabel) {
        current.label = child.props.children;
      } else if (child.type === SelectGroup || child.type === SelectSeparator) {
        current = { id: String(groups.length), options: [] };
        groups.push(current);
        if (child.type === SelectGroup) collect(child.props.children);
        current = { id: String(groups.length), options: [] };
        groups.push(current);
      } else if (child.type === Fragment) collect(child.props.children);
    });
  collect(content.props.children);
  const change = (next: string) => {
    if (!controlled) setLocalValue(next);
    onValueChange?.(next);
  };
  // Portaled content keeps ownership markers supplied by its host surface.
  const contentDataAttributes = Object.fromEntries(
    Object.entries(content.props).filter(([key]) => key.startsWith('data-')),
  );
  return (
    <Context.Provider value={{ selected, value, disabled, size, setOpen }}>
      <QuietDropdown
        label={trigger.props['aria-label'] ?? 'Options'}
        trigger={trigger}
        groups={groups}
        selected={value === undefined ? [] : [value]}
        onSelect={change}
        disabled={disabled}
        open={open ?? localOpen}
        onOpenChange={setOpen}
        contentClassName={cn(
          'min-w-[var(--radix-popover-trigger-width)]',
          content.props.className,
        )}
        contentProps={{
          ...contentDataAttributes,
          align: content.props.align ?? 'start',
          side: content.props.side,
          sideOffset: content.props.sideOffset,
          onCloseAutoFocus: content.props.onCloseAutoFocus,
          onEscapeKeyDown: content.props.onEscapeKeyDown,
          onPointerDownOutside: content.props.onPointerDownOutside,
        }}
      />
      {name && (
        <input
          type="hidden"
          name={name}
          value={value ?? ''}
          disabled={disabled}
          required={required}
        />
      )}
    </Context.Provider>
  );
}

export function SelectTrigger({
  children,
  className,
  size: explicitSize,
  variant = 'ghost',
  onKeyDown,
  ...props
}: ComponentProps<typeof Legacy.SelectTrigger>) {
  const context = useContext(Context);
  const size = explicitSize ?? context.size;
  return (
    <button
      type="button"
      role="combobox"
      disabled={context.disabled}
      data-size={size}
      data-variant={variant}
      className={cn(
        'flex min-w-0 items-center justify-between gap-1.5 whitespace-nowrap text-ui [&_svg]:shrink-0',
        pickerTriggerVariants({ variant }),
        size === 'sm' ? 'h-7 px-2' : size === 'ui' ? 'h-8 px-2' : 'h-9 px-3',
        className,
      )}
      {...props}
      data-slot="select-trigger"
      data-placeholder={context.selected === undefined ? '' : undefined}
      onKeyDown={(event) => {
        onKeyDown?.(event);
        if (
          !event.defaultPrevented &&
          (event.key === 'ArrowDown' || event.key === 'ArrowUp')
        ) {
          event.preventDefault();
          context.setOpen?.(true);
        }
      }}
    >
      {children}
      <ArrowDown01Icon
        aria-hidden="true"
        className="size-3.5 shrink-0 text-muted-foreground"
      />
    </button>
  );
}
export function SelectValue({
  placeholder,
  children,
  ...props
}: ComponentProps<typeof Legacy.SelectValue>) {
  const { selected } = useContext(Context);
  return (
    <span
      data-slot="select-value"
      className="flex min-w-0 items-center gap-1.5 truncate"
      {...props}
    >
      {children ?? selected ?? placeholder}
    </span>
  );
}
// These elements describe options; Select extracts their data instead of mounting a second menu.
export function SelectContent({
  children,
}: ComponentProps<typeof Legacy.SelectContent>) {
  return <>{children}</>;
}
export function SelectItem({
  children,
}: ComponentProps<typeof Legacy.SelectItem>) {
  return <>{children}</>;
}
export function SelectGroup({
  children,
}: ComponentProps<typeof Legacy.SelectGroup>) {
  return <>{children}</>;
}
export function SelectLabel({
  children,
}: ComponentProps<typeof Legacy.SelectLabel>) {
  return <>{children}</>;
}
export function SelectSeparator(
  props: ComponentProps<typeof Legacy.SelectSeparator>,
) {
  void props; // Marker props are consumed by Select while collecting groups.
  return null;
}
