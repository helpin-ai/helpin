import { useState } from 'react';
import { toast } from 'sonner';
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import {
  PLAN_OPTIONS,
  annualDiscountPct,
  formatCents,
  formatNumber,
  planPriceCents,
} from '@/lib/billingUtils';
import type { BillingInterval, BillingPlan } from '@/lib/billingTypes';
import { useBillingCheckout } from '@/hooks/queries';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  workspaceId: string;
  workspaceName: string;
  currentPlan: BillingPlan;
}

export function PlanChangeModal({
  open,
  onOpenChange,
  workspaceId,
  workspaceName,
  currentPlan,
}: Props) {
  const [plan, setPlan] = useState<BillingPlan>(
    currentPlan === 'starter' ? 'starter' : 'growth',
  );
  const [interval, setInterval] = useState<BillingInterval>('monthly');

  const checkout = useBillingCheckout();

  const handleConfirm = () => {
    checkout.mutate(
      { wsId: workspaceId, data: { plan, interval } },
      {
        onSuccess: (res) => {
          if (res?.url) {
            window.location.href = res.url;
          } else {
            toast.error('No checkout URL returned');
          }
        },
        onError: (e) => toast.error(e instanceof Error ? e.message : 'Checkout failed'),
      },
    );
  };

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-lg">
        <DialogHeader>
          <DialogTitle>Change plan · {workspaceName}</DialogTitle>
          <DialogDescription>
            Pick a plan and billing interval. You'll be taken to Stripe to confirm payment.
          </DialogDescription>
        </DialogHeader>

        <div className="space-y-4">
          {/* Interval toggle */}
          <div className="flex items-center gap-1 rounded-lg bg-muted p-1 text-xs">
            {(['monthly', 'annual'] as BillingInterval[]).map((iv) => (
              <button
                key={iv}
                type="button"
                onClick={() => setInterval(iv)}
                className={cn(
                  'flex-1 rounded-md px-3 py-1.5 font-medium capitalize transition-colors',
                  interval === iv ? 'bg-background shadow-sm' : 'text-muted-foreground',
                )}
              >
                {iv}
                {iv === 'annual' && <span className="ml-1 text-emerald-600">save</span>}
              </button>
            ))}
          </div>

          {/* Plan cards */}
          <div className="grid grid-cols-2 gap-3">
            {PLAN_OPTIONS.map((opt) => {
              const selected = plan === opt.plan;
              const priceCents = planPriceCents(opt, interval);
              const discount = annualDiscountPct(opt);
              return (
                <button
                  key={opt.plan}
                  type="button"
                  onClick={() => setPlan(opt.plan)}
                  className={cn(
                    'rounded-lg border p-3 text-left transition-colors',
                    selected
                      ? 'border-primary ring-1 ring-primary'
                      : 'border-border hover:border-muted-foreground/40',
                  )}
                >
                  <div className="flex items-center justify-between">
                    <span className="text-sm font-semibold">{opt.label}</span>
                    {currentPlan === opt.plan && (
                      <span className="text-[10px] text-muted-foreground">current</span>
                    )}
                  </div>
                  <div className="mt-1 text-lg font-bold">
                    {formatCents(priceCents)}
                    <span className="text-xs font-normal text-muted-foreground">/mo</span>
                  </div>
                  {interval === 'annual' && discount > 0 && (
                    <div className="text-[11px] text-emerald-600">
                      {discount}% off · billed annually
                    </div>
                  )}
                  <div className="mt-1 text-xs text-muted-foreground">
                    {formatNumber(opt.credits)} AI usage / mo
                  </div>
                </button>
              );
            })}
          </div>

          <div className="rounded-md border bg-muted/30 px-3 py-2 text-xs text-muted-foreground">
            Stripe Checkout will collect and save the payment method for this subscription.
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button onClick={handleConfirm} disabled={checkout.isPending}>
            {checkout.isPending ? 'Redirecting…' : 'Continue to payment'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
