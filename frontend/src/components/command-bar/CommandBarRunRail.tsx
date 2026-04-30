import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { toast } from 'sonner';
import { ArrowUpRight01Icon, BotIcon, Cancel01Icon } from '@/lib/icons';
import { Button } from '@/components/ui/button';
import { Badge } from '@/components/ui/badge';
import { CodingSessionDrawer } from '@/components/pm/CodingSession/CodingSessionDrawer';
import { ACTIVE_RUN_STATUSES, getAgentRunDisplayStatus } from '@/components/pm/agentRunConstants';
import { agentService } from '@/lib/services/agentService';
import { commandBarService } from '@/lib/services/commandBarService';
import { useWorkspaceStore } from '@/stores/workspaceStore';
import { useCommandBarRunStore, type CommandBarRunPlan, type RailFilter } from '@/stores/commandBarStore';
import { PromotionDialog } from '@/components/command-bar/PromotionDialog';
import { CommandPlanTimeline, StandaloneRunTimeline, type RunAction } from '@/components/command-bar/CommandRunTimeline';
import { cn } from '@/lib/utils';
import type { AgentRun, CommandBarPlanStep } from '@/lib/pmTypes';

type RailState = 'attention' | 'running' | 'queued' | 'completed';
type RailItem =
  | { id: string; type: 'plan'; state: RailState; plan: CommandBarRunPlan }
  | { id: string; type: 'run'; state: RailState; run: AgentRun };

const FILTERS: Array<{ key: RailFilter; label: string }> = [
  { key: 'all', label: 'All' },
  { key: 'running', label: 'Running' },
  { key: 'queued', label: 'Queued' },
  { key: 'failed', label: 'Failed' },
];

function matchesFilter(state: RailState, filter: RailFilter): boolean {
  if (filter === 'all') return true;
  if (filter === 'failed') return state === 'attention';
  return state === filter;
}

