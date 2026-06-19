import { useMemo } from 'react';
import { ExternalLink, Loader2 } from 'lucide-react';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { useBillingCheckout, useBillingPortal, useSetBillingOnDemand, useWorkspaceBilling } from '@/hooks/queries';
import type { BillingInterval, BillingPlan, WorkspaceBillingSummary } from '@/lib/types';
import { SettingsPageFrame } from './SettingsPageFrame';

const PLAN_OPTIONS: Array<{
  id: Exclude<BillingPlan, 'free'>;
  name: string;
  monthly: number;
  annual: number;
  credits: number;
}> = [
  { id: 'starter', name: 'Starter', monthly: 99, annual: 948, credits: 5_000 },
  { id: 'growth', name: 'Growth', monthly: 299, annual: 2_868, credits: 25_000 },
];

const PLAN_LABELS: Record<string, string> = {
  free: 'Free',
  starter: 'Starter',
  growth: 'Growth',
};

export function BillingSettingsPage() {
  return (
    <SettingsPageFrame section="billing">
      {({ workspaceId, permissions }) => (
        <BillingSettingsContent workspaceId={workspaceId} editable={permissions.canManageSettings} />
      )}
    </SettingsPageFrame>
  );
}

function BillingSettingsContent({ workspaceId, editable }: { workspaceId: string; editable: boolean }) {
  const { data: billing, isLoading } = useWorkspaceBilling(workspaceId);
  const checkout = useBillingCheckout(workspaceId);
  const portal = useBillingPortal(workspaceId);
  const setOnDemand = useSetBillingOnDemand(workspaceId);

  const creditPct = useMemo(() => {
    if (!billing?.included_credits) return 0;
    return Math.min(100, Math.round((billing.credits_used / billing.included_credits) * 100));
  }, [billing]);

  const startCheckout = async (plan: Exclude<BillingPlan, 'free'>, interval: BillingInterval) => {
    const result = await checkout.mutateAsync({ plan, interval, return_url: window.location.href }).catch((error) => {
      toast.error(error instanceof Error ? error.message : 'Could not start checkout');
      return null;
    });
    if (result?.url) window.location.href = result.url;
  };

  const openPortal = async () => {
    const result = await portal.mutateAsync(window.location.href).catch((error) => {
      toast.error(error instanceof Error ? error.message : 'Could not open billing portal');
      return null;
    });
    if (result?.url) window.location.href = result.url;
  };

  const toggleOnDemand = async (enabled: boolean) => {
    await setOnDemand.mutateAsync(enabled).then(
      () => toast.success(enabled ? 'On-demand credits enabled' : 'On-demand credits disabled'),
      (error) => toast.error(error instanceof Error ? error.message : 'Could not update on-demand credits'),
    );
  };

  if (isLoading || !billing) {
    return (
      <div className="space-y-4">
        <Skeleton className="h-36 w-full" />
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }

  return (
    <div className="space-y-5">
      <Card>
        <CardHeader className="pb-3">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <CardTitle className="text-base">Current plan</CardTitle>
              <CardDescription>{billingStatusCopy(billing)}</CardDescription>
            </div>
            <Badge variant={billing.status === 'past_due' ? 'destructive' : 'secondary'}>
              {PLAN_LABELS[billing.plan] ?? billing.plan}
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="space-y-5">
          <div className="grid gap-4 md:grid-cols-3">
            <Metric label="Billing period" value={periodCopy(billing)} />
            <Metric label="Included credits" value={formatNumber(billing.included_credits)} />
            <Metric label="On-demand blocks" value={formatNumber(billing.on_demand_blocks_invoiced)} />
          </div>
          <div className="space-y-2">
            <div className="flex items-center justify-between gap-3 text-sm">
              <span className="font-medium">AI credits</span>
              <span className="text-muted-foreground">
                {formatNumber(billing.credits_remaining)} remaining of {formatNumber(billing.included_credits)}
              </span>
            </div>
            <Progress value={creditPct} />
            <p className="text-xs text-muted-foreground">{formatNumber(billing.credits_used)} credits used this period.</p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button onClick={() => void openPortal()} disabled={!editable || !billing.manage_billing_enabled || portal.isPending}>
              {portal.isPending && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
              Manage billing
              <ExternalLink className="ml-2 h-4 w-4" />
            </Button>
          </div>
          {billing.warning && <p className="text-sm text-destructive">{billing.warning}</p>}
        </CardContent>
      </Card>

      <div className="grid gap-4 lg:grid-cols-2">
        {PLAN_OPTIONS.map((plan) => (
          <Card key={plan.id}>
            <CardHeader className="pb-3">
              <CardTitle className="text-base">{plan.name}</CardTitle>
              <CardDescription>{formatNumber(plan.credits)} AI credits per month</CardDescription>
            </CardHeader>
            <CardContent className="space-y-4">
              <div className="grid gap-3 sm:grid-cols-2">
                <PriceOption
                  label="Monthly"
                  price={`$${plan.monthly}/mo`}
                  disabled={!editable || checkout.isPending}
                  active={billing.plan === plan.id && billing.billing_interval === 'monthly'}
                  onClick={() => void startCheckout(plan.id, 'monthly')}
                />
                <PriceOption
                  label="Annual"
                  price={`$${plan.annual}/yr`}
                  disabled={!editable || checkout.isPending}
                  active={billing.plan === plan.id && billing.billing_interval === 'annual'}
                  onClick={() => void startCheckout(plan.id, 'annual')}
                />
              </div>
            </CardContent>
          </Card>
        ))}
      </div>

      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="text-base">On-demand credits</CardTitle>
          <CardDescription>$50 per 5,000-credit block for paid plans.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between gap-4">
            <div className="space-y-1">
              <p className="text-sm font-medium">{billing.on_demand_enabled ? 'Enabled' : 'Disabled'}</p>
              <p className="text-sm text-muted-foreground">
                {billing.on_demand_available
                  ? 'Extra blocks are invoiced through Stripe when included credits run out.'
                  : 'Upgrade to Starter or Growth to enable additional credit blocks.'}
              </p>
            </div>
            <Switch
              checked={billing.on_demand_enabled}
              onCheckedChange={(checked) => void toggleOnDemand(checked)}
              disabled={!editable || !billing.on_demand_available || setOnDemand.isPending}
            />
          </div>
        </CardContent>
      </Card>
    </div>
  );
}

function Metric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-sm font-medium">{value}</p>
    </div>
  );
}

function PriceOption({
  label,
  price,
  active,
  disabled,
  onClick,
}: {
  label: string;
  price: string;
  active: boolean;
  disabled: boolean;
  onClick: () => void;
}) {
  return (
    <Button variant={active ? 'default' : 'outline'} className="h-auto justify-between py-3" onClick={onClick} disabled={disabled}>
      <span>{label}</span>
      <span className="font-semibold">{price}</span>
    </Button>
  );
}

function billingStatusCopy(billing: WorkspaceBillingSummary): string {
  if (billing.trialing && billing.trial_ends_at) {
    return `Growth trial ends ${formatDate(billing.trial_ends_at)}`;
  }
  if (billing.status === 'past_due') return 'Payment requires attention';
  if (billing.plan === 'free') return 'Free plan with monthly AI credits';
  return `${billing.billing_interval === 'annual' ? 'Annual' : 'Monthly'} subscription`;
}

function periodCopy(billing: WorkspaceBillingSummary): string {
  if (billing.current_period_end) return `Renews ${formatDate(billing.current_period_end)}`;
  return 'Monthly';
}

function formatDate(value: string): string {
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(value));
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value);
}
