import { create } from 'zustand';

interface TaskPanelState {
  taskId: string | null;
  requestKey: number;
  lastClosedTaskId: string | null;
  lastClosedAt: number;
  openTask: (taskId: string) => void;
  close: () => void;
  rememberClosedTask: (taskId: string) => void;
  shouldSuppressOpen: (taskId: string) => boolean;
}

const RECENTLY_CLOSED_TASK_SUPPRESSION_MS = 2_000;

export function clearTaskSearchParam() {
  if (typeof window === 'undefined') return;

  const url = new URL(window.location.href);
  if (url.searchParams.has('task')) {
    url.searchParams.delete('task');
    window.history.replaceState({}, '', url.toString());
  }
}

export const useTaskPanelStore = create<TaskPanelState>((set, get) => ({
  taskId: null,
  requestKey: 0,
  lastClosedTaskId: null,
  lastClosedAt: 0,
  openTask: (taskId) =>
    set({
      taskId,
      requestKey: get().requestKey + 1,
    }),
  close: () => {
    clearTaskSearchParam();
    set({ taskId: null });
  },
  rememberClosedTask: (taskId) =>
    set({
      lastClosedTaskId: taskId,
      lastClosedAt: Date.now(),
    }),
  shouldSuppressOpen: (taskId) => {
    const state = get();
    return (
      state.lastClosedTaskId === taskId &&
      Date.now() - state.lastClosedAt < RECENTLY_CLOSED_TASK_SUPPRESSION_MS
    );
  },
}));
