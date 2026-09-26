import { useMemo } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { ArrowRight02Icon, SparklesIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog';
import { useWorkspaceBilling } from '@/ee/hooks/queries/useBilling';
import { useOrganizationStore } from '@/stores/organizationStore';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import {
  GROWTH_FEATURE_BENEFITS,
  GROWTH_UNLIMITED_BENEFITS,
  getUpgradeRequiredReason,
  type UpgradeRequiredReason,
} from '@/ee/lib/upgradeRequired';
import { BILLING_CHOOSE_PLAN_SEARCH } from '@/ee/lib/billingNavigation';

interface UpgradeRequiredDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onUpgrade?: () => void;
  reason: UpgradeRequiredReason | string | null;
}

export function UpgradeRequiredDialog({ open, onOpenChange, onUpgrade, reason }: UpgradeRequiredDialogProps) {
  const navigate = useNavigate();
  const workspace = useWorkspaceStore((s) => s.currentWorkspace);
  const orgRole = useOrganizationStore((s) => s.currentOrganization?.role);
  const { data: billing } = useWorkspaceBilling(workspace?.id);

  const resolved = useMemo(
    () => typeof reason === 'string' ? getUpgradeRequiredReason(reason) : reason,
    [reason],
  );
  const canManageBilling = Boolean(billing?.manage_billing_enabled) || orgRole === 'owner';
  const featureBenefits = useMemo(() => {
    if (!resolved?.primaryBenefit || GROWTH_UNLIMITED_BENEFITS.includes(resolved.primaryBenefit)) {
      return GROWTH_FEATURE_BENEFITS.slice(0, 6);
    }
    return [
      resolved.primaryBenefit,
      ...GROWTH_FEATURE_BENEFITS.filter((benefit) => benefit !== resolved.primaryBenefit),
    ].slice(0, 6);
  }, [resolved?.primaryBenefit]);
  const unlimitedBenefits = useMemo(() => {
    if (!resolved?.primaryBenefit || !GROWTH_UNLIMITED_BENEFITS.includes(resolved.primaryBenefit)) {
      return GROWTH_UNLIMITED_BENEFITS;
    }
    return [
      resolved.primaryBenefit,
      ...GROWTH_UNLIMITED_BENEFITS.filter((benefit) => benefit !== resolved.primaryBenefit),
    ];
  }, [resolved?.primaryBenefit]);

  const handleUpgrade = () => {
    if (!workspace?.slug) return;
    onUpgrade?.();
    onOpenChange(false);
    void navigate({
      to: '/w/$slug/settings/billing',
      params: { slug: workspace.slug },
      search: BILLING_CHOOSE_PLAN_SEARCH,
    });
  };

  if (!resolved) return null;

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="sm:max-w-xl">
        <DialogHeader>
          <div className="mb-2 flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
            <SparklesIcon className="h-4 w-4" />
          </div>
          <DialogTitle>{resolved.title}</DialogTitle>
          <DialogDescription>{resolved.message}</DialogDescription>
        </DialogHeader>

        <div className="rounded-lg border border-border bg-muted/30 p-4">
          <div className="text-xs font-medium uppercase tracking-wide text-muted-foreground">
            The Growth plan includes
          </div>
          <div className="mt-3 grid gap-4 sm:grid-cols-[1fr_0.8fr]">
            <BenefitColumn benefits={featureBenefits} />
            <BenefitColumn benefits={unlimitedBenefits} accent />
          </div>
        </div>

        <DialogFooter>
          <Button variant="outline" onClick={() => onOpenChange(false)}>
            Close
          </Button>
          <Button onClick={handleUpgrade} disabled={!canManageBilling || !workspace?.slug}>
            {canManageBilling ? 'Upgrade' : 'Ask owner'}
            {canManageBilling && <ArrowRight02Icon className="h-4 w-4" />}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function BenefitColumn({
  benefits,
  accent = false,
}: {
  benefits: string[];
  accent?: boolean;
}) {
  return (
    <div className="min-w-0">
      <div className="grid gap-2">
        {benefits.map((benefit) => (
          <div key={benefit} className="flex min-w-0 items-center gap-2 text-sm">
            <span className={accent ? 'h-1.5 w-1.5 shrink-0 rounded-full bg-emerald-500' : 'h-1.5 w-1.5 shrink-0 rounded-full bg-primary'} />
            <span className="min-w-0">{benefit}</span>
          </div>
        ))}
      </div>
    </div>
  );
}
