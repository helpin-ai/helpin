import { useMemo, useState } from 'react';
import { Check } from 'lucide-react';

import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import { formatAssignableMemberName, matchesAssignableMemberValue } from '@/lib/assignableMembers';
import { cn } from '@/lib/utils';
import type { AssignableMember } from '@/lib/types';

type PopoverAlign = 'start' | 'center' | 'end';
type MemberValueGetter = (member: AssignableMember) => string;

interface BaseMemberPickerProps {
  members: AssignableMember[];
  renderTrigger: () => React.ReactNode;
  triggerClassName?: string;
  contentClassName?: string;
  align?: PopoverAlign;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  getMemberValue?: MemberValueGetter;
  disabled?: boolean;
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
              {selectedValues.includes('__none__') && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
                  className="h-5 w-5"
                  fallbackClassName="text-[8px]"
                />
                <span className="min-w-0 flex-1 truncate">{member.display_name || member.email}</span>
                {isSelected && <Check className="ml-auto h-3.5 w-3.5 text-primary" />}
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
  contentClassName,
  align = 'start',
  open,
  onOpenChange,
  getMemberValue = defaultGetMemberValue,
  disabled = false,
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

  return (
    <Popover open={isOpen} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className={cn(DEFAULT_TRIGGER_CLASSNAME, disabled && 'cursor-default hover:bg-transparent', triggerClassName)}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <span className="flex min-w-0 max-w-full items-center gap-1.5 overflow-hidden [&_span:last-child]:truncate [&_span:last-child]:whitespace-nowrap">
            {renderTrigger()}
          </span>
        </button>
      </PopoverTrigger>
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
  contentClassName,
  align = 'start',
  open,
  onOpenChange,
  getMemberValue = defaultGetMemberValue,
  disabled = false,
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

  return (
    <Popover open={isOpen} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          disabled={disabled}
          className={cn(DEFAULT_TRIGGER_CLASSNAME, disabled && 'cursor-default hover:bg-transparent', triggerClassName)}
          onClick={(event) => event.stopPropagation()}
          onKeyDown={(event) => event.stopPropagation()}
        >
          <span className="flex min-w-0 max-w-full items-center gap-1.5 overflow-hidden [&_span:last-child]:truncate [&_span:last-child]:whitespace-nowrap">
            {renderTrigger()}
          </span>
        </button>
      </PopoverTrigger>
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
