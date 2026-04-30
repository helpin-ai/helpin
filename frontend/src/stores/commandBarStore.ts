import { create } from 'zustand';
import type { AgentRun, CommandBarPlanStep, CommandBarPlanSummary } from '@/lib/pmTypes';

export interface CommandBarRunPlan {
  id: string;
  steps: CommandBarPlanStep[];
  runIdsByStep: Record<number, string>;
  planKind?: CommandBarPlanSummary['plan_kind'];
  status?: CommandBarPlanSummary['status'];
  prompt?: string;
  currentStepIndex?: number;
}

export type RailMode = 'closed' | 'peek' | 'open';
export type RailFilter = 'all' | 'running' | 'queued' | 'failed';
export type DockViewMode = 'conversation' | 'list';

const RAIL_MODE_KEY = 'helpin:cmdk-rail:mode';
const RAIL_FILTER_KEY = 'helpin:cmdk-rail:filter';

function loadRailMode(): RailMode {
  if (typeof window === 'undefined') return 'closed';
  try {
    const raw = window.localStorage.getItem(RAIL_MODE_KEY);
    if (raw === 'closed' || raw === 'peek' || raw === 'open') return raw;
  } catch {
    /* ignore */
  }
  return 'closed';
}

function persistRailMode(mode: RailMode) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(RAIL_MODE_KEY, mode);
  } catch {
    /* ignore */
  }
}

function loadRailFilter(): RailFilter {
  if (typeof window === 'undefined') return 'all';
  try {
    const raw = window.localStorage.getItem(RAIL_FILTER_KEY);
    if (raw === 'all' || raw === 'running' || raw === 'queued' || raw === 'failed') return raw;
  } catch {
    /* ignore */
  }
  return 'all';
}

function persistRailFilter(filter: RailFilter) {
  if (typeof window === 'undefined') return;
  try {
    window.localStorage.setItem(RAIL_FILTER_KEY, filter);
  } catch {
    /* ignore */
  }
}

interface CommandBarRunState {
  runIds: string[];
  runsById: Record<string, AgentRun>;
  planIds: string[];
  plansById: Record<string, CommandBarRunPlan>;
  railMode: RailMode;
  railFilter: RailFilter;
  viewMode: DockViewMode;
  listFilter: string;
  selectedRunId: string | null;
  /**
   * Set true when the user explicitly opens/closes the rail in this session.
   * hydratePlans uses this to avoid clobbering a user's manual click that
   * lands before the initial plans fetch resolves.
   */
  userSetRailMode: boolean;
  addRuns: (runs: AgentRun[]) => void;
  addPlan: (plan: CommandBarRunPlan, runs?: AgentRun[]) => void;
  hydratePlans: (plans: CommandBarPlanSummary[]) => void;
  updateRun: (run: AgentRun) => void;
  updatePlan: (plan: CommandBarPlanSummary, runs?: AgentRun[]) => void;
  setRailMode: (mode: RailMode) => void;
  setRailFilter: (filter: RailFilter) => void;
  setViewMode: (mode: DockViewMode) => void;
  setListFilter: (filter: string) => void;
  setSelectedRunId: (id: string | null) => void;
  clear: () => void;
}

