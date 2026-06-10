import { useCallback, useEffect, useMemo, useState } from 'react';

import { BotIcon } from '@/lib/icons';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { EpicDeliveryDag } from '@/components/agents/dock/EpicDeliveryDag';
import { planSummaryToRunPlan } from '@/components/agents/dock/planSummary';
import { commandBarService } from '@/lib/services/commandBarService';
import type { CommandBarPlanSummary } from '@/lib/pmTypes';
import { buildRunsById, pickLatestDeliveryPlan, planRunIdSet } from './epicDeliveryDag';

interface EpicDeliveryRunsPanelProps {
  workspaceId: string;
  epicId: string;
}

/**
 * Surfaces the epic's latest command-bar DAG/pipeline delivery, live and
 * visible to anyone who can read the epic (the dock only shows it to the user
 * who triggered it). Read-only: dispatch/cancel stay in the dock. Renders
 * nothing when the epic has no delivery plan.
 */
export function EpicDeliveryRunsPanel({ workspaceId, epicId }: EpicDeliveryRunsPanelProps) {
  const [plans, setPlans] = useState<CommandBarPlanSummary[]>([]);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [drawerOpen, setDrawerOpen] = useState(false);

  const load = useCallback(async () => {
    const res = await commandBarService.listEpicPlans(workspaceId, epicId);
    setPlans(res.data?.plans ?? []);
  }, [epicId, workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const summary = useMemo(() => pickLatestDeliveryPlan(plans), [plans]);
  const plan = useMemo(() => (summary ? planSummaryToRunPlan(summary) : null), [summary]);
  const runsById = useMemo(() => (summary ? buildRunsById(summary) : {}), [summary]);

  // Live updates: DAG child runs target tasks (not the epic), so correlate by
  // the plan's run-id set rather than parent_id. While the plan is running but
  // has no runs yet, refetch on any agent-run event so the first run lands.
  useEffect(() => {
    if (!plan) return;
    const runIds = planRunIdSet(plan);
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      const runId = detail?.entity_id;
      if (runIds.has(runId ?? '') || (plan.status === 'running' && runIds.size === 0)) {
        void load();
      }
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [load, plan]);

  // Poll while the delivery is running, as a self-heal for any missed events.
  useEffect(() => {
    if (plan?.status !== 'running') return;
    const intervalId = window.setInterval(() => {
      void load();
    }, 5_000);
    return () => window.clearInterval(intervalId);
  }, [load, plan?.status]);

  if (!plan) return null;

  return (
    <div className="overflow-hidden rounded-md border border-border/60 bg-card">
      <div className="flex items-center gap-2 px-3 py-2">
        <BotIcon className="h-3.5 w-3.5 text-muted-foreground" />
        <span className="text-xs font-semibold uppercase tracking-wide text-foreground/70">
          Delivery
        </span>
      </div>
      <div className="border-t border-border/60 px-3 py-2.5">
        <EpicDeliveryDag
          plan={plan}
          runsById={runsById}
          onOpenRun={(runId) => {
            setSelectedRunId(runId);
            setDrawerOpen(true);
          }}
        />
      </div>

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
