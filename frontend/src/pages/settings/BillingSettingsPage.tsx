import { useEffect, useMemo, useState } from 'react';
import {
  Ban,
  BarChart3,
  CalendarClock,
  Check,
  CreditCard,
  CircleAlert,
  ExternalLink,
  FileText,
  Layers,
  Loader2,
  MapPin,
  MessageCircle,
  ReceiptText,
  X,
} from 'lucide-react';
import { toast } from 'sonner';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogMedia,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { Switch } from '@/components/ui/switch';
import { UsageDetail } from '@/components/billing/UsageDetail';
import { useBillingCheckout, useBillingPlanChange, useBillingPlanChangePreview, useBillingPortal, useConfirmBillingCheckout, useResumeBillingSubscription, useSetBillingOnDemand, useWorkspaceBilling } from '@/hooks/queries';
import type { PlanChangePreview } from '@/lib/billingTypes';
import type { BillingInterval, BillingPlan, WorkspaceBillingSummary } from '@/lib/types';
import { SettingsPageFrame } from './SettingsPageFrame';

const PLAN_OPTIONS: Array<{
  id: Exclude<BillingPlan, 'free'>;
  name: string;
  monthly: number;
  annual: number;
  credits: number;
  description: string;
  features: string[];
  popular?: boolean;
}> = [
  {
    id: 'starter',
    name: 'Starter',
    monthly: 99,
    annual: 948,
    credits: 5_000,
    description: 'For teams that need the full platform without limits on work.',
    features: [
      'Everything in Free',
      'Basic automations / built-in agents',
      'Tasks & inbox custom views',
      'Team inboxes',
      'Connect email channels',
      'GitHub integration',
      'Custom help center domain',
    ],
  },
  {
    id: 'growth',
    name: 'Growth',
    monthly: 299,
    annual: 2_868,
    credits: 25_000,
    description: 'For growing teams that want custom AI agents and full control.',
    popular: true,
    features: [
      'Everything in Starter',
      'Unlimited deals in CRM',
      'Multilingual help center',
      'AI article translation',
      'Custom AI agents',
      'Advanced automations',
      'Agent scheduling & cron',
      'Remove Helpin branding',
      'Priority support',
    ],
  },
];

type PaidPlanOption = (typeof PLAN_OPTIONS)[number];

const FREE_PLAN = {
  name: 'Free',
  credits: 1_000,
  description: 'For solo founders and small projects getting started.',
  features: [
    'All modules: PM, Support, Sales, Docs',
    'Tasks, epics & board views',
    'Live chat widget',
    'Shared inbox',
    'CRM with contacts & deals',
    'Public help center',
    'Built-in AI agents, limited',
    'Import from other tools',
  ],
};

const COMPARISON_FEATURES: Array<{
  name: string;
  category?: boolean;
  free?: boolean | string;
  starter?: boolean | string;
  growth?: boolean | string;
}> = [
  { name: 'Users & Access', category: true },
  { name: 'Users', free: '2', starter: 'Unlimited', growth: 'Unlimited' },
  { name: 'Mobile access', free: true, starter: true, growth: true },
  { name: 'Project Management', category: true },
  { name: 'Tasks & stories', free: true, starter: true, growth: true },
  { name: 'Epics', free: true, starter: true, growth: true },
  { name: 'Objectives', free: true, starter: true, growth: true },
  { name: 'Sprints', free: true, starter: true, growth: true },
  { name: 'Custom fields', free: false, starter: true, growth: true },
  { name: 'Tasks custom views', free: false, starter: true, growth: true },
  { name: 'Roadmap', free: false, starter: true, growth: true },
  { name: 'Support', category: true },
  { name: 'Live chat widget', free: true, starter: true, growth: true },
  { name: 'Shared inbox', free: true, starter: true, growth: true },
  { name: 'Team inboxes', free: false, starter: true, growth: true },
  { name: 'Inbox custom views', free: false, starter: true, growth: true },
  { name: 'Inbox saved replies', free: false, starter: true, growth: true },
  { name: 'Email forwarding', free: false, starter: true, growth: true },
  { name: 'Sender addresses', free: false, starter: true, growth: true },
  { name: 'Round robin assignment', free: false, starter: false, growth: true },
  { name: 'SLA policies', free: false, starter: false, growth: true },
  { name: 'AI conversation routing', free: false, starter: false, growth: true },
  { name: 'Coverage gap detection', free: false, starter: false, growth: true },
  { name: 'Sales / CRM', category: true },
  { name: 'Contacts', free: true, starter: true, growth: true },
  { name: 'Deals', free: true, starter: true, growth: 'Unlimited' },
  { name: 'Gmail sync', free: false, starter: true, growth: true },
  { name: 'Buyer signal detection', free: false, starter: true, growth: true },
  { name: 'Deal automation', free: false, starter: false, growth: true },
  { name: 'Docs / Knowledge', category: true },
  { name: 'Documents', free: '200', starter: '1,000', growth: 'Unlimited' },
  { name: 'Internal docs', free: true, starter: true, growth: true },
  { name: 'Public help center', free: true, starter: true, growth: true },
  { name: 'Custom domain', free: false, starter: true, growth: true },
  { name: 'Help center redirects', free: false, starter: true, growth: true },
  { name: 'Multilingual help center', free: false, starter: false, growth: true },
  { name: 'AI article translation', free: false, starter: false, growth: true },
  { name: 'AI Agents', category: true },
  { name: 'AI credits/month', free: '1,000', starter: '5,000', growth: '25,000' },
  { name: 'Built-in agents', free: 'Limited', starter: true, growth: true },
  { name: 'Custom agents', free: false, starter: false, growth: true },
  { name: 'Agent scheduling', free: false, starter: false, growth: true },
  { name: 'On-demand credit blocks', free: false, starter: '$50 / 5,000', growth: '$50 / 5,000' },
  { name: 'Platform', category: true },
  { name: 'GitHub integration', free: false, starter: true, growth: true },
  { name: 'Basic automations', free: false, starter: true, growth: true },
  { name: 'Advanced automations', free: false, starter: false, growth: true },
  { name: 'Branding & Support', category: true },
  { name: 'Widget branding', free: 'Helpin', starter: 'Helpin', growth: 'Removed' },
  { name: 'Module access controls', free: true, starter: true, growth: true },
  { name: 'Priority support', free: false, starter: false, growth: true },
];

const INCLUDED_MODULES = [
  { label: 'Project Management', sub: 'Plan, build, ship', Icon: Layers, color: 'oklch(0.52 0.16 250)', bg: 'oklch(0.52 0.16 250 / 0.08)' },
  { label: 'Support', sub: 'Triage, resolve, notify', Icon: MessageCircle, color: 'oklch(0.58 0.15 55)', bg: 'oklch(0.58 0.15 55 / 0.08)' },
  { label: 'Sales / CRM', sub: 'Track, follow up, close', Icon: BarChart3, color: 'oklch(0.52 0.14 28)', bg: 'oklch(0.52 0.14 28 / 0.08)' },
  { label: 'Docs', sub: 'Internal, help center', Icon: FileText, color: 'oklch(0.55 0.16 160)', bg: 'oklch(0.55 0.16 160 / 0.08)' },
];

