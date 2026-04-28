import { useCallback, useEffect, useMemo, useState } from 'react';
import { toast } from 'sonner';
import { BotIcon, Cancel01Icon, Loading01Icon, Menu01Icon, Tick01Icon, ViewIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus, STATUS_META } from '@/components/pm/agentRunConstants';
import { agentService } from '@/lib/services/agentService';
import { commandBarService } from '@/lib/services/commandBarService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import type { AgentRun } from '@/lib/pmTypes';

function targetLabel(run: AgentRun) {
  return run.target_info?.title || run.target_info?.task_key || `${run.target_type} ${run.target_id.slice(0, 8)}`;
}

export function CommandBarRunRail() {
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id);
  const runIds = useCommandBarRunStore((s) => s.runIds);
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const planIds = useCommandBarRunStore((s) => s.planIds);
  const plansById = useCommandBarRunStore((s) => s.plansById);
  const railOpen = useCommandBarRunStore((s) => s.railOpen);
  const updateRun = useCommandBarRunStore((s) => s.updateRun);
  const hydratePlans = useCommandBarRunStore((s) => s.hydratePlans);
  const updatePlan = useCommandBarRunStore((s) => s.updatePlan);
  const setRailOpen = useCommandBarRunStore((s) => s.setRailOpen);
  const clear = useCommandBarRunStore((s) => s.clear);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [busyRunId, setBusyRunId] = useState<string | null>(null);
  const [busyPlanId, setBusyPlanId] = useState<string | null>(null);

  const runs = useMemo(
    () => runIds.map((id) => runsById[id]).filter(Boolean),
    [runIds, runsById],
  );
  const plans = useMemo(
    () => planIds.map((id) => plansById[id]).filter(Boolean),
    [planIds, plansById],
  );
  const planRunIds = useMemo(() => {
    const ids = new Set<string>();
    for (const plan of plans) {
      for (const runId of Object.values(plan.runIdsByStep)) ids.add(runId);
    }
    return ids;
  }, [plans]);
  const standaloneRuns = useMemo(
    () => runs.filter((run) => !planRunIds.has(run.id)),
    [planRunIds, runs],
  );
  const activeCount = runs.filter((run) => ACTIVE_RUN_STATUSES.has(run.status)).length;

  useEffect(() => {
    if (!workspaceId) return;
    let cancelled = false;
    void commandBarService.listPlans(workspaceId, 10).then((res) => {
      if (!cancelled && res.data?.plans) hydratePlans(res.data.plans);
    });
    return () => {
      cancelled = true;
    };
  }, [hydratePlans, workspaceId]);

  const refreshRun = useCallback(
    async (runId: string, allowUnknown = false) => {
      if (!workspaceId || (!allowUnknown && !runIds.includes(runId))) return;
      const res = await agentService.getRun(workspaceId, runId);
      if (res.data) updateRun(res.data);
    },
    [runIds, updateRun, workspaceId],
  );

  useEffect(() => {
    const createdHandler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id) void refreshRun(detail.entity_id, true);
    };
    const updatedHandler = (event: Event) => {
      const detail = (event as CustomEvent<{ entity_id?: string }>).detail;
      if (detail?.entity_id) void refreshRun(detail.entity_id);
    };
    window.addEventListener('agent_run-created', createdHandler);
    window.addEventListener('agent_run-updated', updatedHandler);
    return () => {
      window.removeEventListener('agent_run-created', createdHandler);
      window.removeEventListener('agent_run-updated', updatedHandler);
    };
  }, [refreshRun]);

  const runAction = async (run: AgentRun, action: 'approve' | 'cancel') => {
    if (!workspaceId) return;
    setBusyRunId(run.id);
    try {
      const res = action === 'approve'
        ? await agentService.approveRun(workspaceId, run.id)
        : await agentService.cancelRun(workspaceId, run.id);
      if (res.error || !res.data) {
        toast.error(res.error ?? `Failed to ${action} run`);
        return;
      }
      updateRun(res.data);
    } finally {
      setBusyRunId(null);
    }
  };

  const promoteRun = async (run: AgentRun) => {
    if (!workspaceId) return;
    const name = window.prompt('Reusable agent name');
    if (!name?.trim()) return;
    setBusyRunId(run.id);
    try {
      const res = await commandBarService.promoteRunToAgent(workspaceId, run.id, { name: name.trim() });
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to save agent');
        return;
      }
      toast.success(`Saved ${res.data.agent.name}`);
    } finally {
      setBusyRunId(null);
    }
  };

  const cancelPlan = async (planId: string) => {
    if (!workspaceId) return;
    setBusyPlanId(planId);
    try {
      const res = await commandBarService.cancelPlan(workspaceId, planId);
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to cancel plan');
        return;
      }
      updatePlan(res.data.plan, res.data.runs ?? []);
      toast.success('Command plan cancelled');
    } finally {
      setBusyPlanId(null);
    }
  };

  const retryPlan = async (planId: string, stepIndex: number) => {
    if (!workspaceId) return;
    setBusyPlanId(planId);
    try {
      const res = await commandBarService.retryPlan(workspaceId, planId, stepIndex);
      if (res.error || !res.data) {
        toast.error(res.error ?? 'Failed to retry plan');
        return;
      }
      updatePlan(res.data.plan, [res.data.run]);
      toast.success(`Retrying step ${stepIndex + 1}`);
    } finally {
      setBusyPlanId(null);
    }
  };

  const renderRunActions = (run: AgentRun) => {
    const displayStatus = getAgentRunDisplayStatus(run);
    const awaitingApproval = displayStatus === 'awaiting_approval';
    const busy = busyRunId === run.id;
    return (
      <div className="flex items-center justify-end gap-1.5">
        {awaitingApproval ? (
          <Button type="button" variant="outline" size="sm" className="h-7 gap-1.5" disabled={busy} onClick={() => void runAction(run, 'approve')}>
            {busy ? <Loading01Icon className="h-3.5 w-3.5 animate-spin" /> : <Tick01Icon className="h-3.5 w-3.5" />}
            Approve
          </Button>
        ) : null}
        {ACTIVE_RUN_STATUSES.has(run.status) ? (
          <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busy} onClick={() => void runAction(run, 'cancel')}>
            Cancel
          </Button>
        ) : null}
        {run.status === 'completed' ? (
          <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busy} onClick={() => void promoteRun(run)}>
            Save agent
          </Button>
        ) : null}
        <Button type="button" variant="ghost" size="sm" className="h-7 gap-1.5" onClick={() => setSelectedRunId(run.id)}>
          <ViewIcon className="h-3.5 w-3.5" />
          Open
        </Button>
      </div>
    );
  };

  const renderStatusBadge = (run: AgentRun) => {
    const displayStatus = getAgentRunDisplayStatus(run);
    const meta = STATUS_META[displayStatus] ?? STATUS_META[run.status];
    return (
      <Badge variant={meta?.variant ?? 'outline'} className={meta?.className}>
        {meta?.label ?? run.status}
      </Badge>
    );
  };

  if (runs.length === 0 && plans.length === 0) return null;

  if (!railOpen) {
    return (
      <Button
        type="button"
        size="sm"
        variant="outline"
        className="fixed right-4 top-20 z-40 h-9 gap-2 border-border/70 bg-background shadow-sm"
        onClick={() => setRailOpen(true)}
      >
        <Menu01Icon className="h-4 w-4" />
        {activeCount} active
      </Button>
    );
  }

  return (
    <>
      <aside className="fixed right-4 top-20 z-40 w-[min(24rem,calc(100vw-2rem))] overflow-hidden rounded-md border border-border/70 bg-background shadow-xl">
        <div className="flex items-center justify-between border-b px-3 py-2">
          <div className="flex items-center gap-2">
            <BotIcon className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium">Command Runs</span>
            <Badge variant="outline" className="text-[10px]">{activeCount} active</Badge>
          </div>
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={() => setRailOpen(false)}>
              <Menu01Icon className="h-4 w-4" />
            </Button>
            <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={clear}>
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>
        </div>
        <div className="max-h-[min(28rem,calc(100vh-8rem))] overflow-y-auto p-2">
          {plans.map((plan) => (
            <div key={plan.id} className="space-y-2 border-b border-border/60 px-2 py-2 last:border-0">
              <div className="flex items-center justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">Command Plan</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {plan.steps.length} sequential steps{plan.status ? ` • ${plan.status}` : ''}
                  </p>
                </div>
                <div className="flex items-center gap-1">
                  <Badge variant="outline" className="text-[10px]">plan</Badge>
                  {plan.status === 'running' ? (
                    <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busyPlanId === plan.id} onClick={() => void cancelPlan(plan.id)}>
                      Cancel plan
                    </Button>
                  ) : null}
                  {plan.status === 'failed' || plan.status === 'cancelled' ? (
                    <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busyPlanId === plan.id} onClick={() => void retryPlan(plan.id, plan.currentStepIndex ?? 0)}>
                      Retry
                    </Button>
                  ) : null}
                </div>
              </div>
              <div className="space-y-1.5">
                {plan.steps.map((step, index) => {
                  const runId = plan.runIdsByStep[index];
                  const run = runId ? runsById[runId] : undefined;
                  const previousRunId = index > 0 ? plan.runIdsByStep[index - 1] : undefined;
                  const previousRun = previousRunId ? runsById[previousRunId] : undefined;
                  const waitingLabel = previousRun && ACTIVE_RUN_STATUSES.has(previousRun.status)
                    ? `Waiting for step ${index}`
                    : 'Waiting';
                  return (
                    <div key={`${plan.id}-${index}`} className="rounded border border-border/60 px-2 py-2">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <p className="truncate text-sm font-medium">
                            Step {index + 1}: {step.agent_name}
                          </p>
                          <p className="truncate text-xs text-muted-foreground">
                            {run ? `${run.target_type} • ${run.execution_stage || run.id.slice(0, 8)}` : waitingLabel}
                          </p>
                        </div>
                        {run ? renderStatusBadge(run) : <Badge variant="outline" className="text-[10px]">queued next</Badge>}
                      </div>
                      {run ? <div className="mt-2">{renderRunActions(run)}</div> : null}
                    </div>
                  );
                })}
              </div>
            </div>
          ))}
          {standaloneRuns.map((run) => (
            <div key={run.id} className="space-y-2 border-b border-border/60 px-2 py-2 last:border-0">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="truncate text-sm font-medium">{targetLabel(run)}</p>
                  <p className="truncate text-xs text-muted-foreground">
                    {run.target_type} • {run.execution_stage || run.id.slice(0, 8)}
                  </p>
                </div>
                {renderStatusBadge(run)}
              </div>
              {renderRunActions(run)}
            </div>
          ))}
        </div>
      </aside>
      <CodingSessionDrawer
        sessionId={selectedRunId}
        open={!!selectedRunId}
        onOpenChange={(open) => {
          if (!open) setSelectedRunId(null);
        }}
        title="Command Run"
      />
    </>
  );
}
