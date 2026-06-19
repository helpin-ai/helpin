import { useState } from 'react';
import { toast } from 'sonner';
import {
  Popover,
  PopoverContent,
  PopoverHeader,
  PopoverTitle,
  PopoverTrigger,
} from '@/components/ui/popover';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import type { MemberWithUser } from '@/lib/types';
import type { WorkspaceBillingCard } from '@/lib/billingTypes';
import { useSetBillingOwner } from '@/hooks/queries';

interface Props {
  orgId: string;
  card: WorkspaceBillingCard;
  members: MemberWithUser[];
  disabled?: boolean;
}

export function BillingOwnerPopover({ orgId, card, members, disabled }: Props) {
  const [open, setOpen] = useState(false);
  const setOwner = useSetBillingOwner(orgId);

  const selectedId = card.billing_owner?.user_id ?? null;

  const handleSelect = (userId: string | null) => {
    setOwner.mutate(
      { wsId: card.workspace_id, data: { user_id: userId } },
      {
        onSuccess: () => {
          toast.success('Billing owner updated');
          setOpen(false);
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed to update'),
      },
    );
  };

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className="h-6 px-1.5 text-xs" disabled={disabled}>
          Change
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-72 p-0">
        <PopoverHeader className="px-3 pt-3">
          <PopoverTitle className="text-xs font-medium">Billing owner</PopoverTitle>
        </PopoverHeader>
        <div className="max-h-64 overflow-y-auto p-1">
          <button
            type="button"
            onClick={() => handleSelect(null)}
            className={cn(
              'flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-xs hover:bg-accent',
              selectedId === null && 'bg-accent',
            )}
          >
            <span className="text-muted-foreground">No billing owner</span>
            {selectedId === null && <span className="text-muted-foreground">✓</span>}
          </button>
          {members.map((m) => (
            <button
              key={m.user_id}
              type="button"
              onClick={() => handleSelect(m.user_id)}
              className={cn(
                'flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-xs hover:bg-accent',
                selectedId === m.user_id && 'bg-accent',
              )}
            >
              <span className="min-w-0 truncate">
                {m.full_name || m.email}
                <span className="ml-1 text-muted-foreground">{m.email}</span>
              </span>
              {selectedId === m.user_id && <span className="text-muted-foreground">✓</span>}
            </button>
          ))}
        </div>
      </PopoverContent>
    </Popover>
  );
}