export function CommandBarRunRail() {
  const workspaceId = useWorkspaceStore((s) => s.currentWorkspace?.id);
  const runIds = useCommandBarRunStore((s) => s.runIds);
  const runsById = useCommandBarRunStore((s) => s.runsById);
  const planIds = useCommandBarRunStore((s) => s.planIds);
  const plansById = useCommandBarRunStore((s) => s.plansById);
  const railMode = useCommandBarRunStore((s) => s.railMode);
  const updateRun = useCommandBarRunStore((s) => s.updateRun);
  const addRuns = useCommandBarRunStore((s) => s.addRuns);
  const hydratePlans = useCommandBarRunStore((s) => s.hydratePlans);
  const updatePlan = useCommandBarRunStore((s) => s.updatePlan);
  const setRailMode = useCommandBarRunStore((s) => s.setRailMode);
  const filter = useCommandBarRunStore((s) => s.railFilter);
  const setFilter = useCommandBarRunStore((s) => s.setRailFilter);
  const clear = useCommandBarRunStore((s) => s.clear);
  const selectedRunId = useCommandBarRunStore((s) => s.selectedRunId);
  const setSelectedRunId = useCommandBarRunStore((s) => s.setSelectedRunId);
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

  const items = useMemo<RailItem[]>(() => {
    const planItems = plans.map((plan) => ({
      id: `plan-${plan.id}`,
      type: 'plan' as const,
      state: classifyPlan(plan, runsById),
      plan,
    }));
    const runItems = standaloneRuns.map((run) => ({
      id: `run-${run.id}`,
      type: 'run' as const,
      state: classifyRun(run),
      run,
    }));
    return [...planItems, ...runItems];
  }, [plans, runsById, standaloneRuns]);

  const counts = useMemo(() => {
    const next: Record<RailFilter, number> = { all: items.length, running: 0, queued: 0, failed: 0 };
    for (const item of items) {
      if (item.state === 'attention') next.failed += 1;
      else if (item.state === 'running') next.running += 1;
      else if (item.state === 'queued') next.queued += 1;
    }
    return next;
  }, [items]);

  const visibleItems = items.filter((item) => matchesFilter(item.state, filter));
  const activeCount = runs.filter((run) => ACTIVE_RUN_STATUSES.has(run.status)).length;

  useEffect(() => {
    if (!workspaceId) return;
    let cancelled = false;
    void commandBarService.listPlans(workspaceId, 10).then((res) => {
      if (!cancelled && res.data?.plans) hydratePlans(res.data.plans);
    });
    void agentService.listRecentRuns(workspaceId, 20).then((res) => {
      if (!cancelled && res.data?.runs?.length) addRuns(res.data.runs);
    });
    return () => {
      cancelled = true;
    };
  }, [addRuns, hydratePlans, workspaceId]);

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

  const runAction = async (run: AgentRun, action: RunAction) => {
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

  const dismissAll = async () => {
    if (!workspaceId) return;
    const visiblePlanIds = visibleItems.flatMap((item) => (item.type === 'plan' ? [item.plan.id] : []));
    if (visiblePlanIds.length === 0) {
      clear();
      return;
    }
    const res = await commandBarService.dismissPlans(workspaceId, visiblePlanIds);
    if (res.error) {
      toast.error(res.error);
      return;
    }
    clear();
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
      const retryRuns = res.data.runs ?? (res.data.run ? [res.data.run] : []);
      updatePlan(res.data.plan, retryRuns);
      const isFanOut = res.data.plan.plan_kind === 'fan_out';
      toast.success(isFanOut ? `Retrying ${retryRuns.length} failed target${retryRuns.length === 1 ? '' : 's'}` : `Retrying step ${stepIndex + 1}`);
    } finally {
      setBusyPlanId(null);
    }
  };

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

  if (runs.length === 0 && plans.length === 0) return null;
  if (railMode === 'closed') return null;

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
            <span className="text-[9px] uppercase tracking-wider">{items.length}</span>
          )}
        </button>
      </aside>
    );
  }

  return (
    <>
      <aside
        className="relative z-30 flex h-full w-[23.5rem] shrink-0 flex-col border-l border-border/70 bg-background"
        aria-label="Command runs"
      >
        <div className="border-b border-border/70">
          <div className="flex items-center justify-between px-3 py-2">
            <div className="flex min-w-0 items-center gap-2">
              <BotIcon className="h-4 w-4 text-muted-foreground" />
              <span className="text-sm font-medium">Command runs</span>
            </div>
            <div className="flex items-center gap-1">
              <kbd className="hidden h-5 items-center rounded border border-border/70 bg-muted px-1.5 text-[10px] font-medium text-muted-foreground sm:inline-flex" title="Toggle command runs">⌘K</kbd>
              <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={() => setRailMode('peek')} title="Collapse to peek (⌘.)">
                <ArrowUpRight01Icon className="h-4 w-4" />
              </Button>
              <Button type="button" variant="ghost" size="icon" className="h-7 w-7" onClick={() => void dismissAll()} title="Clear all">
                <Cancel01Icon className="h-4 w-4" />
              </Button>
            </div>
          </div>
          <div className="flex gap-3 overflow-x-auto px-3 pb-2">
            {FILTERS.map((item) => {
              const active = filter === item.key;
              return (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => setFilter(item.key)}
                  className={cn(
                    'group/tab shrink-0 -mb-px border-b-2 border-transparent pb-1.5 text-[11px] transition-colors',
                    active
                      ? 'border-foreground text-foreground'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  <span className={active ? 'font-medium' : ''}>{item.label}</span>
                  <span className={cn('ml-1.5 tabular-nums', active ? 'text-foreground/70' : 'text-muted-foreground/70')}>{counts[item.key]}</span>
                </button>
              );
            })}
          </div>
        </div>

        <div className="flex-1 overflow-y-auto">
          {visibleItems.length === 0 ? (
            <div className="px-4 py-8 text-center text-sm text-muted-foreground">No command runs in this view.</div>
          ) : (
            renderSections(visibleItems).map((section) => (
              <section key={section.key}>
                <div className="sticky top-0 z-10 bg-background/95 px-3 pb-1.5 pt-3 text-[10px] font-semibold uppercase tracking-[0.12em] text-muted-foreground backdrop-blur">
                  {section.label}
                </div>
                {section.items.map((item) => item.type === 'plan' ? (
                  <CommandPlanTimeline
                    key={item.id}
                    plan={item.plan}
                    runsById={runsById}
                    busyPlanId={busyPlanId}
                    busyRunId={busyRunId}
                    onOpenRun={setSelectedRunId}
                    onRunAction={(run, action) => void runAction(run, action)}
                    onCancelPlan={(planId) => void cancelPlan(planId)}
                    onRetryPlan={(planId, stepIndex) => void retryPlan(planId, stepIndex)}
                    onPromoteRun={openPromotion}
                  />
                ) : (
                  <StandaloneRunTimeline
                    key={item.id}
                    run={item.run}
                    busyRunId={busyRunId}
                    onOpenRun={setSelectedRunId}
                    onRunAction={(run, action) => void runAction(run, action)}
                  />
                ))}
              </section>
            ))
          )}
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

function classifyPlan(plan: CommandBarRunPlan, runsById: Record<string, AgentRun>): RailState {
  const runs = Object.values(plan.runIdsByStep).map((runId) => runsById[runId]).filter(Boolean);
  if (plan.status === 'failed' || plan.status === 'cancelled') return 'attention';
  if (runs.some((run) => classifyRun(run) === 'attention')) return 'attention';
  if (plan.status === 'completed') return 'completed';
  if (runs.some((run) => ACTIVE_RUN_STATUSES.has(run.status))) return 'running';
  return 'queued';
}

function classifyRun(run: AgentRun): RailState {
  const displayStatus = getAgentRunDisplayStatus(run);
  if (displayStatus === 'awaiting_approval' || displayStatus === 'awaiting_input' || displayStatus === 'awaiting_auth') return 'attention';
  if (run.status === 'failed' || run.status === 'cancelled') return 'attention';
  if (ACTIVE_RUN_STATUSES.has(run.status)) return 'running';
  if (run.status === 'completed') return 'completed';
  return 'queued';
}

function renderSections(items: RailItem[]) {
  const order: Array<{ key: RailState; label: string }> = [
    { key: 'running', label: 'Running now' },
    { key: 'queued', label: 'Queued' },
    { key: 'attention', label: 'Failed · Needs attention' },
    { key: 'completed', label: 'Completed' },
  ];
  return order
    .map((section) => ({ ...section, items: items.filter((item) => item.state === section.key) }))
    .filter((section) => section.items.length > 0);
}
