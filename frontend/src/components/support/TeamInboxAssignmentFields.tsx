import { useState } from 'react';
import { ArrowDown01Icon, Tick01Icon, HelpCircleIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { Command, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from '@/components/ui/command';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { AssignableMember } from '@/lib/types';
import { inboxAssignmentOptions, type InboxAssignmentMode } from './teamInboxAssignment';

export function TeamInboxAssignmentFields({ mode, members, selectedIDs, loading, onModeChange, onMembersChange }: {
  mode: InboxAssignmentMode;
  members: AssignableMember[];
  selectedIDs: string[];
  loading: boolean;
  onModeChange: (mode: InboxAssignmentMode) => void;
  onMembersChange: (ids: string[]) => void;
}) {
  const [open, setOpen] = useState(false);
  const multiple = mode === 'round_robin';
  const allSelected = members.length > 0 && members.every(member => selectedIDs.includes(member.id));
  const selected = members.filter(member => selectedIDs.includes(member.id));
  const selectionLabel = selected.length === 1
    ? selected[0].display_name || selected[0].email
    : selected.length > 1 ? `${selected.length} members selected` : multiple ? 'Select members' : 'Select a member';

  return (
    <div className="mt-4 space-y-3">
      <div className="space-y-1.5">
        <div className="flex items-center gap-1.5">
          <Label htmlFor="inbox-assignment-mode">Assignment</Label>
          <Tooltip>
            <TooltipTrigger asChild>
              <button type="button" aria-label="About inbox assignment" className="text-muted-foreground hover:text-foreground">
                <HelpCircleIcon className="h-3.5 w-3.5" />
              </button>
            </TooltipTrigger>
            <TooltipContent className="max-w-64">
              Inbox access lets members follow conversations. Only selected members receive automatic assignments. AI handoffs also respect the workspace’s leave-unassigned setting.
            </TooltipContent>
          </Tooltip>
        </div>
        <Select value={mode} onValueChange={onModeChange}>
          <SelectTrigger id="inbox-assignment-mode" className="w-full">
            <SelectValue>{inboxAssignmentOptions.find(option => option.value === mode)?.label}</SelectValue>
          </SelectTrigger>
          <SelectContent>
            {inboxAssignmentOptions.map(option => (
              <SelectItem key={option.value} value={option.value} textValue={option.label}>
                <span className="flex flex-col items-start gap-0.5 py-0.5">
                  <span>{option.label}</span>
                  <span className="text-xs text-muted-foreground">{option.description}</span>
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      {mode !== 'manual' && (
        <div className="space-y-1.5">
          <Label htmlFor="inbox-assignment-members">{multiple ? 'Assign between' : 'Assign to'}</Label>
          <Popover open={open} onOpenChange={setOpen}>
            <PopoverTrigger asChild>
              <Button id="inbox-assignment-members" type="button" variant="outline" role="combobox" aria-expanded={open} disabled={loading} className="w-full justify-between font-normal">
                <span className="truncate">{loading ? 'Loading members…' : selectionLabel}</span>
                <ArrowDown01Icon className="ml-2 h-4 w-4 shrink-0 text-muted-foreground" />
              </Button>
            </PopoverTrigger>
            <PopoverContent align="start" className="w-[var(--radix-popover-trigger-width)] p-0">
              <Command>
                <CommandInput placeholder="Search members…" />
                {multiple && members.length > 5 && (
                  <div className="border-b px-2 py-1">
                    <Button type="button" variant="ghost" size="sm" className="w-full justify-start" onClick={() => onMembersChange(allSelected ? [] : members.map(member => member.id))}>
                      {allSelected ? 'Clear selection' : 'Select all'}
                    </Button>
                  </div>
                )}
                <CommandList>
                  <CommandEmpty>No eligible members found.</CommandEmpty>
                  <CommandGroup>
                    {members.map(member => (
                      <CommandItem key={member.id} value={`${member.display_name} ${member.email} ${member.id}`} onSelect={() => {
                        onMembersChange(multiple
                          ? selectedIDs.includes(member.id) ? selectedIDs.filter(id => id !== member.id) : [...selectedIDs, member.id]
                          : [member.id]);
                        if (!multiple) setOpen(false);
                      }} className="gap-2">
                        <UserAvatar name={member.display_name || member.email} avatarUrl={member.avatar_url} className="h-6 w-6" />
                        <span className="min-w-0 flex-1 truncate">{member.display_name || member.email}</span>
                        {selectedIDs.includes(member.id) && <Tick01Icon className="h-4 w-4 text-primary" aria-label="Selected" />}
                      </CommandItem>
                    ))}
                  </CommandGroup>
                </CommandList>
              </Command>
            </PopoverContent>
          </Popover>
          {!loading && selected.length === 0 && (
            <p className="text-xs text-muted-foreground">
              {members.length === 0 ? 'Add members with Support access to this inbox first.' : multiple ? 'Select at least one member.' : 'Select one member.'}
            </p>
          )}
        </div>
      )}
    </div>
  );
}
