import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import type { MemberWithUser } from '@/lib/types';
import type { PaymentMethod, WorkspaceBillingCard as WSCard } from '@/lib/billingTypes';
import {
  INTERVAL_LABEL,
  PLAN_LABEL,
  formatDate,
  formatNumber,
  planBadgeLabel,
  statusBadgeVariant,
} from '@/lib/billingUtils';
import { useSetOnDemand, useBillingPortal } from '@/hooks/queries';
import { BilledToPopover } from './BilledToPopover';
import { BillingOwnerPopover } from './BillingOwnerPopover';

interface Props {
  orgId: string;
  card: WSCard;
  cards: PaymentMethod[];
  members: MemberWithUser[];
  canManageOwner: boolean;
  onManage: (card: WSCard) => void;
  onChangePlan: (card: WSCard) => void;
  onAddCard: () => void;
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <div className="text-[11px] uppercase tracking-wide text-muted-foreground">{label}</div>
      <div className="mt-0.5 flex items-center justify-between gap-2 text-sm">{children}</div>
    </div>
  );
}

export function WorkspaceBillingCard({
  orgId,
  card,
  cards,
  members,
  canManageOwner,
  onManage,
  onChangePlan,
  onAddCard,
}: Props) {
  const setOnDemand = useSetOnDemand(orgId);
  const portal = useBillingPortal();

  const isFree = card.plan === 'free';
  const isTrial = card.trialing;
  const isPastDue = card.status === 'past_due' || card.status === 'canceled';
  const isPaid = !isFree && !isTrial;

  const creditsPct = card.included_credits
    ? Math.min(100, Math.round((card.credits_used / card.included_credits) * 100))
    : 0;

  const onDemandDisabled = isFree || isTrial || !card.can_manage || setOnDemand.isPending;

  const handleOnDemand = (enabled: boolean) => {
    setOnDemand.mutate(
      { wsId: card.workspace_id, enabled },
      {
        onSuccess: () => toast.success(enabled ? 'On-demand enabled' : 'On-demand disabled'),
        onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed to update'),
      },
    );
  };

  const handlePortal = () => {
    portal.mutate(card.workspace_id, {
      onSuccess: (res) => {
        if (res?.url) window.location.href = res.url;
        else toast.error('No portal URL returned');
      },
      onError: (e) => toast.error(e instanceof Error ? e.message : 'Failed to open portal'),
    });
  };

  // Primary CTA depends on state.
  const primaryCta = isPastDue
    ? { label: 'Update payment', onClick: handlePortal }
    : isFree || isTrial
      ? { label: 'Upgrade', onClick: () => onChangePlan(card) }
      : { label: 'Manage', onClick: () => onManage(card) };

  return (
    <div
      className={cn(
        'flex flex-col gap-4 rounded-xl border bg-card p-4 transition-shadow hover:shadow-sm',
        isPastDue && 'border-destructive/60',
      )}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate font-medium">{card.workspace_name}</div>
          {isTrial && card.trial_ends_at && (
            <div className="text-xs text-muted-foreground">
              Trial ends {formatDate(card.trial_ends_at)} → Free
            </div>
          )}
          {isPastDue && (
            <div className="text-xs font-medium text-destructive">
              Payment past due — update your card
            </div>
          )}
        </div>
        <Badge variant={statusBadgeVariant(card.status, isTrial)} className="shrink-0 capitalize">
          {isPastDue ? 'Past due' : planBadgeLabel(card.plan, isTrial)}
        </Badge>
      </div>

      {/* Credits */}
      <div>
        <div className="mb-1 flex items-center justify-between text-xs">
          <span className="text-muted-foreground">Credits</span>
          <span className="tabular-nums">
            {formatNumber(card.credits_used)} / {formatNumber(card.included_credits)} used
          </span>
        </div>
        <Progress value={creditsPct} className={cn(isPastDue && '[&>div]:bg-destructive')} />
      </div>

      {/* Plan */}
      <Row label="Plan">
        <span>
          {PLAN_LABEL[card.plan]} · {INTERVAL_LABEL[card.billing_interval]}
        </span>
        <span className="text-xs text-muted-foreground">
          {isTrial ? `ends ${formatDate(card.trial_ends_at)}` : `renews ${formatDate(card.current_period_end)}`}
        </span>
      </Row>

      {/* Billed to */}
      <Row label="Billed to">
        <span className="min-w-0 truncate">
          {card.payment_method ? (
            <span className="capitalize">
              {card.payment_method.brand} ···· {card.payment_method.last4}
            </span>
          ) : (
            <span className="text-muted-foreground">Organization card</span>
          )}
        </span>
        {card.can_manage && (
          <BilledToPopover orgId={orgId} card={card} cards={cards} onAddCard={onAddCard} />
        )}
      </Row>

      {/* Billing owner */}
      <Row label="Billing owner">
        <span className="min-w-0 truncate">
          {card.billing_owner?.name ?? <span className="text-muted-foreground">Unassigned</span>}
        </span>
        {canManageOwner && (
          <BillingOwnerPopover orgId={orgId} card={card} members={members} />
        )}
      </Row>

      {/* On-demand */}
      <Row label="On-demand credits">
        <span className="text-xs text-muted-foreground">
          {isFree || isTrial ? 'Available on paid plans' : 'Buy credits past your plan limit'}
        </span>
        <Switch
          checked={card.on_demand_enabled}
          disabled={onDemandDisabled}
          onCheckedChange={handleOnDemand}
        />
      </Row>

      {/* CTAs */}
      <div className="mt-auto flex items-center gap-2 pt-1">
        <Button
          className="flex-1"
          variant={isPastDue ? 'destructive' : 'default'}
          size="sm"
          onClick={primaryCta.onClick}
          disabled={!card.can_manage}
        >
          {primaryCta.label}
        </Button>
        {isPaid && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => onChangePlan(card)}
            disabled={!card.can_manage}
          >
            Change plan
          </Button>
        )}
        {!isPastDue && isPaid && (
          <Button variant="ghost" size="sm" onClick={() => onManage(card)}>
            Usage
          </Button>
        )}
      </div>
    </div>
  );
}