export const useCommandBarRunStore = create<CommandBarRunState>((set) => ({
  runIds: [],
  runsById: {},
  planIds: [],
  plansById: {},
  railMode: loadRailMode(),
  railFilter: loadRailFilter(),
  viewMode: 'conversation',
  listFilter: '',
  selectedRunId: null,
  userSetRailMode: false,
  addRuns: (runs) =>
    set((state) => {
      const runsById = { ...state.runsById };
      const ids = new Set(state.runIds);
      for (const run of runs) {
        runsById[run.id] = run;
        ids.add(run.id);
      }
      return { runsById, runIds: Array.from(ids) };
    }),
  addPlan: (plan, runs = []) =>
    set((state) => {
      const runsById = { ...state.runsById };
      const runIds = new Set(state.runIds);
      const planIds = new Set(state.planIds);
      for (const run of runs) {
        runsById[run.id] = run;
        runIds.add(run.id);
      }
      planIds.add(plan.id);
      return {
        runsById,
        runIds: Array.from(runIds),
        planIds: Array.from(planIds),
        plansById: { ...state.plansById, [plan.id]: plan },
      };
    }),
  hydratePlans: (plans) =>
    set((state) => {
      const runsById = { ...state.runsById };
      const runIds = new Set(state.runIds);
      const planIds = new Set(state.planIds);
      const plansById = { ...state.plansById };
      for (const plan of plans) {
        planIds.add(plan.id);
        plansById[plan.id] = {
          id: plan.id,
          steps: plan.steps,
          runIdsByStep: plan.run_ids_by_step ?? {},
          planKind: plan.plan_kind,
          status: plan.status,
          prompt: plan.prompt,
          currentStepIndex: plan.current_step_index,
        };
        for (const run of plan.runs ?? []) {
          runsById[run.id] = run;
          runIds.add(run.id);
        }
      }
      // On page load, only restore the rail's saved open/peek state if there
      // is actual live work to show. Stale history from a prior session
      // shouldn't pop the rail open on every page navigation. But if the user
      // already clicked the sidebar Runs button (or otherwise set mode in this
      // session), respect that — don't clobber it with a force-close.
      const hasActive = Object.values(runsById).some(
        (r) => r.status === 'running' || r.status === 'queued' || r.status === 'paused',
      );
      const railMode =
        state.userSetRailMode || hasActive ? state.railMode : 'closed';
      return {
        runsById,
        runIds: Array.from(runIds),
        planIds: Array.from(planIds),
        plansById,
        railMode,
      };
    }),
  updatePlan: (plan, runs = []) =>
    set((state) => {
      const runsById = { ...state.runsById };
      const runIds = new Set(state.runIds);
      for (const run of runs) {
        runsById[run.id] = run;
        runIds.add(run.id);
      }
      return {
        runsById,
        runIds: Array.from(runIds),
        planIds: Array.from(new Set([...state.planIds, plan.id])),
        plansById: {
          ...state.plansById,
          [plan.id]: {
            id: plan.id,
            steps: plan.steps,
            runIdsByStep: plan.run_ids_by_step ?? {},
            planKind: plan.plan_kind,
            status: plan.status,
            prompt: plan.prompt,
            currentStepIndex: plan.current_step_index,
          },
        },
      };
    }),
  updateRun: (run) =>
    set((state) => {
      const planMeta = getCommandBarPlanMeta(run);
      const runKnown = state.runIds.includes(run.id);
      const planKnown = planMeta?.planId ? Boolean(state.plansById[planMeta.planId]) : false;
      const parentKnown = run.parent_run_id ? state.runIds.includes(run.parent_run_id) : false;
      if (!runKnown && !planKnown && !parentKnown) return state;

      const runIds = new Set(state.runIds);
      runIds.add(run.id);

      const plansById = { ...state.plansById };
      const planIds = new Set(state.planIds);
      if (planMeta?.planId) {
        const existing = plansById[planMeta.planId];
        plansById[planMeta.planId] = {
          id: planMeta.planId,
          steps: existing?.steps?.length ? existing.steps : planMeta.steps,
          planKind: existing?.planKind ?? planMeta.planKind,
          status: existing?.status,
          prompt: existing?.prompt,
          currentStepIndex: planMeta.stepIndex,
          runIdsByStep: {
            ...(existing?.runIdsByStep ?? {}),
            [planMeta.stepIndex]: run.id,
          },
        };
        planIds.add(planMeta.planId);
      }

      return {
        runsById: { ...state.runsById, [run.id]: run },
        runIds: Array.from(runIds),
        planIds: Array.from(planIds),
        plansById,
      };
    }),
  setRailMode: (mode) => {
    persistRailMode(mode);
    set({ railMode: mode, userSetRailMode: true });
  },
  setRailFilter: (filter) => {
    persistRailFilter(filter);
    set({ railFilter: filter });
  },
  setViewMode: (mode) => set({ viewMode: mode }),
  setListFilter: (filter) => set({ listFilter: filter }),
  setSelectedRunId: (id) => set({ selectedRunId: id }),
  clear: () => set({ runIds: [], runsById: {}, planIds: [], plansById: {}, selectedRunId: null }),
}));

function getCommandBarPlanMeta(run: AgentRun): { planId: string; stepIndex: number; steps: CommandBarPlanStep[]; planKind?: CommandBarPlanSummary['plan_kind'] } | null {
  const input = run.input as {
    trigger?: {
      source?: string;
      trigger_type?: string;
      context?: unknown;
    };
  } | null;
  const trigger = input?.trigger;
  if (trigger?.source !== 'command_bar' || trigger?.trigger_type !== 'command_bar') return null;
  const context = trigger.context as {
    plan_id?: string;
    step_index?: number;
    steps?: CommandBarPlanStep[];
  } | undefined;
  if (!context?.plan_id || typeof context.step_index !== 'number' || !Array.isArray(context.steps)) return null;
  return { planId: context.plan_id, stepIndex: context.step_index, steps: context.steps, planKind: context.steps[0]?.plan_kind };
}
