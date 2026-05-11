import { useMemo, useState } from 'react';
import type { ReactElement, ReactNode } from 'react';
import { Tick01Icon } from '@/lib/icons';

import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { formatAssignableMemberName, matchesAssignableMemberValue } from '@/lib/assignableMembers';
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
  value: string;
  onChange: (memberId: string) => void;
  noneLabel?: string;
}

interface MultiMemberPickerPopoverProps extends BaseMemberPickerProps {
  values: string[];
  onChange: (memberIds: string[]) => void;
}

const DEFAULT_TRIGGER_CLASSNAME =
  'inline-flex max-w-full min-w-0 items-center overflow-hidden rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer';

function TooltipWrappedTrigger({
  label,
  popoverOpen,
  children,
}: {
  label?: string;
  popoverOpen: boolean;
  children: ReactElement;
}) {
  if (!label) return children;

  return (
    <Tooltip open={popoverOpen ? false : undefined}>
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
  onToggle,
  noneLabel,
  multiple,
  getMemberValue,
}: {
  members: AssignableMember[];
  selectedValues: string[];
  onToggle: (value: string) => void;
  noneLabel?: string;
  multiple: boolean;
  getMemberValue: MemberValueGetter;
}) {
  return (
    <Command>
      <CommandInput placeholder="Search members..." className="h-8 text-xs" />
      <CommandList>
        <CommandEmpty className="py-3 text-center text-xs text-muted-foreground">
          No members found
        </CommandEmpty>
        <CommandGroup>
          {!multiple && noneLabel ? (
            <CommandItem
              value={noneLabel}
              onSelect={() => onToggle('__none__')}
              className={cn(
                'flex min-w-0 items-center gap-2 text-xs',
                selectedValues.includes('__none__') && 'font-medium text-foreground',
              )}
            >
              <span className="min-w-0 flex-1 truncate">{noneLabel}</span>
              {selectedValues.includes('__none__') && <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" />}
            </CommandItem>
          ) : null}

          {members.map((member) => {
            const optionValue = `${formatAssignableMemberName(member)} ${member.email}`;
            const memberId = getMemberValue(member);
            const isSelected = selectedValues.some((value) =>
              matchesAssignableMemberValue(member, value, getMemberValue),
            );

            return (
              <CommandItem
                key={memberId}
                value={optionValue}
                onSelect={() => onToggle(memberId)}
                className="flex min-w-0 items-center gap-2 text-xs"
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
                <span className="min-w-0 flex-1 truncate">{member.display_name || member.email}</span>
                {isSelected && <Tick01Icon className="ml-auto h-3.5 w-3.5 text-primary" />}
              </CommandItem>
            );
          })}
        </CommandGroup>
      </CommandList>
    </Command>
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
  disabled = false,
  lazyMount = false,
}: MemberPickerPopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isOpen = open ?? uncontrolledOpen;
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
      type="button"
      disabled={disabled}
      aria-label={triggerLabel}
      className={cn(DEFAULT_TRIGGER_CLASSNAME, disabled && 'cursor-default hover:bg-transparent', triggerClassName)}
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
        <PopoverContent
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
          />
        </PopoverContent>
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
}: MultiMemberPickerPopoverProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(false);
  const isOpen = open ?? uncontrolledOpen;
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
      type="button"
      disabled={disabled}
      aria-label={triggerLabel}
      className={cn(DEFAULT_TRIGGER_CLASSNAME, disabled && 'cursor-default hover:bg-transparent', triggerClassName)}
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
        <PopoverContent
          className={cn('z-[60] w-[240px] p-0', contentClassName)}
          align={align}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <MemberList
            members={selectableMembers}
            selectedValues={values}
            onToggle={handleToggle}
            multiple
            getMemberValue={getMemberValue}
          />
        </PopoverContent>
      ) : null}
    </Popover>
  );
}
