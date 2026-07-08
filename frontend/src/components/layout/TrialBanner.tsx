import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { cn } from '@/lib/utils';
import { daysUntil } from '@/lib/billingUtils';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useOrganizationStore } from '@/stores/organizationStore';
import {
  useWorkspaceBilling,
  useWorkspaceAccess,
  usePermissions,
} from '@/hooks/queries';
import { PlanChangeModal } from '@/components/billing/PlanChangeModal';

/**
 * Subtle trial nudge shown in the sidebar footer for the active workspace
 * whenever its billing status is `trialing`. Visible to everyone; managers
 * get an Upgrade button, others get a gentle note.
 */
export function TrialBanner({ collapsed = false }: { collapsed?: boolean }) {
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id);
  const workspaceName = useWorkspaceStore((s) => s.currentWorkspace?.name ?? '');
  const isOrgOwner = useOrganizationStore((s) => s.currentOrganization?.role === 'owner');

  const { data: billing } = useWorkspaceBilling(workspaceId);
  const { data: access } = useWorkspaceAccess(workspaceId ?? '');
  const { canManageSettings } = usePermissions(access);

  const [planOpen, setPlanOpen] = useState(false);

  if (!billing || billing.status !== 'trialing') return null;

  const canManage = (billing.manage_billing_enabled ?? false) || canManageSettings || isOrgOwner;
  const days = daysUntil(billing.trial_ends_at);
  const urgent = days <= 3;

  if (collapsed) {
    return (
      <div
        className={cn(
          'mx-2 my-1 flex items-center justify-center rounded-md py-1 text-[10px] font-semibold',
          urgent
            ? 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-400'
            : 'bg-muted text-muted-foreground',
        )}
        title={`Trial ends in ${days} day${days === 1 ? '' : 's'}`}
      >
        {days}d
      </div>
    );
  }

  return (
    <div
      className={cn(
        'mx-2 my-2 rounded-lg border p-2.5 text-center text-xs',
        urgent
          ? 'border-amber-300 bg-amber-50 dark:border-amber-500/40 dark:bg-amber-500/10'
          : 'border-border bg-muted/50',
      )}
    >
      <div
        className={cn(
          'font-medium',
          urgent ? 'text-amber-800 dark:text-amber-300' : 'text-foreground',
        )}
      >
        Trial ends in {days} day{days === 1 ? '' : 's'}
      </div>
      {canManage ? (
        <>
          <Button
            size="sm"
            variant={urgent ? 'default' : 'outline'}
            className="mt-2 h-7 w-full text-xs"
            onClick={() => setPlanOpen(true)}
          >
            Upgrade
          </Button>
          {planOpen && (
            <PlanChangeModal
              open={planOpen}
              onOpenChange={setPlanOpen}
              workspaceId={billing.workspace_id}
              workspaceName={workspaceName}
              currentPlan={billing.plan}
            />
          )}
        </>
      ) : (
        <div className="mt-1 text-muted-foreground">Ask your workspace owner to upgrade.</div>
      )}
    </div>
  );
}
