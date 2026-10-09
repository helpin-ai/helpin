import { useQuietDropdownFocusReturn } from '@/components/design-system/use-quiet-dropdown-focus-return';
import { useMemo, useState } from 'react';
import type { ReactElement, ReactNode } from 'react';

import {
  QuietDropdownOptions,
  QuietDropdownEmpty,
  QuietDropdownGroup,
  QuietDropdownItem,
} from '@/components/design-system/quiet-dropdown';
import {
  QuietDropdownRoot as Popover,
  QuietDropdownTrigger as PopoverTrigger,
} from '@/components/design-system/quiet-dropdown';
import { PMDropdownContent } from './PMDropdownContent';
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip';
import { UserAvatar } from '@/components/pm/UserAvatar';
import {
  formatAssignableMemberName,
  matchesAssignableMemberValue,
} from '@/lib/assignableMembers';
import { cn } from '@/lib/utils';
import type { AssignableMember } from '@/lib/types';

type PopoverAlign = 'start' | 'center' | 'end';
type MemberValueGetter = (member: AssignableMember) => string;

interface BaseMemberPickerProps {
  members: AssignableMember[];
  renderTrigger: () => ReactNode;
  triggerClassName?: string;
  triggerLabel?: string;
  contentClassName?: string;
  align?: PopoverAlign;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  getMemberValue?: MemberValueGetter;
  disabled?: boolean;
  lazyMount?: boolean;
}

interface MemberPickerPopoverProps extends BaseMemberPickerProps {
  getDisabledReason?: (member: AssignableMember) => string | null | undefined;
  value: string;
  onChange: (memberId: string) => void;
  noneLabel?: string;
}

interface MultiMemberPickerPopoverProps extends BaseMemberPickerProps {
  values: string[];
  onChange: (memberIds: string[]) => void;
  /** IDs that appear on some but not all items in the selection (rendered italic + muted). */
  partialIds?: string[];
}

const DEFAULT_TRIGGER_CLASSNAME =
  'inline-flex max-w-full min-w-0 items-center overflow-hidden rounded-md px-1.5 py-0.5 text-ui transition-colors hover:bg-accent cursor-pointer';

function TooltipWrappedTrigger({
  label,
  popoverOpen,
  children,
}: {
  label?: string;
  popoverOpen: boolean;
  children: ReactElement;
}) {
  if (!label || popoverOpen) return children;

  return (
    <Tooltip>
      <TooltipTrigger asChild>{children}</TooltipTrigger>
      <TooltipContent side="top">{label}</TooltipContent>
    </Tooltip>
  );
}

function defaultGetMemberValue(member: AssignableMember) {
  return member.id;
}

function getSelectableMembers(members: AssignableMember[]) {
  return members.filter((member) => member.status === 'active');
}

function MemberList({
  members,
  selectedValues,
  partialValues,
  onToggle,
  noneLabel,
  multiple,
  getMemberValue,
  getDisabledReason,
}: {
  members: AssignableMember[];
  selectedValues: string[];
  partialValues?: string[];
  onToggle: (value: string) => void;
  noneLabel?: string;
  multiple: boolean;
  getMemberValue: MemberValueGetter;
  getDisabledReason?: (member: AssignableMember) => string | null | undefined;
}) {
  return (
    <QuietDropdownOptions searchPlaceholder="Search members...">
      <QuietDropdownEmpty className="py-3 text-center text-ui text-muted-foreground">
        No members found
      </QuietDropdownEmpty>
      <QuietDropdownGroup>
        {!multiple && noneLabel ? (
          <QuietDropdownItem
            data-checked={selectedValues.includes('__none__')}
            value="__none__"
            keywords={[noneLabel]}
            onSelect={() => onToggle('__none__')}
            className={cn(
              'flex min-w-0 items-center gap-2 text-ui',
              selectedValues.includes('__none__') &&
                'font-medium text-foreground',
            )}
          >
            <span className="min-w-0 flex-1 truncate">{noneLabel}</span>
          </QuietDropdownItem>
        ) : null}

        {members.map((member) => {
          const optionValue = `${formatAssignableMemberName(member)} ${member.email}`;
          const memberId = getMemberValue(member);
          const isSelected = selectedValues.some((value) =>
            matchesAssignableMemberValue(member, value, getMemberValue),
          );

          const isPartial = !!partialValues?.includes(memberId);
          const disabledReason = getDisabledReason?.(member);
          const item = (
            <QuietDropdownItem
              data-checked={isSelected}
              key={memberId}
              value={memberId}
              keywords={[optionValue]}
              disabled={Boolean(disabledReason)}
              onSelect={() => {
                if (!disabledReason) onToggle(memberId);
              }}
              className={cn(
                'flex min-w-0 items-center gap-2 text-ui',
                isPartial && 'italic text-muted-foreground',
              )}
            >
              <UserAvatar
                name={member.display_name || member.email}
                avatarUrl={member.avatar_url}
                avatarStyle={member.avatar_style}
                avatarSeed={member.avatar_seed}
                avatarBackgroundMode={member.avatar_background_mode}
                avatarBackgroundColor={member.avatar_background_color}
                className="h-5 w-5"
                fallbackClassName="text-[8px]"
              />
              <span className="min-w-0 flex-1 truncate">
                {member.display_name || member.email}
              </span>
            </QuietDropdownItem>
          );
          if (!disabledReason) return item;
          return (
            <Tooltip key={memberId}>
              <TooltipTrigger asChild>
                <div tabIndex={0} aria-label={`${member.display_name || member.email}: ${disabledReason}`}>{item}</div>
              </TooltipTrigger>
              <TooltipContent side="right" className="z-[70] max-w-64">{disabledReason}</TooltipContent>
            </Tooltip>
          );
        })}
      </QuietDropdownGroup>
    </QuietDropdownOptions>
  );
}

