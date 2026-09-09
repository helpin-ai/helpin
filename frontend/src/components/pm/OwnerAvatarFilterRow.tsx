import { useMemo, useState } from 'react';
import {
  QuietDropdownRoot as Popover,
  QuietDropdownContent as PopoverContent,
  QuietDropdownTrigger as PopoverTrigger,
} from '@/components/design-system/quiet-dropdown';
import {
  QuietDropdownOptions,
  QuietDropdownEmpty,
  QuietDropdownGroup,
  QuietDropdownItem,
} from '@/components/design-system/quiet-dropdown';
import { QuickTooltip } from '@/components/ui/quick-tooltip';
import { useWorkspaceMemberPresenceMap } from '@/hooks/queries';
import type { AssignableMember } from '@/lib/types';
import { cn } from '@/lib/utils';
import { UserAvatar } from './UserAvatar';

const OWNER_AVATAR_INLINE_CAP = 7;

export function OwnerAvatarFilterRow({
  workspaceId,
  members,
  selectedIds,
  onToggle,
  className,
}: {
  workspaceId: string;
  members: AssignableMember[];
  selectedIds: string[];
  onToggle: (memberId: string) => void;
  className?: string;
}) {
  const { data: memberPresenceByUserId } =
    useWorkspaceMemberPresenceMap(workspaceId);
  const [overflowOpen, setOverflowOpen] = useState(false);

  // Sort selected first so they're guaranteed visible in the inline row.
  const sortedMembers = useMemo(() => {
    const selected: AssignableMember[] = [];
    const unselected: AssignableMember[] = [];
    for (const m of members) {
      if (selectedIds.includes(m.id)) selected.push(m);
      else unselected.push(m);
    }
    return [...selected, ...unselected];
  }, [members, selectedIds]);

  const visible = sortedMembers.slice(0, OWNER_AVATAR_INLINE_CAP);
  const overflow = sortedMembers.slice(OWNER_AVATAR_INLINE_CAP);
  const overflowCount = overflow.length;
  const hasOverflowSelected = overflow.some((m) => selectedIds.includes(m.id));

  if (members.length === 0) return null;

  return (
    <div className={cn('ml-3 flex min-w-0 items-center -space-x-1', className)}>
      {visible.map((member) => {
        const isSelected = selectedIds.includes(member.id);
        const label = member.display_name?.trim() || member.email;
        const presenceStatus = member.user_id
          ? (memberPresenceByUserId?.get(member.user_id)?.status ?? null)
          : null;
        return (
          <QuickTooltip key={member.id} label={label}>
            <button
              type="button"
              onClick={() => onToggle(member.id)}
              className={`relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full ring-offset-1 ring-offset-background transition-all hover:z-10 ${
                isSelected
                  ? 'z-10 ring-[1.5px] ring-ring'
                  : 'ring-0 opacity-70 hover:opacity-100'
              }`}
              aria-pressed={isSelected}
              aria-label={`Filter by owner ${label}`}
            >
              <UserAvatar
                name={label}
                avatarUrl={member.avatar_url}
                avatarStyle={member.avatar_style}
                avatarSeed={member.avatar_seed}
                avatarBackgroundMode={member.avatar_background_mode}
                avatarBackgroundColor={member.avatar_background_color}
                presenceStatus={presenceStatus}
                className="h-6 w-6"
                fallbackClassName="text-[8px]"
              />
            </button>
          </QuickTooltip>
        );
      })}
      {overflowCount > 0 && (
        <Popover open={overflowOpen} onOpenChange={setOverflowOpen}>
          <QuickTooltip label={`${overflowCount} more`}>
            <PopoverTrigger asChild>
              <button
                type="button"
                className={`relative flex h-8 w-8 shrink-0 items-center justify-center rounded-full text-[10px] font-medium ring-offset-1 ring-offset-background transition-colors hover:z-10 ${
                  hasOverflowSelected
                    ? 'z-10 bg-primary/10 text-primary ring-[1.5px] ring-ring'
                    : 'bg-muted text-muted-foreground hover:bg-accent hover:text-foreground'
                }`}
                aria-label={`Show ${overflowCount} more members`}
              >
                +{overflowCount}
              </button>
            </PopoverTrigger>
          </QuickTooltip>
          <PopoverContent className="w-64 p-0" align="start">
            <QuietDropdownOptions searchPlaceholder="Search members...">
              <QuietDropdownEmpty>No members.</QuietDropdownEmpty>
              <QuietDropdownGroup>
                {members.map((member) => {
                  const isSelected = selectedIds.includes(member.id);
                  const label = member.display_name?.trim() || member.email;
                  const presenceStatus = member.user_id
                    ? (memberPresenceByUserId?.get(member.user_id)?.status ??
                      null)
                    : null;
                  return (
                    <QuietDropdownItem
                      data-checked={isSelected}
                      key={member.id}
                      value={member.id}
                      keywords={[label, member.email]}
                      onSelect={() => onToggle(member.id)}
                    >
                      <UserAvatar
                        name={label}
                        avatarUrl={member.avatar_url}
                        avatarStyle={member.avatar_style}
                        avatarSeed={member.avatar_seed}
                        avatarBackgroundMode={member.avatar_background_mode}
                        avatarBackgroundColor={member.avatar_background_color}
                        presenceStatus={presenceStatus}
                        className="mr-2 h-5 w-5"
                        fallbackClassName="text-[8px]"
                      />
                      <span className="min-w-0 flex-1 truncate">{label}</span>
                    </QuietDropdownItem>
                  );
                })}
              </QuietDropdownGroup>
            </QuietDropdownOptions>
          </PopoverContent>
        </Popover>
      )}
    </div>
  );
}
