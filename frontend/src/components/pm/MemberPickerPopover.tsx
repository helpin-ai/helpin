import { useState } from 'react';
import { Check } from 'lucide-react';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { UserAvatar } from '@/components/pm/UserAvatar';
import type { AssignableMember } from '@/lib/types';
import { formatAssignableMemberName } from '@/lib/assignableMembers';

interface MemberPickerPopoverProps {
  value: string;
  members: AssignableMember[];
  onChange: (memberId: string) => void;
  renderTrigger: () => React.ReactNode;
  noneLabel?: string;
}

export function MemberPickerPopover({
  value,
  members,
  onChange,
  renderTrigger,
  noneLabel = 'None',
}: MemberPickerPopoverProps) {
  const [open, setOpen] = useState(false);

  const joined = members.filter((m) => m.status === 'active');
  const invited = members.filter((m) => m.status === 'pending');

  const handleSelect = (id: string) => {
    onChange(id);
    setOpen(false);
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          type="button"
          className="inline-flex max-w-full items-center gap-1.5 overflow-hidden rounded-md px-1.5 py-0.5 text-xs transition-colors hover:bg-accent cursor-pointer"
        >
          {renderTrigger()}
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-64 p-0.5 z-[60]" align="start" onWheel={(e) => e.stopPropagation()}>
        <div className="flex max-h-60 flex-col overflow-y-auto overscroll-contain">
          {/* None option */}
          <button
            type="button"
            className={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs transition-colors cursor-pointer
              ${value === '__none__' ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
            `}
            onClick={() => handleSelect('__none__')}
          >
            <span className="truncate">{noneLabel}</span>
            {value === '__none__' && <Check className="ml-auto h-3 w-3 shrink-0" />}
          </button>

          {/* Joined members */}
          {joined.length > 0 && (
            <>
              <div className="px-2 pt-2 pb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">
                Members
              </div>
              {joined.map((m) => (
                <MemberRow
                  key={m.id}
                  member={m}
                  selected={value === m.id}
                  onSelect={handleSelect}
                />
              ))}
            </>
          )}

          {/* Invited members */}
          {invited.length > 0 && (
            <>
              <div className="px-2 pt-2 pb-1 text-[10px] font-medium uppercase tracking-wider text-muted-foreground/70">
                Invited
              </div>
              {invited.map((m) => (
                <MemberRow
                  key={m.id}
                  member={m}
                  selected={value === m.id}
                  onSelect={handleSelect}
                />
              ))}
            </>
          )}
        </div>
      </PopoverContent>
    </Popover>
  );
}

function MemberRow({
  member,
  selected,
  onSelect,
}: {
  member: AssignableMember;
  selected: boolean;
  onSelect: (id: string) => void;
}) {
  const name = formatAssignableMemberName(member);

  return (
    <button
      type="button"
      className={`flex items-center gap-2 rounded-sm px-2 py-1.5 text-xs transition-colors cursor-pointer
        ${selected ? 'bg-accent text-foreground font-medium' : 'text-muted-foreground hover:bg-accent hover:text-foreground'}
      `}
      onClick={() => onSelect(member.id)}
    >
      <UserAvatar
        name={member.display_name || member.email}
        avatarUrl={member.avatar_url}
        className="h-5 w-5"
        fallbackClassName="text-[8px]"
      />
      <span className="truncate">{name}</span>
      {selected && <Check className="ml-auto h-3 w-3 shrink-0" />}
    </button>
  );
}