export function MemberPickerPopover({
  value,
  members,
  onChange,
  renderTrigger,
  noneLabel = 'None',
  triggerClassName,
  triggerLabel,
  contentClassName,
  align = 'start',
  open,
  onOpenChange,
  getMemberValue = defaultGetMemberValue,
  getDisabledReason,
  disabled = false,
  lazyMount = false,
}: MemberPickerPopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isOpen = open ?? uncontrolledOpen;
  const triggerRef = useQuietDropdownFocusReturn(isOpen, lazyMount);
  const setOpen = onOpenChange ?? setUncontrolledOpen;

  const selectableMembers = useMemo(
    () => getSelectableMembers(members),
    [members],
  );

  const handleToggle = (selectedValue: string) => {
    onChange(selectedValue);
    setOpen(false);
  };

  const trigger = (
    <button
      ref={triggerRef}
      type="button"
      disabled={disabled}
      aria-label={triggerLabel}
      className={cn(
        DEFAULT_TRIGGER_CLASSNAME,
        disabled && 'cursor-default hover:bg-transparent',
        triggerClassName,
      )}
      onClick={(event) => {
        event.stopPropagation();
        if (lazyMount && !disabled && !isOpen) {
          setOpen(true);
        }
      }}
      onKeyDown={(event) => event.stopPropagation()}
    >
      <span className="flex min-w-0 max-w-full items-center gap-1.5 overflow-hidden [&_span:last-child]:truncate [&_span:last-child]:whitespace-nowrap">
        {renderTrigger()}
      </span>
    </button>
  );

  if (lazyMount && !disabled && !isOpen) {
    return (
      <TooltipWrappedTrigger label={triggerLabel} popoverOpen={isOpen}>
        {trigger}
      </TooltipWrappedTrigger>
    );
  }

  return (
    <Popover open={isOpen} onOpenChange={setOpen}>
      <TooltipWrappedTrigger label={triggerLabel} popoverOpen={isOpen}>
        <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      </TooltipWrappedTrigger>
      {!disabled ? (
        <PMDropdownContent
          className={cn('z-[60] w-[240px] p-0', contentClassName)}
          align={align}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <MemberList
            members={selectableMembers}
            selectedValues={[value || '__none__']}
            onToggle={handleToggle}
            noneLabel={noneLabel}
            multiple={false}
            getMemberValue={getMemberValue}
            getDisabledReason={getDisabledReason}
          />
        </PMDropdownContent>
      ) : null}
    </Popover>
  );
}

export function MultiMemberPickerPopover({
  values,
  members,
  onChange,
  renderTrigger,
  triggerClassName,
  triggerLabel,
  contentClassName,
  align = 'start',
  open,
  onOpenChange,
  getMemberValue = defaultGetMemberValue,
  disabled = false,
  lazyMount = false,
  partialIds,
}: MultiMemberPickerPopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isOpen = open ?? uncontrolledOpen;
  const triggerRef = useQuietDropdownFocusReturn(isOpen, lazyMount);
  const setOpen = onOpenChange ?? setUncontrolledOpen;

  const selectableMembers = useMemo(
    () => getSelectableMembers(members),
    [members],
  );

  const handleToggle = (selectedValue: string) => {
    const nextValues = values.includes(selectedValue)
      ? values.filter((value) => value !== selectedValue)
      : [...values, selectedValue];
    onChange(nextValues);
  };

  const trigger = (
    <button
      ref={triggerRef}
      type="button"
      disabled={disabled}
      aria-label={triggerLabel}
      className={cn(
        DEFAULT_TRIGGER_CLASSNAME,
        disabled && 'cursor-default hover:bg-transparent',
        triggerClassName,
      )}
      onClick={(event) => {
        event.stopPropagation();
        if (lazyMount && !disabled && !isOpen) {
          setOpen(true);
        }
      }}
      onKeyDown={(event) => event.stopPropagation()}
    >
      <span className="flex min-w-0 max-w-full items-center gap-1.5 overflow-hidden [&_span:last-child]:truncate [&_span:last-child]:whitespace-nowrap">
        {renderTrigger()}
      </span>
    </button>
  );

  if (lazyMount && !disabled && !isOpen) {
    return (
      <TooltipWrappedTrigger label={triggerLabel} popoverOpen={isOpen}>
        {trigger}
      </TooltipWrappedTrigger>
    );
  }

  return (
    <Popover open={isOpen} onOpenChange={setOpen}>
      <TooltipWrappedTrigger label={triggerLabel} popoverOpen={isOpen}>
        <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      </TooltipWrappedTrigger>
      {!disabled ? (
        <PMDropdownContent
          className={cn('z-[60] w-[240px] p-0', contentClassName)}
          align={align}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <MemberList
            members={selectableMembers}
            selectedValues={values}
            partialValues={partialIds}
            onToggle={handleToggle}
            multiple
            getMemberValue={getMemberValue}
          />
        </PMDropdownContent>
      ) : null}
    </Popover>
  );
}