const BILLING_FAQS = [
  {
    q: 'Do I need to buy modules separately?',
    a: 'No. Every plan includes PM, Support, Sales, and Docs. Paid plans unlock higher limits and advanced AI automation.',
  },
  {
    q: 'What happens if I exceed my AI credit limit?',
    a: 'Free workspaces pause AI usage until the next monthly refresh. Paid workspaces can enable on-demand credits at $50 per 5,000-credit block.',
  },
  {
    q: 'Can I switch plans anytime?',
    a: 'Yes. Paid plan and interval changes apply immediately with prorated billing. Moving to Free is scheduled for your next renewal date.',
  },
  {
    q: 'Do unused credits roll over?',
    a: 'No. Included AI credits reset each billing period so usage stays predictable.',
  },
  {
    q: 'Are seats really unlimited?',
    a: 'Starter and Growth include unlimited seats. Free workspaces include 2 seats.',
  },
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
  const [choosingPlan, setChoosingPlan] = useState(false);
  const [selectedInterval, setSelectedInterval] = useState<BillingInterval>('annual');
  const [checkoutResult, setCheckoutResult] = useState<'success' | 'cancelled' | null>(null);
  const [checkoutConfirming, setCheckoutConfirming] = useState(false);
  const [checkoutConfirmationDelayed, setCheckoutConfirmationDelayed] = useState(false);
  const [cancelConfirmOpen, setCancelConfirmOpen] = useState(false);
  const [planPreview, setPlanPreview] = useState<PlanChangePreview | null>(null);
  const [pendingPlanAction, setPendingPlanAction] = useState<string | null>(null);
  const { data: billing, isLoading, refetch } = useWorkspaceBilling(workspaceId);
  const checkout = useBillingCheckout(workspaceId);
  const confirmCheckout = useConfirmBillingCheckout(workspaceId);
  const previewPlanChange = useBillingPlanChangePreview(workspaceId);
  const changePlan = useBillingPlanChange(workspaceId);
  const resumeSubscription = useResumeBillingSubscription(workspaceId);
  const portal = useBillingPortal(workspaceId);
  const setOnDemand = useSetBillingOnDemand(workspaceId);

  useEffect(() => {
    const url = new URL(window.location.href);
    const result = url.searchParams.get('billing');
    const checkoutSessionID = url.searchParams.get('checkout_session_id');
    if (result !== 'success' && result !== 'cancelled' && result !== 'canceled') return;

    const normalized = result === 'success' ? 'success' : 'cancelled';
    url.searchParams.delete('billing');
    url.searchParams.delete('checkout_session_id');
    window.history.replaceState({}, '', `${url.pathname}${url.search}${url.hash}`);

    if (normalized === 'success') {
      if (checkoutSessionID) {
        void confirmCheckout.mutateAsync({ session_id: checkoutSessionID }).then(
          () => {
            setCheckoutResult(null);
            setCheckoutConfirming(false);
            setCheckoutConfirmationDelayed(false);
            setChoosingPlan(false);
            toast.success('Plan updated.');
          },
          () => {
            setCheckoutResult('success');
            setCheckoutConfirming(true);
            setCheckoutConfirmationDelayed(false);
            toast.info('Payment complete. Syncing your plan.');
            void refetch();
          },
        );
      } else {
        setCheckoutResult('success');
        setCheckoutConfirming(true);
        setCheckoutConfirmationDelayed(false);
        toast.info('Payment complete. Syncing your plan.');
        void refetch();
      }
    } else {
      setCheckoutResult(normalized);
      setCheckoutConfirming(false);
      setCheckoutConfirmationDelayed(false);
      toast.info('Checkout was cancelled. No billing changes were made.');
    }
  }, [confirmCheckout, refetch]);

  useEffect(() => {
    if (checkoutResult !== 'success' || !checkoutConfirming) return;
    if (billing?.stripe_subscription_id || billing?.manage_billing_enabled) {
      setCheckoutConfirming(false);
      setCheckoutResult(null);
      toast.success('Billing updated.');
      return;
    }

    let attempts = 0;
    const interval = window.setInterval(() => {
      attempts += 1;
      void refetch().then((result) => {
        const next = result.data;
        if (next?.stripe_subscription_id || next?.manage_billing_enabled) {
          window.clearInterval(interval);
          setCheckoutConfirming(false);
          setCheckoutResult(null);
          toast.success('Billing updated.');
          return;
        }
        if (attempts >= 20) {
          window.clearInterval(interval);
          setCheckoutConfirming(false);
          setCheckoutConfirmationDelayed(true);
        }
      });
    }, 3000);

    return () => window.clearInterval(interval);
  }, [billing?.manage_billing_enabled, billing?.stripe_subscription_id, checkoutConfirming, checkoutResult, refetch]);

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

  const startPlanChange = async (plan: BillingPlan, interval: BillingInterval, prorationDate?: number) => {
    if (!billing) return;
    if (billing.plan === 'free' || !billing.stripe_subscription_id) {
      if (plan !== 'free') {
        await startCheckout(plan, interval);
      }
      return;
    }
    const result = await changePlan.mutateAsync({ plan, interval, proration_date: prorationDate }).catch((error) => {
      toast.error(error instanceof Error ? error.message : 'Could not change plan');
      return null;
    });
    if (!result) return;
    if (result.pending_plan) {
      toast.success(result.pending_plan === 'free' ? 'Cancellation scheduled' : 'Plan change scheduled');
    } else {
      toast.success('Plan updated');
    }
    setChoosingPlan(false);
  };

  const beginPaidPlanChange = async (plan: Exclude<BillingPlan, 'free'>, interval: BillingInterval) => {
    const actionKey = `${plan}:${interval}`;
    setPendingPlanAction(actionKey);
    if (!billing || billing.plan === 'free' || !billing.stripe_subscription_id) {
      await startPlanChange(plan, interval).finally(() => setPendingPlanAction(null));
      return;
    }
    if (billing.plan === plan && billing.billing_interval === interval && !billing.pending_plan) {
      setPendingPlanAction(null);
      return;
    }
    const preview = await previewPlanChange.mutateAsync({ plan, interval }).catch((error) => {
      toast.error(error instanceof Error ? error.message : 'Could not preview plan change');
      return null;
    });
    if (preview) setPlanPreview(preview);
    setPendingPlanAction(null);
  };

  const openPortal = async (action: string) => {
    const result = await portal.mutateAsync(window.location.href).catch((error) => {
      toast.error(error instanceof Error ? error.message : `Could not open ${action}`);
      return null;
    });
    if (result?.url) {
      window.open(result.url, '_blank', 'noopener,noreferrer');
    }
  };

  const toggleOnDemand = async (enabled: boolean) => {
    await setOnDemand.mutateAsync(enabled).then(
      () => toast.success(enabled ? 'On-demand credits enabled' : 'On-demand credits disabled'),
      (error) => toast.error(error instanceof Error ? error.message : 'Could not update on-demand credits'),
    );
  };

  if (isLoading || !billing) {
    return (
      <div className="mx-auto max-w-3xl space-y-4">
        <Skeleton className="h-32 w-full" />
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-80 w-full" />
      </div>
    );
  }

  const canUsePortal = editable && billing.manage_billing_enabled;
  const planLabel = billingOverviewPlanTitle(billing);
  const planStatusCopy = billingStatusCopy(billing);
  const isPaid = billing.plan !== 'free';
  const annualNudge = annualBillingNudge(billing);

  if (choosingPlan) {
    return (
      <div className="mx-auto max-w-6xl space-y-8 pt-8">
        <div className="flex flex-col gap-4 pt-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="space-y-1">
            <h3 className="text-xl font-semibold tracking-normal">Choose a plan</h3>
            <p className="text-sm text-muted-foreground">
              Compare Free, Starter, and Growth to choose the right workspace plan.
            </p>
          </div>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="w-fit shrink-0"
            onClick={() => setChoosingPlan(false)}
          >
            Back to overview
          </Button>
        </div>

        <CheckoutReturnNotice result={checkoutResult} confirming={checkoutConfirming} delayed={checkoutConfirmationDelayed} onRefresh={() => void refetch()} />
        <BillingNoticeBanner billing={billing} onPortal={() => void openPortal('payment method management')} portalLoading={portal.isPending} />
        <PendingBillingNotice
          billing={billing}
          editable={editable}
          resumeLoading={resumeSubscription.isPending}
          onResume={() => {
            void resumeSubscription.mutateAsync().then(
              () => toast.success('Subscription resumed.'),
              (error) => toast.error(error instanceof Error ? error.message : 'Could not resume subscription'),
            );
          }}
        />
        <EntitlementNotice billing={billing} />
        <EveryPlanIncludes />

        <div className="flex justify-center">
          <div className="grid grid-cols-2 rounded-lg border bg-muted/30 p-1 shadow-sm">
            <Button
              type="button"
              size="default"
              variant="ghost"
              className={`min-w-32 font-semibold ${
                selectedInterval === 'monthly'
                  ? 'border border-border bg-background text-foreground shadow-sm hover:bg-background'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
              onClick={() => setSelectedInterval('monthly')}
            >
              Monthly
            </Button>
            <Button
              type="button"
              size="default"
              variant="ghost"
              className={`min-w-40 gap-2 font-semibold ${
                selectedInterval === 'annual'
                  ? 'border border-border bg-background text-foreground shadow-sm hover:bg-background'
                  : 'text-muted-foreground hover:text-foreground'
              }`}
              onClick={() => setSelectedInterval('annual')}
            >
              Annual
              <span
                className={`rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                  selectedInterval === 'annual'
                    ? 'bg-primary/10 text-primary'
                    : 'bg-muted text-muted-foreground'
                }`}
              >
                Save 20%
              </span>
            </Button>
          </div>
        </div>

        <div className="grid gap-3 lg:grid-cols-3">
          <FreePlanCard
            billing={billing}
            interval={selectedInterval}
            editable={editable}
            loading={changePlan.isPending}
            onSelect={() => setCancelConfirmOpen(true)}
          />
          {PLAN_OPTIONS.map((plan) => (
            <PlanChoiceCard
              key={plan.id}
              plan={plan}
              interval={selectedInterval}
              billing={billing}
              editable={editable}
              disabled={checkout.isPending || changePlan.isPending || previewPlanChange.isPending}
              loading={pendingPlanAction === `${plan.id}:${selectedInterval}`}
              onSelect={() => void beginPaidPlanChange(plan.id, selectedInterval)}
            />
          ))}
        </div>

        <PlanComparison />
        <BillingFAQs />
        <CancelSubscriptionDialog
          billing={billing}
          open={cancelConfirmOpen}
          loading={changePlan.isPending}
          onOpenChange={setCancelConfirmOpen}
          onConfirm={() => {
            void startPlanChange('free', 'monthly').finally(() => setCancelConfirmOpen(false));
          }}
        />
        <PlanChangePreviewDialog
          preview={planPreview}
          loading={changePlan.isPending}
          onOpenChange={(open) => {
            if (!open && !changePlan.isPending) setPlanPreview(null);
          }}
          onConfirm={() => {
            if (!planPreview) return;
            void startPlanChange(planPreview.target_plan, planPreview.target_interval, planPreview.proration_date)
              .then(() => setPlanPreview(null));
          }}
        />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl space-y-8">
      <section className="space-y-3">
        <CheckoutReturnNotice result={checkoutResult} confirming={checkoutConfirming} delayed={checkoutConfirmationDelayed} onRefresh={() => void refetch()} />
        <Card className="overflow-hidden">
          <CardContent className="p-0">
            <div className="flex flex-col gap-5 p-5 sm:flex-row sm:items-center sm:justify-between">
              <div className="min-w-0 space-y-1">
                <div className="flex flex-wrap items-center gap-2">
                  <h3 className="text-2xl font-semibold tracking-normal">{planLabel}</h3>
                  {billing.plan !== 'free' && (
                    <Badge
                      variant={billing.status === 'past_due' ? 'destructive' : billing.trialing ? 'secondary' : 'outline'}
                      className={statusBadgeClassName(billing)}
                    >
                      {statusLabel(billing)}
                    </Badge>
                  )}
                </div>
                {planStatusCopy && <p className="text-sm text-muted-foreground">{planStatusCopy}</p>}
              </div>
              {isPaid ? (
                <Button
                  onClick={() => setChoosingPlan(true)}
                  disabled={!editable}
                  className="shrink-0"
                >
                  Change plan
                </Button>
              ) : (
                <Button
                  onClick={() => setChoosingPlan(true)}
                  disabled={!editable}
                  className="shrink-0"
                >
                  Choose plan
                </Button>
              )}
            </div>

            <div className="grid border-t bg-muted/20 sm:grid-cols-3">
              <PlanMetric label="Billing period" value={periodCopy(billing)} />
              <PlanMetric label={nextChargeMetricLabel(billing)} value={nextChargeMetricValue(billing)} />
              <PlanMetric label="Included credits" value={`${formatNumber(billing.included_credits)} / month`} />
            </div>
          </CardContent>
        </Card>
        {annualNudge && (
          <AnnualBillingNudge
            nudge={annualNudge}
            editable={editable}
            loading={changePlan.isPending}
            onSwitch={() => {
              if (billing.plan !== 'free') void startPlanChange(billing.plan, 'annual');
            }}
          />
        )}
        <BillingNoticeBanner billing={billing} onPortal={() => void openPortal('payment method management')} portalLoading={portal.isPending} />
        <PendingBillingNotice
          billing={billing}
          editable={editable}
          resumeLoading={resumeSubscription.isPending}
          onResume={() => {
            void resumeSubscription.mutateAsync().then(
              () => toast.success('Subscription resumed.'),
              (error) => toast.error(error instanceof Error ? error.message : 'Could not resume subscription'),
            );
          }}
        />
        <EntitlementNotice billing={billing} />
        {billing.warning && <p className="text-sm text-destructive">{billing.warning}</p>}
      </section>

      <section className="space-y-3">
        <SectionHeading
          title="On-demand credits"
          description="Allow extra AI credit blocks when included credits run out."
        />
        <Card>
          <CardContent className="p-5">
            <div className="flex items-center justify-between gap-4">
              <div className="space-y-1">
                <p className="text-sm font-medium">{billing.on_demand_enabled ? 'Enabled' : 'Disabled'}</p>
                <p className="text-sm text-muted-foreground">
                  {billing.on_demand_available
                    ? '$50 per 5,000-credit block, added to your next invoice.'
                    : 'Available on Starter and Growth workspaces with an active subscription.'}
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
      </section>

      <section className="space-y-3">
        <SectionHeading
          title="Credit usage"
          description={`Current billing period resets on ${formatDate(billing.current_period_end)}.`}
        />
        <Card>
          <CardContent className="space-y-5 p-5">
            <div className="space-y-2">
              <div className="flex items-center justify-between gap-3 text-sm">
                <span className="font-medium">Included AI credits</span>
                <span className="text-muted-foreground">
                  {formatNumber(billing.credits_remaining)} remaining
                </span>
              </div>
              <Progress value={creditPct} />
            </div>
            <UsageDetail
              workspaceId={workspaceId}
              periodStart={billing.current_period_start}
              periodEnd={billing.current_period_end}
            />
          </CardContent>
        </Card>
      </section>

      <section className="space-y-3">
        <SectionHeading
          title="Billing management"
          description="Payment method, billing address, invoices, and cancellation are handled securely in Stripe."
        />
        <Card>
          <CardContent className="divide-y p-0">
            <PortalAction
              icon={CreditCard}
              title="Change payment method"
              description="Update the card or payment method used for this workspace."
              disabled={!canUsePortal || portal.isPending}
              loading={portal.isPending}
              onClick={() => void openPortal('payment method management')}
            />
            <PortalAction
              icon={MapPin}
              title="Update address details"
              description="Edit billing address, tax information, and receipt details."
              disabled={!canUsePortal || portal.isPending}
              loading={portal.isPending}
              onClick={() => void openPortal('billing details')}
            />
            <PortalAction
              icon={ReceiptText}
              title="View invoices"
              description="Open Stripe to review receipts, invoices, and payment history."
              disabled={!canUsePortal || portal.isPending}
              loading={portal.isPending}
              onClick={() => void openPortal('invoices')}
            />
            <PortalAction
              icon={Ban}
              title="Cancel subscription"
              description="Manage cancellation for the current workspace subscription."
              disabled={!canUsePortal || !isPaid || portal.isPending}
              loading={portal.isPending}
              danger
              onClick={() => void openPortal('subscription cancellation')}
            />
          </CardContent>
        </Card>
      </section>
    </div>
  );
}

function SectionHeading({
  title,
  description,
  align = 'left',
  size = 'sm',
}: {
  title: string;
  description: string;
  align?: 'left' | 'center';
  size?: 'sm' | 'lg';
}) {
  return (
    <div className={align === 'center' ? 'text-center' : undefined}>
      <h3 className={size === 'lg' ? 'text-xl font-semibold tracking-normal' : 'text-sm font-semibold'}>{title}</h3>
      <p className={size === 'lg' ? 'mt-2 text-sm text-muted-foreground' : 'mt-1 text-sm text-muted-foreground'}>{description}</p>
    </div>
  );
}

function PlanMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="border-t px-5 py-4 first:border-t-0 sm:border-l sm:border-t-0 sm:first:border-l-0">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 text-sm font-medium">{value}</p>
    </div>
  );
}

type AnnualBillingNudgeCopy = {
  planName: string;
  monthlyTotal: number;
  annualTotal: number;
  savings: number;
  monthlyEquivalent: number;
};

function AnnualBillingNudge({
  nudge,
  editable,
  loading,
  onSwitch,
}: {
  nudge: AnnualBillingNudgeCopy;
  editable: boolean;
  loading: boolean;
  onSwitch: () => void;
}) {
  return (
    <div className="rounded-md border border-primary/20 bg-primary/[0.04] px-4 py-4">
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex min-w-0 items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary/10 text-primary">
            <CalendarClock className="h-4 w-4" />
          </div>
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <p className="text-sm font-semibold">Save ${formatNumber(nudge.savings)}/year with annual billing</p>
              <Badge className="bg-primary text-primary-foreground">20% off</Badge>
            </div>
            <p className="mt-1 text-sm text-muted-foreground">
              Switch {nudge.planName} to annual for ${formatNumber(nudge.annualTotal)}/year instead of ${formatNumber(nudge.monthlyTotal)}/year, equal to ${formatNumber(nudge.monthlyEquivalent)}/month.
            </p>
            {!editable && (
              <p className="mt-2 text-xs text-muted-foreground">Ask a workspace owner or admin to switch billing.</p>
            )}
          </div>
        </div>
        {editable && (
          <Button type="button" className="shrink-0" onClick={onSwitch} disabled={loading}>
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Switch to annual
          </Button>
        )}
      </div>
    </div>
  );
}

function BillingNoticeBanner({
  billing,
  onPortal,
  portalLoading,
}: {
  billing: WorkspaceBillingSummary;
  onPortal: () => void;
  portalLoading: boolean;
}) {
  if (billing.billing_notice_type === 'payment_failed' || billing.status === 'past_due') {
    return (
      <div className="flex flex-col gap-3 rounded-md border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900 dark:border-red-500/40 dark:bg-red-500/10 dark:text-red-200 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-start gap-3">
          <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
          <div>
            <p className="font-medium">Payment needs attention</p>
            <p className="mt-1">
              {billing.billing_notice_message || 'Payment failed. Update your payment method to keep this workspace active.'}
            </p>
          </div>
        </div>
        <Button size="sm" variant="destructive" onClick={onPortal} disabled={portalLoading || !billing.manage_billing_enabled} className="shrink-0">
          {portalLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          Update payment
        </Button>
      </div>
    );
  }

  const trialEnd = billing.trial_will_end_at || billing.trial_ends_at;
  if (billing.billing_notice_type === 'trial_will_end' && trialEnd) {
    return (
      <div className="flex items-start gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200">
        <CalendarClock className="mt-0.5 h-4 w-4 shrink-0" />
        <div>
          <p className="font-medium">Trial ending soon</p>
          <p className="mt-1">Your Growth trial ends on {formatDate(trialEnd)}. Choose a plan to keep Growth limits active.</p>
        </div>
      </div>
    );
  }

  return null;
}

function CheckoutReturnNotice({
  result,
  confirming = false,
  delayed = false,
  onRefresh,
}: {
  result: 'success' | 'cancelled' | null;
  confirming?: boolean;
  delayed?: boolean;
  onRefresh?: () => void;
}) {
  if (!result) return null;
  if (result === 'success') {
    return (
      <div className="flex items-start gap-3 rounded-md border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900 dark:border-emerald-500/40 dark:bg-emerald-500/10 dark:text-emerald-200">
        <Check className="mt-0.5 h-4 w-4 shrink-0" />
        <div className="min-w-0 flex-1">
          <p className="font-medium">{delayed ? 'Still syncing your plan' : 'Payment complete'}</p>
          <p className="mt-1">
            {delayed
              ? 'Your payment succeeded, but the workspace has not reflected the plan yet. Refresh now or check again in a moment.'
              : confirming
                ? 'Syncing your workspace plan...'
                : 'Your workspace plan is being synced.'}
          </p>
        </div>
        {delayed && onRefresh && (
          <Button type="button" size="sm" variant="outline" className="shrink-0" onClick={onRefresh}>
            Refresh
          </Button>
        )}
      </div>
    );
  }
  return (
    <div className="flex items-start gap-3 rounded-md border px-4 py-3 text-sm">
      <CircleAlert className="mt-0.5 h-4 w-4 shrink-0 text-muted-foreground" />
      <div>
        <p className="font-medium">Checkout cancelled</p>
        <p className="mt-1 text-muted-foreground">No billing changes were made.</p>
      </div>
    </div>
  );
}

function EntitlementNotice({ billing }: { billing: WorkspaceBillingSummary }) {
  if (!billing.seat_over_limit && !billing.entitlement_warning) return null;
  return (
    <div className="flex items-start gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200">
      <CircleAlert className="mt-0.5 h-4 w-4 shrink-0" />
      <div>
        <p className="font-medium">Workspace is over the Free limit</p>
        <p className="mt-1">
          {billing.entitlement_warning ||
            `This workspace is over the Free plan seat limit: ${billing.seat_usage ?? 0} seats used, ${billing.seat_limit ?? 2} included. Remove members or upgrade to invite more people.`}
        </p>
      </div>
    </div>
  );
}

function EveryPlanIncludes() {
  return (
    <Card>
      <CardHeader className="pb-3 text-center">
        <CardTitle className="text-base">Every plan includes all core modules</CardTitle>
        <CardDescription>
          Paid plans unlock higher limits and advanced AI automation.
        </CardDescription>
      </CardHeader>
      <CardContent>
        <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
          {INCLUDED_MODULES.map(({ label, sub, Icon, color, bg }) => (
            <div key={label} className="rounded-md border bg-muted/20 p-4 text-center">
              <div className="mx-auto mb-3 flex h-9 w-9 items-center justify-center rounded-md" style={{ background: bg }}>
                <Icon className="h-4 w-4" style={{ color }} />
              </div>
              <p className="text-sm font-semibold">{label}</p>
              <p className="mt-1 text-xs text-muted-foreground">{sub}</p>
            </div>
          ))}
        </div>
      </CardContent>
    </Card>
  );
}

function PendingBillingNotice({
  billing,
  editable = false,
  resumeLoading = false,
  onResume,
}: {
  billing: WorkspaceBillingSummary;
  editable?: boolean;
  resumeLoading?: boolean;
  onResume?: () => void;
}) {
  if (!billing.pending_plan || !billing.pending_change_at) return null;
  const pendingLabel = PLAN_LABELS[billing.pending_plan] ?? billing.pending_plan;
  const currentLabel = PLAN_LABELS[billing.plan] ?? billing.plan;
  const isCancel = billing.pending_plan === 'free' || billing.cancel_at_period_end;
  const pendingCredits = includedCreditsForPlan(billing.pending_plan);
  const seatCopy = billing.pending_plan === 'free' && billing.seat_usage && billing.seat_usage > 2
    ? ` Free includes 2 seats; this workspace currently has ${billing.seat_usage}.`
    : '';
  return (
    <div className="flex flex-col gap-3 rounded-md border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200 sm:flex-row sm:items-start sm:justify-between">
      <div className="flex items-start gap-3">
        <CalendarClock className="mt-0.5 h-4 w-4 shrink-0" />
        <div>
        <p className="font-medium">
          {isCancel ? 'Subscription cancellation scheduled' : 'Plan change scheduled'}
        </p>
        <p className="mt-1">
          {isCancel
            ? `This workspace will move to Free on ${formatDate(billing.pending_change_at)}. ${currentLabel} remains active until then.${seatCopy}`
            : `${pendingLabel} will begin on ${formatDate(billing.pending_change_at)}. ${currentLabel} remains active until then.`}
        </p>
        <p className="mt-1">
          You have used {formatNumber(billing.credits_used)} of {formatNumber(billing.included_credits)} {currentLabel} credits. {pendingLabel} includes {formatNumber(pendingCredits)} credits per billing period.
        </p>
        </div>
      </div>
      {isCancel && editable && onResume && (
        <Button type="button" size="sm" variant="outline" className="shrink-0 bg-background" onClick={onResume} disabled={resumeLoading}>
          {resumeLoading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
          Keep subscription
        </Button>
      )}
    </div>
  );
}

function FreePlanCard({
  billing,
  interval,
  editable,
  loading,
  onSelect,
}: {
  billing: WorkspaceBillingSummary;
  interval: BillingInterval;
  editable: boolean;
  loading: boolean;
  onSelect: () => void;
}) {
  const isCurrent = billing.plan === 'free';
  const isPending = billing.pending_plan === 'free';
  const seatLimit = billing.seat_limit || 2;
  const seatOverFreeLimit = (billing.seat_usage ?? 0) > seatLimit;
  const actionLabel = isCurrent ? 'Current plan' : isPending ? 'Move to Free scheduled' : 'Move to Free at renewal';
  const blockerCopy = seatOverFreeLimit
    ? `Free includes ${seatLimit} seats. This workspace has ${billing.seat_usage} seats. Remove ${(billing.seat_usage ?? 0) - seatLimit} members or stay on a paid plan.`
    : '';
  return (
    <Card className="h-full">
      <CardHeader className="space-y-3 pb-3">
        <div className="flex items-start justify-between gap-3">
          <div>
            <CardTitle className="text-base">{FREE_PLAN.name}</CardTitle>
            <CardDescription className="mt-1">{FREE_PLAN.description}</CardDescription>
          </div>
          {isCurrent && <Badge variant="secondary">Current plan</Badge>}
          {isPending && <Badge variant="outline">Scheduled</Badge>}
        </div>
        <div className="flex items-end gap-1">
          <span className="text-3xl font-semibold tracking-normal">$0</span>
          <span className="pb-1 text-sm text-muted-foreground">/month</span>
        </div>
        {interval === 'annual' && (
          <p className="text-xs font-medium text-transparent" aria-hidden="true">
            Billed annually
          </p>
        )}
      </CardHeader>
      <CardContent className="space-y-5">
        <Button
          className="w-full"
          variant={isCurrent || isPending ? 'outline' : 'destructive'}
          disabled={!editable || loading || isCurrent || isPending || seatOverFreeLimit}
          onClick={onSelect}
          aria-label={`${actionLabel} Free`}
        >
          {loading && !isCurrent && !isPending ? <Loader2 className="mr-2 h-4 w-4 animate-spin" /> : <Check className="mr-2 h-4 w-4" />}
          {actionLabel}
        </Button>
        {blockerCopy && (
          <p className="text-xs leading-5 text-amber-700 dark:text-amber-300">{blockerCopy}</p>
        )}

        <div className="grid grid-cols-2 gap-3 border-y py-4 text-center">
          <div>
            <p className="text-sm font-semibold">2</p>
            <p className="mt-0.5 text-[11px] uppercase text-muted-foreground">Seats</p>
          </div>
          <div>
            <p className="text-sm font-semibold">{formatNumber(FREE_PLAN.credits)}/mo</p>
            <p className="mt-0.5 text-[11px] uppercase text-muted-foreground">AI credits</p>
          </div>
        </div>

        <ul className="space-y-2.5">
          {FREE_PLAN.features.map((feature) => (
            <li key={feature} className="flex items-start gap-2 text-sm">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
              <span className="text-muted-foreground">{feature}</span>
            </li>
          ))}
        </ul>
      </CardContent>
    </Card>
  );
}

function PlanChoiceCard({
  plan,
  interval,
  billing,
  editable,
  disabled,
  loading,
  onSelect,
}: {
  plan: PaidPlanOption;
  interval: BillingInterval;
  billing: WorkspaceBillingSummary;
  editable: boolean;
  disabled: boolean;
  loading: boolean;
  onSelect: () => void;
}) {
  const price = interval === 'annual' ? Math.round(plan.annual / 12) : plan.monthly;
  const state = planActionState(billing, plan.id, interval);

  return (
    <Card className={`h-full ${plan.popular ? 'border-primary shadow-sm' : ''}`}>
        <CardHeader className="space-y-3 pb-3">
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle className="text-base">{plan.name}</CardTitle>
              <CardDescription className="mt-1">{plan.description}</CardDescription>
            </div>
            <div className="flex flex-col items-end gap-2">
              {plan.popular && <Badge>Most popular</Badge>}
              {state.kind === 'current' && <Badge variant="secondary">Current</Badge>}
              {state.kind === 'scheduled' && <Badge variant="outline">Scheduled</Badge>}
            </div>
          </div>
          <div className="flex flex-wrap items-end gap-2">
            {interval === 'annual' && (
              <span className="pb-1 text-lg font-medium text-muted-foreground line-through">${plan.monthly}</span>
            )}
            <span className="text-3xl font-semibold tracking-normal">${price}</span>
            <span className="pb-1 text-sm text-muted-foreground">/month</span>
          </div>
          {interval === 'annual' && (
            <p className="text-xs font-medium text-primary">Billed annually at ${plan.annual}/year</p>
          )}
        </CardHeader>
        <CardContent className="space-y-5">
          <Button
            className="w-full"
            variant={state.kind === 'current' || state.kind === 'scheduled' ? 'outline' : 'default'}
            onClick={onSelect}
            disabled={!editable || disabled || loading || state.kind === 'current' || state.kind === 'scheduled'}
            aria-label={`${state.label} ${plan.name}`}
          >
            {loading && state.kind !== 'current' && state.kind !== 'scheduled' && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            {state.label}
          </Button>

          <div className="grid grid-cols-2 gap-3 border-y py-4 text-center">
            <div>
              <p className="text-sm font-semibold">Unlimited</p>
              <p className="mt-0.5 text-[11px] uppercase text-muted-foreground">Seats</p>
            </div>
            <div>
              <p className="text-sm font-semibold">{formatNumber(plan.credits)}/mo</p>
              <p className="mt-0.5 text-[11px] uppercase text-muted-foreground">AI credits</p>
            </div>
          </div>

          <ul className="space-y-2.5">
            {plan.features.map((feature, index) => (
              <li key={feature} className="flex items-start gap-2 text-sm">
                {index === 0 ? (
                  <span className="font-medium text-foreground">{feature}</span>
                ) : (
                  <>
                    <Check className="mt-0.5 h-4 w-4 shrink-0 text-primary" />
                    <span className="text-muted-foreground">{feature}</span>
                  </>
                )}
              </li>
            ))}
          </ul>
        </CardContent>
      </Card>
  );
}

function PlanComparison() {
  return (
    <section className="space-y-3">
      <SectionHeading
        title="Detailed comparison"
        description="All plans include the full Helpin workspace. Paid plans increase limits and unlock advanced control."
        align="center"
        size="lg"
      />
      <Card>
        <CardContent className="p-0">
          <div className="overflow-x-auto">
            <table className="w-full min-w-[720px] text-left text-sm">
              <thead>
                <tr className="border-b bg-muted/20">
                  <th className="px-5 py-3 font-medium text-muted-foreground">Feature</th>
                  <th className="px-4 py-3 text-center font-semibold">Free</th>
                  <th className="px-4 py-3 text-center font-semibold">Starter</th>
                  <th className="px-4 py-3 text-center font-semibold text-primary">Growth</th>
                </tr>
              </thead>
              <tbody>
                {COMPARISON_FEATURES.map((row) => {
                  if (row.category) {
                    return (
                      <tr key={row.name}>
                        <td colSpan={4} className="px-5 pb-2 pt-6 text-xs font-semibold uppercase text-muted-foreground">
                          {row.name}
                        </td>
                      </tr>
                    );
                  }
                  return (
                    <tr key={row.name} className="border-t">
                      <td className="px-5 py-3 font-medium">{row.name}</td>
                      <td className="px-4 py-3 text-center"><ComparisonValue value={row.free} /></td>
                      <td className="px-4 py-3 text-center"><ComparisonValue value={row.starter} /></td>
                      <td className="px-4 py-3 text-center"><ComparisonValue value={row.growth} /></td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}

function ComparisonValue({ value }: { value?: boolean | string }) {
  if (value === true) return <Check className="mx-auto h-4 w-4 text-primary" />;
  if (value === false || value === undefined) return <span className="text-muted-foreground/40">-</span>;
  return <span className="font-medium">{value}</span>;
}

function BillingFAQs() {
  return (
    <section className="space-y-3">
      <SectionHeading
        title="FAQs"
        description="Common billing and plan questions before you upgrade."
        align="center"
        size="lg"
      />
      <Card>
        <CardContent className="divide-y p-0">
          {BILLING_FAQS.map((faq) => (
            <details key={faq.q} className="group px-5 py-4">
              <summary className="cursor-pointer list-none text-sm font-medium">
                <span className="flex items-center justify-between gap-4">
                  {faq.q}
                  <span className="text-lg leading-none text-muted-foreground transition-transform group-open:rotate-45">+</span>
                </span>
              </summary>
              <p className="mt-3 pr-8 text-sm text-muted-foreground">{faq.a}</p>
            </details>
          ))}
        </CardContent>
      </Card>
    </section>
  );
}

function CancelSubscriptionDialog({
  billing,
  open,
  loading,
  onOpenChange,
  onConfirm,
}: {
  billing: WorkspaceBillingSummary;
  open: boolean;
  loading: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) {
  const currentLabel = PLAN_LABELS[billing.plan] ?? billing.plan;
  const renewalDate = formatDate(billing.current_period_end);
  const seatCopy = billing.seat_usage && billing.seat_usage > 2
    ? ` This workspace currently has ${billing.seat_usage} seats, so new invites will be blocked on Free until the workspace is back within the 2-seat limit.`
    : '';

  return (
    <AlertDialog open={open} onOpenChange={(nextOpen) => {
      if (!loading) onOpenChange(nextOpen);
    }}>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogMedia className="bg-destructive/10 text-destructive">
            <Ban className="h-7 w-7" />
          </AlertDialogMedia>
          <AlertDialogTitle>Cancel subscription?</AlertDialogTitle>
          <AlertDialogDescription>
            This will cancel the {currentLabel} subscription at renewal. The workspace stays on {currentLabel} until {renewalDate}, then moves to Free with 2 seats and 1,000 AI credits per month.{seatCopy}
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>Keep current plan</AlertDialogCancel>
          <AlertDialogAction
            variant="destructive"
            disabled={loading}
            onClick={(event) => {
              event.preventDefault();
              onConfirm();
            }}
          >
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Cancel at renewal
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function PlanChangePreviewDialog({
  preview,
  loading,
  onOpenChange,
  onConfirm,
}: {
  preview: PlanChangePreview | null;
  loading: boolean;
  onOpenChange: (open: boolean) => void;
  onConfirm: () => void;
}) {
  if (!preview) return null;
  const targetLabel = `${PLAN_LABELS[preview.target_plan] ?? preview.target_plan} ${preview.target_interval === 'annual' ? 'Annual' : 'Monthly'}`;
  const currentLabel = `${PLAN_LABELS[preview.current_plan] ?? preview.current_plan} ${preview.current_interval === 'annual' ? 'Annual' : 'Monthly'}`;
  const creditAppliedCents = Math.max(0, -preview.total_cents);
  const amountCopy = preview.amount_due_cents === 0
    ? '$0 due today'
    : `${formatMoney(preview.amount_due_cents, preview.currency)} due today`;
  const prorationLines = preview.lines.filter((line) => line.proration).slice(0, 4);

  return (
    <AlertDialog open={!!preview} onOpenChange={(open) => {
      if (!loading) onOpenChange(open);
    }}>
      <AlertDialogContent className="!max-w-[min(720px,calc(100vw-2rem))]">
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="absolute right-4 top-4 h-8 w-8"
          aria-label="Close"
          disabled={loading}
          onClick={() => onOpenChange(false)}
        >
          <X className="h-4 w-4" />
        </Button>
        <AlertDialogHeader>
          <AlertDialogMedia className="bg-primary/10 text-primary">
            <CreditCard className="h-7 w-7" />
          </AlertDialogMedia>
          <AlertDialogTitle>Confirm plan change</AlertDialogTitle>
          <AlertDialogDescription>
            Review the billing preview before switching from {currentLabel} to {targetLabel}.
          </AlertDialogDescription>
        </AlertDialogHeader>

        <div className="space-y-4 text-sm">
          <div className={`grid gap-3 ${creditAppliedCents > 0 ? 'sm:grid-cols-3' : 'sm:grid-cols-2'}`}>
            <PreviewStat label="Today" value={amountCopy} />
            {creditAppliedCents > 0 && (
              <PreviewStat label="Credit after change" value={`${formatMoney(creditAppliedCents, preview.currency)} credit`} />
            )}
            <PreviewStat label="Next billing date" value={formatDate(preview.current_period_end)} />
          </div>

          <div className="grid gap-3 sm:grid-cols-2">
            <PreviewStat label="Effective" value="Immediately" />
            <PreviewStat label="New plan" value={targetLabel} />
          </div>

          <div className="rounded-md border bg-muted/20 p-3">
            <p className="font-medium">{planChangeSummary(preview, creditAppliedCents)}</p>
            <p className="mt-1 text-muted-foreground">
              The change applies immediately. Future invoices use the {targetLabel} price.
            </p>
          </div>

          <div className="rounded-md border bg-muted/20 p-3">
            <p className="font-medium">AI credits after change</p>
            <p className="mt-1 text-muted-foreground">
              {formatNumber(preview.target_included_credits)} credits per month. {formatNumber(preview.credits_used)} used this period, {formatNumber(preview.credits_remaining_after)} available after the change.
            </p>
          </div>

          {prorationLines.length > 0 && (
            <div className="rounded-md border p-3">
              <p className="font-medium">How this is calculated</p>
              <div className="mt-2 space-y-2">
                {prorationLines.map((line, index) => (
                  <div key={`${line.description}-${index}`} className="flex items-start justify-between gap-3 text-muted-foreground">
                    <span className="min-w-0">{prorationLineLabel(line, currentLabel, targetLabel)}</span>
                    <span className="shrink-0 font-medium text-foreground">{formatSignedMoney(line.amount_cents, preview.currency)}</span>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>

        <AlertDialogFooter>
          <AlertDialogCancel disabled={loading}>Cancel</AlertDialogCancel>
          <AlertDialogAction
            disabled={loading}
            onClick={(event) => {
              event.preventDefault();
              onConfirm();
            }}
          >
            {loading && <Loader2 className="mr-2 h-4 w-4 animate-spin" />}
            Confirm change
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}

function prorationLineLabel(line: PlanChangePreview['lines'][number], currentLabel: string, targetLabel: string): string {
  if (line.amount_cents < 0) return `Credit for unused ${currentLabel} time`;
  if (line.amount_cents > 0) return `Charge for ${targetLabel} time until billing date`;
  return 'Proration adjustment';
}

function planChangeSummary(preview: PlanChangePreview, creditAppliedCents: number): string {
  if (preview.amount_due_cents > 0) {
    return `${formatMoney(preview.amount_due_cents, preview.currency)} will be charged today.`;
  }
  if (creditAppliedCents > 0) {
    return `No charge today. ${formatMoney(creditAppliedCents, preview.currency)} will be applied as credit to future invoices.`;
  }
  return 'No charge today.';
}

function PreviewStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border bg-background p-3">
      <p className="text-xs text-muted-foreground">{label}</p>
      <p className="mt-1 font-semibold">{value}</p>
    </div>
  );
}

function PortalAction({
  icon: Icon,
  title,
  description,
  disabled,
  loading,
  danger,
  onClick,
}: {
  icon: typeof FileText;
  title: string;
  description: string;
  disabled: boolean;
  loading: boolean;
  danger?: boolean;
  onClick: () => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      disabled={disabled}
      className="flex w-full items-center gap-4 px-5 py-4 text-left transition-colors hover:bg-muted/50 disabled:cursor-not-allowed disabled:opacity-50"
    >
      <span className={`flex h-9 w-9 shrink-0 items-center justify-center rounded-md border ${danger ? 'text-destructive' : 'text-muted-foreground'}`}>
        <Icon className="h-4 w-4" />
      </span>
      <span className="min-w-0 flex-1">
        <span className={`block text-sm font-medium ${danger ? 'text-destructive' : 'text-foreground'}`}>{title}</span>
        <span className="mt-0.5 block text-sm text-muted-foreground">{description}</span>
      </span>
      {loading ? (
        <Loader2 className="h-4 w-4 shrink-0 animate-spin text-muted-foreground" />
      ) : (
        <ExternalLink className="h-4 w-4 shrink-0 text-muted-foreground" />
      )}
    </button>
  );
}

function planActionState(
  billing: WorkspaceBillingSummary,
  targetPlan: Exclude<BillingPlan, 'free'>,
  targetInterval: BillingInterval,
): { kind: 'current' | 'scheduled' | 'upgrade' | 'downgrade' | 'switch'; label: string } {
  if (billing.pending_plan === targetPlan && billing.pending_billing_interval === targetInterval) {
    return { kind: 'scheduled', label: 'Scheduled' };
  }
  if (billing.plan === targetPlan && billing.billing_interval === targetInterval && !billing.pending_plan) {
    return { kind: 'current', label: 'Current plan' };
  }
  if (billing.plan === 'free' || !billing.stripe_subscription_id) {
    return { kind: 'upgrade', label: 'Upgrade' };
  }
  if (planChangeIsDeferred(billing.plan, billing.billing_interval, targetPlan, targetInterval)) {
    return {
      kind: planRank(targetPlan) < planRank(billing.plan) ? 'downgrade' : 'switch',
      label: planRank(targetPlan) < planRank(billing.plan) ? 'Downgrade at renewal' : 'Switch at renewal',
    };
  }
  return {
    kind: planRank(targetPlan) > planRank(billing.plan) ? 'upgrade' : 'switch',
    label: planRank(targetPlan) > planRank(billing.plan)
      ? 'Upgrade now'
      : planRank(targetPlan) < planRank(billing.plan)
        ? 'Downgrade now'
        : 'Switch now',
  };
}

function planChangeIsDeferred(
  currentPlan: BillingPlan | string,
  currentInterval: BillingInterval | string,
  targetPlan: BillingPlan,
  targetInterval: BillingInterval,
) {
  if (targetPlan === 'free') return true;
  void currentPlan;
  void currentInterval;
  void targetInterval;
  return false;
}

function planRank(plan: BillingPlan | string): number {
  if (plan === 'growth') return 2;
  if (plan === 'starter') return 1;
  return 0;
}

function includedCreditsForPlan(plan: BillingPlan | string): number {
  if (plan === 'growth') return 25_000;
  if (plan === 'starter') return 5_000;
  return 1_000;
}

function annualBillingNudge(billing: WorkspaceBillingSummary): AnnualBillingNudgeCopy | null {
  if (billing.plan === 'free') return null;
  if (billing.billing_interval !== 'monthly') return null;
  if (billing.status !== 'active') return null;
  if (billing.trialing || billing.pending_plan || billing.cancel_at_period_end) return null;
  if (!billing.stripe_subscription_id) return null;

  const plan = PLAN_OPTIONS.find((option) => option.id === billing.plan);
  if (!plan) return null;

  const monthlyTotal = plan.monthly * 12;
  const savings = monthlyTotal - plan.annual;
  if (savings <= 0) return null;

  return {
    planName: plan.name,
    monthlyTotal,
    annualTotal: plan.annual,
    savings,
    monthlyEquivalent: Math.round(plan.annual / 12),
  };
}

function billingOverviewPlanTitle(billing: WorkspaceBillingSummary): string {
  const plan = PLAN_LABELS[billing.plan] ?? billing.plan;
  if (billing.plan === 'free') return plan;
  const interval = billing.billing_interval === 'annual' ? 'Annual' : 'Monthly';
  return `${plan} ${interval}`;
}

function statusLabel(billing: WorkspaceBillingSummary): string {
  if (billing.status === 'past_due') return 'Past due';
  if (billing.status === 'canceled') return 'Canceled';
  if (hasScheduledCancellation(billing)) return `Active until ${formatDate(cancellationEffectiveDate(billing))}`;
  if (billing.trialing) return 'Trial';
  if (billing.plan === 'free') return 'Free';
  return 'Active';
}

function billingStatusCopy(billing: WorkspaceBillingSummary): string {
  if (hasScheduledCancellation(billing)) {
    return `Subscription cancellation scheduled. This workspace will move to Free on ${formatDate(cancellationEffectiveDate(billing))}.`;
  }
  if (billing.trialing && billing.trial_ends_at) {
    return `${PLAN_LABELS[billing.plan] ?? billing.plan} trial ends ${formatDate(billing.trial_ends_at)}`;
  }
  if (billing.status === 'past_due') return 'Payment requires attention';
  if (billing.plan === 'free') return 'Free plan with monthly AI credits';
  return '';
}

function periodCopy(billing: WorkspaceBillingSummary): string {
  if (billing.plan === 'free') return 'Free';
  return billing.billing_interval === 'annual' ? 'Annual' : 'Monthly';
}

function nextChargeMetricLabel(billing: WorkspaceBillingSummary): string {
  if (hasScheduledCancellation(billing)) return 'Ends on';
  if (billing.plan === 'free' || billing.trialing) return 'Credit reset';
  return 'Next charge';
}

function nextChargeMetricValue(billing: WorkspaceBillingSummary): string {
  if (hasScheduledCancellation(billing)) return formatDate(cancellationEffectiveDate(billing));
  if (billing.plan === 'free' || billing.trialing) return formatDate(billing.current_period_end);
  const amount = billing.next_charge_cents > 0 ? formatMoney(billing.next_charge_cents, 'usd') : '$0';
  return `${amount} on ${formatDate(billing.current_period_end)}`;
}

function hasScheduledCancellation(billing: WorkspaceBillingSummary): boolean {
  return billing.cancel_at_period_end || billing.pending_plan === 'free';
}

function cancellationEffectiveDate(billing: WorkspaceBillingSummary): string | undefined {
  return billing.pending_change_at || billing.current_period_end;
}

function statusBadgeClassName(billing: WorkspaceBillingSummary): string | undefined {
  if (hasScheduledCancellation(billing)) {
    return 'border-amber-200 bg-amber-50 text-amber-900 dark:border-amber-500/40 dark:bg-amber-500/10 dark:text-amber-200';
  }
  if (billing.status === 'active' && !billing.trialing && billing.plan !== 'free') {
    return 'border-emerald-200 bg-emerald-50 text-emerald-800 dark:border-emerald-500/40 dark:bg-emerald-500/10 dark:text-emerald-200';
  }
  return undefined;
}

function formatDate(value?: string): string {
  if (!value) return 'Not set';
  return new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric', year: 'numeric' }).format(new Date(value));
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat().format(value);
}

function formatMoney(cents: number, currency = 'usd'): string {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency: currency || 'usd',
  }).format(cents / 100);
}

function formatSignedMoney(cents: number, currency = 'usd'): string {
  if (cents === 0) return formatMoney(0, currency);
  const sign = cents > 0 ? '+' : '-';
  return `${sign}${formatMoney(Math.abs(cents), currency)}`;
}
