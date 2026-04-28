import { create } from 'zustand';
import type { AgentRun } from '@/lib/pmTypes';

interface CommandBarRunState {
  runIds: string[];
  runsById: Record<string, AgentRun>;
  railOpen: boolean;
  addRuns: (runs: AgentRun[]) => void;
  updateRun: (run: AgentRun) => void;
  setRailOpen: (open: boolean) => void;
  clear: () => void;
}

export const useCommandBarRunStore = create<CommandBarRunState>((set) => ({
  runIds: [],
  runsById: {},
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
  updateRun: (run) =>
    set((state) => {
      if (!state.runIds.includes(run.id)) return state;
      return { runsById: { ...state.runsById, [run.id]: run } };
    }),
  setRailOpen: (railOpen) => set({ railOpen }),
  clear: () => set({ runIds: [], runsById: {}, railOpen: true }),
}));
