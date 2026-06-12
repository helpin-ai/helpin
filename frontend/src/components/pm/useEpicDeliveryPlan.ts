import { useCallback, useEffect, useMemo, useState } from 'react';

import { planSummaryToRunPlan, type CommandBarRunPlan } from '@/components/agents/dock/planSummary';
import { commandBarService } from '@/lib/services/commandBarService';
import type { AgentRun, CommandBarPlanSummary } from '@/lib/pmTypes';
import { buildRunsById, pickLatestDeliveryPlan, planRunIdSet } from './epicDeliveryDag';

export interface EpicDeliveryPlanState {
  plan: CommandBarRunPlan | null;
  runsById: Record<string, AgentRun>;
  reload: () => Promise<void>;
}

/**
 * Loads and keeps live the epic's latest command-bar delivery plan (DAG /
 * task-pipeline), shared by the main-column Delivery panel and the sidebar
 * status chip. Live updates correlate on the plan's run-id set (DAG child
 * runs target tasks, not the epic), with a 5s poll while running as a
 * self-heal for missed events. No-ops when `workspaceId` is empty.
 */
export function useEpicDeliveryPlan(workspaceId: string, epicId: string): EpicDeliveryPlanState {
  const [plans, setPlans] = useState<CommandBarPlanSummary[]>([]);

  const reload = useCallback(async () => {
    if (!workspaceId) return;
    const res = await commandBarService.listEpicPlans(workspaceId, epicId);
    setPlans(res.data?.plans ?? []);
  }, [epicId, workspaceId]);

  useEffect(() => {
    void reload();
  }, [reload]);

  const summary = useMemo(() => pickLatestDeliveryPlan(plans), [plans]);
  const plan = useMemo(() => (summary ? planSummaryToRunPlan(summary) : null), [summary]);
  const runsById = useMemo(() => (summary ? buildRunsById(summary) : {}), [summary]);

  // Live updates: refetch when an event references one of the plan's runs.
  // While the plan is running but has no runs yet, refetch on any agent-run
  // event so the first run lands.
  useEffect(() => {
    if (!plan) return;
    const runIds = planRunIdSet(plan);
    const handler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      const runId = detail?.entity_id;
      if (runIds.has(runId ?? '') || (plan.status === 'running' && runIds.size === 0)) {
        void reload();
      }
    };
    window.addEventListener('agent_run-created', handler);
    window.addEventListener('agent_run-updated', handler);
    return () => {
      window.removeEventListener('agent_run-created', handler);
      window.removeEventListener('agent_run-updated', handler);
    };
  }, [plan, reload]);

  // Poll while the delivery is running, as a self-heal for any missed events.
  useEffect(() => {
    if (plan?.status !== 'running') return;
    const intervalId = window.setInterval(() => {
      void reload();
    }, 5_000);
    return () => window.clearInterval(intervalId);
  }, [plan?.status, reload]);

  return { plan, runsById, reload };
}
