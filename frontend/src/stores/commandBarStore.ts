import { create } from 'zustand';
import type { AgentRun, CommandBarPlanStep, CommandBarPlanSummary } from '@/lib/pmTypes';

export interface CommandBarRunPlan {
  id: string;
  steps: CommandBarPlanStep[];
  runIdsByStep: Record<number, string>;
  status?: CommandBarPlanSummary['status'];
  prompt?: string;
  currentStepIndex?: number;
}

interface CommandBarRunState {
  runIds: string[];
  runsById: Record<string, AgentRun>;
  planIds: string[];
  plansById: Record<string, CommandBarRunPlan>;
  railOpen: boolean;
  addRuns: (runs: AgentRun[]) => void;
  addPlan: (plan: CommandBarRunPlan, runs?: AgentRun[]) => void;
  hydratePlans: (plans: CommandBarPlanSummary[]) => void;
  updateRun: (run: AgentRun) => void;
  updatePlan: (plan: CommandBarPlanSummary, runs?: AgentRun[]) => void;
  setRailOpen: (open: boolean) => void;
  clear: () => void;
}

export const useCommandBarRunStore = create<CommandBarRunState>((set) => ({
  runIds: [],
  runsById: {},
  planIds: [],
  plansById: {},
  railOpen: true,
  addRuns: (runs) =>
    set((state) => {
      const runsById = { ...state.runsById };
      const ids = new Set(state.runIds);
      for (const run of runs) {
        runsById[run.id] = run;
        ids.add(run.id);
      }
      return { runsById, runIds: Array.from(ids), railOpen: true };
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
        railOpen: true,
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
          status: plan.status,
          prompt: plan.prompt,
          currentStepIndex: plan.current_step_index,
        };
        for (const run of plan.runs ?? []) {
          runsById[run.id] = run;
          runIds.add(run.id);
        }
      }
      return {
        runsById,
        runIds: Array.from(runIds),
        planIds: Array.from(planIds),
        plansById,
        railOpen: plans.length ? true : state.railOpen,
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
  setRailOpen: (railOpen) => set({ railOpen }),
  clear: () => set({ runIds: [], runsById: {}, planIds: [], plansById: {}, railOpen: true }),
}));

function getCommandBarPlanMeta(run: AgentRun): { planId: string; stepIndex: number; steps: CommandBarPlanStep[] } | null {
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
  return { planId: context.plan_id, stepIndex: context.step_index, steps: context.steps };
}
