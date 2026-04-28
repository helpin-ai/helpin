import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { ArrowRight01Icon, BotIcon, Cancel01Icon, Loading01Icon, Tick01Icon, ViewIcon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus, STATUS_META } from '@/components/pm/agentRunConstants';
import { agentService } from '@/lib/services/agentService';
import { commandBarService } from '@/lib/services/commandBarService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCommandBarRunStore } from '@/stores/commandBarStore';
import { PromotionDialog } from '@/components/command-bar/PromotionDialog';
import type { AgentRun, CommandBarPlanStep } from '@/lib/pmTypes';

function targetLabel(run: AgentRun) {
  return run.target_info?.title || run.target_info?.task_key || `${run.target_type} ${run.target_id.slice(0, 8)}`;
}

export function CommandBarRunRail() {
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id);
  const runIds = useCommandBarRunStore((s) => s.runIds);
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const planIds = useCommandBarRunStore((s) => s.planIds);
  const plansById = useCommandBarRunStore((s) => s.plansById);
  const railMode = useCommandBarRunStore((s) => s.railMode);
  const updateRun = useCommandBarRunStore((s) => s.updateRun);
  const hydratePlans = useCommandBarRunStore((s) => s.hydratePlans);
  const updatePlan = useCommandBarRunStore((s) => s.updatePlan);
  const setRailMode = useCommandBarRunStore((s) => s.setRailMode);
  const clear = useCommandBarRunStore((s) => s.clear);
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const [busyRunId, setBusyRunId] = useState<string | null>(null);
  const [busyPlanId, setBusyPlanId] = useState<string | null>(null);
  const [promotionRun, setPromotionRun] = useState<{ run: AgentRun; step: CommandBarPlanStep | null; planPrompt?: string } | null>(null);

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

  const planRefetchTimers = useRef<Map<string, ReturnType<typeof setTimeout>>>(new Map());
  const schedulePlanRefetch = useCallback(
    (planId: string) => {
      if (!workspaceId) return;
      const existing = planRefetchTimers.current.get(planId);
      if (existing) clearTimeout(existing);
      const timer = setTimeout(() => {
        planRefetchTimers.current.delete(planId);
        void commandBarService.getPlan(workspaceId, planId).then((res) => {
          if (res.data?.plan) updatePlan(res.data.plan, res.data.plan.runs ?? []);
        });
      }, 600);
      planRefetchTimers.current.set(planId, timer);
    },
    [updatePlan, workspaceId],
  );

  useEffect(() => {
    const timers = planRefetchTimers.current;
    return () => {
      for (const timer of timers.values()) clearTimeout(timer);
      timers.clear();
    };
  }, []);

  const findPlanIdForRun = useCallback(
    (runId: string): string | null => {
      for (const plan of plans) {
        for (const id of Object.values(plan.runIdsByStep)) {
          if (id === runId) return plan.id;
        }
      }
      return null;
    },
    [plans],
  );

  const refreshRun = useCallback(
    async (runId: string, allowUnknown = false) => {
      if (!workspaceId || (!allowUnknown && !runIds.includes(runId))) return;
      const res = await agentService.getRun(workspaceId, runId);
      if (!res.data) return;
      updateRun(res.data);
      const terminal = res.data.status === 'completed' || res.data.status === 'failed' || res.data.status === 'cancelled';
      if (terminal) {
        const planId = findPlanIdForRun(res.data.id);
        if (planId) schedulePlanRefetch(planId);
      }
    },
    [findPlanIdForRun, runIds, schedulePlanRefetch, updateRun, workspaceId],
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

  const findRunStep = useCallback(
    (run: AgentRun): { step: CommandBarPlanStep | null; planPrompt?: string } => {
      for (const plan of plans) {
        for (const [indexStr, runId] of Object.entries(plan.runIdsByStep)) {
          if (runId !== run.id) continue;
          const step = plan.steps[Number(indexStr)] ?? null;
          return { step, planPrompt: plan.prompt };
        }
      }
      return { step: null };
    },
    [plans],
  );

  const openPromotion = (run: AgentRun) => {
    const { step, planPrompt } = findRunStep(run);
    if (!step) {
      toast.error('Cannot save: this run is missing its plan context.');
      return;
    }
    setPromotionRun({ run, step, planPrompt });
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
      const runs = res.data.runs ?? (res.data.run ? [res.data.run] : []);
      updatePlan(res.data.plan, runs);
      const isFanOut = res.data.plan.plan_kind === 'fan_out';
      toast.success(isFanOut ? `Retrying ${runs.length} failed target${runs.length === 1 ? '' : 's'}` : `Retrying step ${stepIndex + 1}`);
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
          <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busy} onClick={() => openPromotion(run)}>
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

  // Cmd+. toggles the rail open/closed
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === '.') {
        e.preventDefault();
        setRailMode(railMode === 'open' ? 'peek' : 'open');
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [railMode, setRailMode]);

  // Empty state: nothing to show, render nothing (the main content can use full width)
  if (runs.length === 0 && plans.length === 0) return null;
  if (railMode === 'closed') return null;

  // Peek state: thin 48px strip on the right edge with a stack of plan/run dots
  if (railMode === 'peek') {
    return (
      <aside
        className="relative z-30 flex h-full w-12 shrink-0 flex-col items-center gap-2 border-l border-border/70 bg-background/80 py-3 backdrop-blur"
        aria-label="Command runs (peek)"
      >
        <button
          type="button"
          onClick={() => setRailMode('open')}
          className="flex flex-col items-center gap-1 text-muted-foreground hover:text-foreground"
          title="Open command runs (⌘.)"
        >
          <BotIcon className="h-4 w-4" />
          {activeCount > 0 ? (
            <Badge variant="outline" className="px-1 py-0 text-[9px]">{activeCount}</Badge>
          ) : (
            <span className="text-[9px] uppercase tracking-wider">{plans.length + standaloneRuns.length}</span>
          )}
        </button>
      </aside>
    );
  }

  // Open state: full docked column
  return (
    <>
      <aside
        className="relative z-30 flex h-full w-[22rem] shrink-0 flex-col border-l border-border/70 bg-background"
        aria-label="Command runs"
      >
        <div className="flex items-center justify-between border-b px-3 py-2">
          <div className="flex items-center gap-2">
            <BotIcon className="h-4 w-4 text-muted-foreground" />
            <span className="text-sm font-medium">Command Runs</span>
            {activeCount > 0 ? (
              <Badge variant="outline" className="text-[10px]">{activeCount} active</Badge>
            ) : null}
          </div>
          <div className="flex items-center gap-1">
            <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={() => setRailMode('peek')} title="Collapse to peek (⌘.)">
              <ArrowRight01Icon className="h-4 w-4" />
            </Button>
            <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={clear} title="Clear all">
              <Cancel01Icon className="h-4 w-4" />
            </Button>
          </div>
        </div>
        <div className="flex-1 overflow-y-auto p-2">
          {plans.map((plan) => {
            const stepWord = plan.steps.length === 1 ? 'step' : 'steps';
            const isSingle = plan.steps.length === 1;
            const isOneShot = plan.planKind === 'one_shot_command' || plan.steps.some((step) => step.plan_kind === 'one_shot_command');
            const isFanOut = plan.planKind === 'fan_out' || plan.steps.some((step) => step.plan_kind === 'fan_out');
            return (
              <div key={plan.id} className="space-y-2 border-b border-border/60 px-2 py-2 last:border-0">
                {!isSingle ? (
                  <div className="flex items-center justify-between gap-3">
                    <div className="min-w-0">
                      <p className="truncate text-sm font-medium">{plan.prompt || 'Command plan'}</p>
                      <p className="truncate text-xs text-muted-foreground">
                        {isFanOut ? `${plan.steps.length} targets · fan-out` : isOneShot ? 'one-shot agent' : `${plan.steps.length} ${stepWord}`}
                        {plan.status ? ` · ${plan.status}` : ''}
                      </p>
                    </div>
                    <div className="flex items-center gap-1">
                      {plan.status === 'running' ? (
                        <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busyPlanId === plan.id} onClick={() => void cancelPlan(plan.id)}>
                          Cancel
                        </Button>
                      ) : null}
                      {plan.status === 'failed' || plan.status === 'cancelled' ? (
                        <Button type="button" variant="ghost" size="sm" className="h-7" disabled={busyPlanId === plan.id} onClick={() => void retryPlan(plan.id, plan.currentStepIndex ?? 0)}>
                          {isFanOut ? 'Retry failed' : 'Retry'}
                        </Button>
                      ) : null}
                    </div>
                  </div>
                ) : null}
                <div className="space-y-1.5">
                  {plan.steps.map((step, index) => {
                    const runId = plan.runIdsByStep[index];
                    const run = runId ? runsById[runId] : undefined;
                    const previousRunId = index > 0 ? plan.runIdsByStep[index - 1] : undefined;
                    const previousRun = previousRunId ? runsById[previousRunId] : undefined;
                    const waitingLabel = isFanOut
                      ? 'Queued'
                      : previousRun && ACTIVE_RUN_STATUSES.has(previousRun.status)
                      ? `Waiting for step ${index}`
                      : 'Queued';
                    const actionLabel = step.instructions || (run && targetLabel(run)) || step.agent_name;
                    return (
                      <div key={`${plan.id}-${index}`} className="rounded border border-border/60 px-2 py-2">
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0">
                            <p className="line-clamp-2 text-sm font-medium">{actionLabel}</p>
                            <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                              {isFanOut ? `Target ${index + 1} · ` : !isSingle ? `Step ${index + 1} · ` : ''}
                              {step.agent_name}{step.plan_kind === 'one_shot_command' ? ' · one-shot' : step.plan_kind === 'fan_out' ? ' · fan-out' : ''} · {step.target.entity_type.replace('_', ' ')}
                              {run?.execution_stage ? ` · ${run.execution_stage}` : ''}
                            </p>
                          </div>
                          {run ? renderStatusBadge(run) : <Badge variant="outline" className="text-[10px]">{waitingLabel}</Badge>}
                        </div>
                        {run ? <div className="mt-2">{renderRunActions(run)}</div> : null}
                      </div>
                    );
                  })}
                </div>
                {isSingle && plan.status === 'failed' ? (
                  <Button type="button" variant="ghost" size="sm" className="h-7 w-full" disabled={busyPlanId === plan.id} onClick={() => void retryPlan(plan.id, 0)}>
                    Retry
                  </Button>
                ) : null}
              </div>
            );
          })}
          {standaloneRuns.map((run) => (
            <div key={run.id} className="space-y-2 border-b border-border/60 px-2 py-2 last:border-0">
              <div className="flex items-start justify-between gap-3">
                <div className="min-w-0">
                  <p className="line-clamp-2 text-sm font-medium">{targetLabel(run)}</p>
                  <p className="mt-0.5 truncate text-[11px] text-muted-foreground">
                    {run.target_type.replace('_', ' ')}
                    {run.execution_stage ? ` · ${run.execution_stage}` : ''}
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
      <PromotionDialog
        open={!!promotionRun}
        onOpenChange={(open) => {
          if (!open) setPromotionRun(null);
        }}
        workspaceId={workspaceId}
        run={promotionRun?.run ?? null}
        step={promotionRun?.step ?? null}
        planPrompt={promotionRun?.planPrompt}
      />
    </>
  );
}
