import { useState } from 'react';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { cn } from '@/lib/utils';
import type { PaymentMethod } from '@/ee/lib/billingTypes';
import { useUpdateCard, useDeleteCard } from '@/ee/hooks/queries/useBilling';

interface Props {
  orgId: string;
  card: PaymentMethod;
}

export function PaymentMethodCard({ orgId, card }: Props) {
  const [linkedOpen, setLinkedOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [cardholder, setCardholder] = useState(card.cardholder);

  const updateCard = useUpdateCard(orgId);
  const deleteCard = useDeleteCard(orgId);

  const handleSetDefault = () => {
    updateCard.mutate(
      { cardId: card.id, data: { is_org_default: true } },
      {
        onSuccess: () => toast.success('Set as default card'),
        onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed'),
      },
    );
  };

  const handleSaveEdit = () => {
    updateCard.mutate(
      { cardId: card.id, data: { cardholder } },
      {
        onSuccess: () => {
          toast.success('Card updated');
          setEditOpen(false);
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed'),
      },
    );
  };

  const handleRemove = () => {
    if (!window.confirm('Remove this card? Workspaces using it fall back to the org default.'))
      return;
    deleteCard.mutate(card.id, {
      onSuccess: () => toast.success('Card removed'),
      onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed'),
    });
  };

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border bg-card p-4">
      <div className="min-w-0">
        <div className="flex items-center gap-2">
          <span className="font-medium capitalize">{card.brand}</span>
          <span className="tabular-nums text-muted-foreground">···· {card.last4}</span>
          {card.is_org_default && (
            <span className="rounded bg-primary/10 px-1.5 py-0.5 text-[10px] font-medium text-primary">
              Default
            </span>
          )}
        </div>
        <div className="mt-0.5 text-xs text-muted-foreground">
          Expires {String(card.exp_month).padStart(2, '0')}/{card.exp_year}
          {card.cardholder && ` · ${card.cardholder}`}
        </div>
        <div className="mt-1 text-xs">
          <span className="text-muted-foreground">
            Linked to {card.linked_workspace_count} workspace
            {card.linked_workspace_count === 1 ? '' : 's'}
          </span>
          {card.linked_workspace_count > 0 && (
            <Popover open={linkedOpen} onOpenChange={setLinkedOpen}>
              <PopoverTrigger asChild>
                <button type="button" className="ml-1 text-primary hover:underline">
                  · See linked
                </button>
              </PopoverTrigger>
              <PopoverContent align="start" className="w-56 p-2 text-xs">
                <div className="mb-1 font-medium">Linked workspaces</div>
                <ul className="space-y-0.5">
                  {card.linked_workspaces.map((w) => (
                    <li key={w.id} className="truncate text-muted-foreground">
                      {w.name}
                    </li>
                  ))}
                </ul>
              </PopoverContent>
            </Popover>
          )}
        </div>
      </div>

      <div className="flex flex-wrap items-center gap-1.5">
        {!card.is_org_default && (
          <Button variant="outline" size="sm" onClick={handleSetDefault}>
            Set default
          </Button>
        )}
        <Button variant="ghost" size="sm" onClick={() => setEditOpen(true)}>
          Edit
        </Button>
        <Button
          variant="ghost"
          size="sm"
          className={cn('text-destructive hover:text-destructive')}
          onClick={handleRemove}
          disabled={deleteCard.isPending}
        >
          Remove
        </Button>
      </div>

      <Dialog open={editOpen} onOpenChange={setEditOpen}>
        <DialogContent className="sm:max-w-sm">
          <DialogHeader>
            <DialogTitle>Edit card details</DialogTitle>
          </DialogHeader>
          <div className="space-y-2">
            <Label htmlFor="cardholder" className="text-xs">
              Cardholder name
            </Label>
            <Input
              id="cardholder"
              value={cardholder}
              onChange={(e) => setCardholder(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button variant="outline" onClick={() => setEditOpen(false)}>
              Cancel
            </Button>
            <Button onClick={handleSaveEdit} disabled={updateCard.isPending}>
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
