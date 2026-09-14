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
import type { PaymentMethod, WorkspaceBillingCard } from '@/ee/lib/billingTypes';
import { useLinkPaymentMethod } from '@/ee/hooks/queries/useBilling';

interface Props {
  orgId: string;
  card: WorkspaceBillingCard;
  cards: PaymentMethod[];
  onAddCard?: () => void;
  disabled?: boolean;
}

export function BilledToPopover({ orgId, card, cards, onAddCard, disabled }: Props) {
  const [open, setOpen] = useState(false);
  const link = useLinkPaymentMethod(orgId);

  const selectedId = card.payment_method?.id ?? null;

  const handleSelect = (paymentMethodId: string | null) => {
    link.mutate(
      { wsId: card.workspace_id, data: { payment_method_id: paymentMethodId } },
      {
        onSuccess: () => {
          toast.success('Payment method updated');
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
          <PopoverTitle className="text-xs font-medium">Billed to</PopoverTitle>
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
            <span>Organization default card</span>
            {selectedId === null && <span className="text-muted-foreground">✓</span>}
          </button>
          {cards.map((c) => (
            <button
              key={c.id}
              type="button"
              onClick={() => handleSelect(c.id)}
              className={cn(
                'flex w-full items-center justify-between rounded-md px-2 py-1.5 text-left text-xs hover:bg-accent',
                selectedId === c.id && 'bg-accent',
              )}
            >
              <span className="capitalize">
                {c.brand} ···· {c.last4}
              </span>
              {selectedId === c.id && <span className="text-muted-foreground">✓</span>}
            </button>
          ))}
        </div>
        {onAddCard && (
          <div className="border-t p-1">
            <Button
              variant="ghost"
              size="sm"
              className="h-7 w-full justify-start text-xs"
              onClick={() => {
                setOpen(false);
                onAddCard();
              }}
            >
              Manage cards in Stripe
            </Button>
          </div>
        )}
      </PopoverContent>
    </Popover>
  );
}
