import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Progress } from '@/components/ui/progress';
import { Switch } from '@/components/ui/switch';
import { cn } from '@/lib/utils';
import type { PaymentMethod, WorkspaceBillingCard as WSCard } from '@/ee/lib/billingTypes';
import {
  INTERVAL_LABEL,
  PLAN_LABEL,
  formatDate,
  planBadgeLabel,
  statusBadgeVariant,
} from '@/ee/lib/billingUtils';
import { formatAIUsagePercent } from '@/lib/aiUsage';
import { useSetOnDemand, useBillingPortal } from '@/ee/hooks/queries/useBilling';
import { BilledToPopover } from './BilledToPopover';
import { getWorkspaceBillingCardPresentation } from './workspaceBillingCardPresentation';

interface Props {
  orgId: string;
  card: WSCard;
  cards: PaymentMethod[];
  onManage: (card: WSCard) => void;
  onChangePlan: (card: WSCard) => void;
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
  onManage,
  onChangePlan,
}: Props) {
  const setOnDemand = useSetOnDemand(orgId);
  const portal = useBillingPortal();
  const view = getWorkspaceBillingCardPresentation(card);

  const isTrial = view.isTrial;
  const isFounderPlan = view.isFounderPlan;
  const isLocked = view.isLocked;
  const isPaymentIssue = view.isPaymentIssue;
  const isActivePaid = view.isActivePaid;

  const usagePct = card.ai_usage_percent ?? 0;

  const onDemandDisabled = isFounderPlan || isLocked || isTrial || !card.can_manage || setOnDemand.isPending || !card.extra_ai_usage_available;

  const handleOnDemand = (enabled: boolean) => {
    setOnDemand.mutate(
      { wsId: card.workspace_id, enabled },
      {
        onSuccess: () => toast.success(enabled ? 'Extra AI usage enabled' : 'Extra AI usage disabled'),
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
  const handlePrimaryCta = () => {
    if (view.primaryCtaAction === 'portal') {
      handlePortal();
    } else if (view.primaryCtaAction === 'upgrade') {
      onChangePlan(card);
    } else {
      onManage(card);
    }
  };

  return (
    <div
      className={cn(
        'flex flex-col gap-4 rounded-xl border bg-card p-4 transition-shadow hover:shadow-sm',
        isPaymentIssue && 'border-destructive/60',
      )}
    >
      {/* Header */}
      <div className="flex items-start justify-between gap-2">
        <div className="min-w-0">
          <div className="truncate font-medium">{card.workspace_name}</div>
          {isTrial && card.trial_ends_at && (
            <div className="text-xs text-muted-foreground">
              Trial ends {formatDate(card.trial_ends_at)}
            </div>
          )}
          {card.status === 'trial_expired' && (
            <div className="text-xs font-medium text-destructive">
              Trial ended — choose a plan to reactivate
            </div>
          )}
          {card.status === 'canceled' && (
            <div className="text-xs font-medium text-destructive">
              Subscription ended — choose a plan to reactivate
            </div>
          )}
          {card.status === 'unpaid' && (
            <div className="text-xs font-medium text-destructive">
              Payment overdue — update payment to reactivate
            </div>
          )}
          {isPaymentIssue && (
            <div className="text-xs font-medium text-destructive">
              Payment past due — update your card
            </div>
          )}
        </div>
        <Badge variant={statusBadgeVariant(card.status, isTrial)} className="shrink-0 capitalize">
          {isPaymentIssue ? 'Payment issue' : isLocked ? 'Locked' : planBadgeLabel(card.plan, isTrial)}
        </Badge>
      </div>

      {/* AI usage */}
      <div>
        <div className="mb-1 flex items-center justify-between text-xs">
          <span className="text-muted-foreground">AI usage</span>
          <span className="tabular-nums">
            {formatAIUsagePercent(usagePct)} used
          </span>
        </div>
        <Progress value={usagePct} className={cn(isPaymentIssue && '[&>div]:bg-destructive')} />
      </div>

      {/* Plan */}
      <Row label="Plan">
        <span>
          {isFounderPlan ? 'Founder plan' : `${PLAN_LABEL[card.plan]} · ${INTERVAL_LABEL[card.billing_interval]}`}
        </span>
        <span className="text-xs text-muted-foreground">
          {isFounderPlan ? `resets ${formatDate(card.current_period_end)}` : isTrial ? `ends ${formatDate(card.trial_ends_at)}` : `renews ${formatDate(card.current_period_end)}`}
        </span>
      </Row>

      {/* Billed to */}
      <Row label="Billed to">
        <span className="min-w-0 truncate">
          <span className={cn(card.payment_method && 'capitalize', !card.payment_method && 'text-muted-foreground')}>
            {view.billedToLabel}
          </span>
        </span>
        {view.showBilledToChange && (
          <BilledToPopover orgId={orgId} card={card} cards={cards} onAddCard={handlePortal} />
        )}
      </Row>

      {/* Extra AI usage */}
      <Row label="Extra AI usage">
        <span className="text-xs text-muted-foreground">
          {view.extraUsageLabel}
        </span>
        <Switch
          checked={card.extra_ai_usage_enabled}
          disabled={onDemandDisabled}
          onCheckedChange={handleOnDemand}
        />
      </Row>

      {/* CTAs */}
      <div className="mt-auto flex items-center gap-2 pt-1">
        <Button
          className="flex-1"
          variant={isPaymentIssue ? 'destructive' : 'default'}
          size="sm"
          onClick={handlePrimaryCta}
          disabled={!card.can_manage}
        >
          {view.primaryCtaLabel}
        </Button>
        {isActivePaid && (
          <Button
            variant="outline"
            size="sm"
            onClick={() => onChangePlan(card)}
            disabled={!card.can_manage}
          >
            Change plan
          </Button>
        )}
        {isActivePaid && (
          <Button variant="ghost" size="sm" onClick={() => onManage(card)}>
            Usage
          </Button>
        )}
      </div>
    </div>
  );
}
