import { useState } from 'react';
import { toast } from 'sonner';

import { Button } from '@/components/ui/button';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { DeliveryPlanView } from '@/components/agents/dock/DeliveryPlanView';
import { StatusDot } from '@/components/agents/dock/StatusDot';
import { planSummaryText } from '@/components/agents/dock/utils';
import type { CommandBarRunPlan } from '@/components/agents/dock/planSummary';
import { commandBarService } from '@/lib/services/commandBarService';
import type { AgentRun } from '@/lib/pmTypes';
import {
  deliveryDotState,
  isPlanStalled,
  planHasActiveRuns,
  planHasFailedSteps,
} from './epicDeliveryDag';

export const EPIC_DELIVERY_PANEL_ID = 'epic-delivery-panel';

interface EpicDeliveryPipelineButtonProps {
  workspaceId: string;
  epicId: string;
  onStarted: () => Promise<void> | void;
  disabled?: boolean;
}

/**
 * Launches the epic delivery pipeline (implement → review → merge every open
 * task on the epic integration branch, ordered by blocking links, then open
 * the epic PR). Backed by POST /pm/epics/{id}/delivery-pipeline.
 */
export function EpicDeliveryPipelineButton({
  workspaceId,
  epicId,
  onStarted,
  disabled = false,
}: EpicDeliveryPipelineButtonProps) {
  const [busy, setBusy] = useState(false);

  const start = async () => {
    setBusy(true);
    try {
      const res = await commandBarService.startEpicDeliveryPipeline(workspaceId, epicId);
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to start delivery pipeline');
        return;
      }
      const skipped = res.data.skipped_tasks?.length ?? 0;
      toast.success(
        `Delivery pipeline started for ${res.data.task_count} task${res.data.task_count === 1 ? '' : 's'}${skipped ? ` (${skipped} skipped)` : ''}`,
      );
      await onStarted();
    } finally {
      setBusy(false);
    }
  };

  return (
    <Button
      variant="outline"
      size="sm"
      className="h-7 text-xs"
      disabled={disabled || busy}
      onClick={() => void start()}
    >
      {busy ? 'Starting…' : 'Run delivery pipeline'}
    </Button>
  );
}

interface EpicDeliveryRunsPanelProps {
  workspaceId: string;
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onReload: () => Promise<void>;
  canEdit?: boolean;
}

/**
 * Surfaces the epic's latest command-bar DAG/pipeline delivery, live and
 * visible to anyone who can read the epic (the dock only shows it to the user
 * who triggered it). Mostly read-only — dispatch/cancel stay in the dock —
 * but editors can resume a stalled delivery or retry a failed one.
 */
export function EpicDeliveryRunsPanel({
  workspaceId,
  plan,
  runsById,
  onReload,
  canEdit = false,
}: EpicDeliveryRunsPanelProps) {
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);
  const [busy, setBusy] = useState(false);

  // Retry whenever something failed/was cancelled and nothing is in flight —
  // including plans the backend still considers "running" (a zombie state
  // resume can't revive, since resume only starts steps with no run yet).
  const canRetry = canEdit && planHasFailedSteps(plan, runsById) && !planHasActiveRuns(plan, runsById);
  const canResume = canEdit && !canRetry && isPlanStalled(plan, runsById);

  const resume = async () => {
    setBusy(true);
    try {
      const res = await commandBarService.resumePlan(workspaceId, plan.id);
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to resume delivery');
        return;
      }
      toast.success('Delivery resumed');
      await onReload();
    } finally {
      setBusy(false);
    }
  };

  const retry = async () => {
    const failedIndex = plan.steps.findIndex((_, i) => {
      const runId = plan.runIdsByStep[i];
      const run = runId ? runsById[runId] : null;
      return !!run && (run.status === 'failed' || run.status === 'cancelled');
    });
    setBusy(true);
    try {
      const res = await commandBarService.retryPlan(
        workspaceId,
        plan.id,
        failedIndex >= 0 ? failedIndex : 0,
      );
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to retry delivery');
        return;
      }
      toast.success('Retrying failed steps');
      await onReload();
    } finally {
      setBusy(false);
    }
  };

  const headerActions =
    canResume || canRetry ? (
      <span className="flex shrink-0 items-center gap-1.5">
        {canResume ? (
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            disabled={busy}
            onClick={() => void resume()}
          >
            Resume
          </Button>
        ) : null}
        {canRetry ? (
          <Button
            variant="outline"
            size="sm"
            className="h-7 text-xs"
            disabled={busy}
            onClick={() => void retry()}
          >
            Retry failed steps
          </Button>
        ) : null}
      </span>
    ) : undefined;

  return (
    <div id={EPIC_DELIVERY_PANEL_ID} className="scroll-mt-4">
      <DeliveryPlanView
        plan={plan}
        runsById={runsById}
        headerActions={headerActions}
        onOpenRun={(runId) => {
          setSelectedRunId(runId);
          setDrawerOpen(true);
        }}
      />

      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={drawerOpen && !!selectedRunId}
        onOpenChange={setDrawerOpen}
        title="Delivery Run"
        description="Interactive agent chat, artifacts, and live tool activity for this delivery step."
      />
    </div>
  );
}

/**
 * Compact one-line delivery status for the epic sidebar. Clicking scrolls to
 * the full Delivery panel in the main column.
 */
export function EpicDeliveryStatusChip({
  plan,
  runsById,
  onClick,
}: {
  plan: CommandBarRunPlan;
  runsById: Record<string, AgentRun>;
  onClick?: () => void;
}) {
  const dot = deliveryDotState(plan, runsById);
  return (
    <button
      type="button"
      onClick={
        onClick ??
        (() =>
          document
            .getElementById(EPIC_DELIVERY_PANEL_ID)
            ?.scrollIntoView({ behavior: 'smooth', block: 'start' }))
      }
      className="mt-2 flex w-full items-center gap-2 rounded-md border border-border/60 bg-card px-2 py-1.5 text-left hover:bg-accent"
      title="Jump to delivery"
    >
      <StatusDot state={dot} />
      <span className="text-[11px] font-medium text-foreground/80">Delivery</span>
      <span className="min-w-0 flex-1 truncate text-[11px] text-muted-foreground">
        {planSummaryText(plan, runsById)}
      </span>
    </button>
  );
}
