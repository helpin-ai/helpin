import { create } from 'zustand';
import type {
  CreateTaskRequest,
  PMView,
  Task,
  TaskGroup,
  TaskMemberColumn,
  TaskStateColumn,
  StateType,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmTaskService } from '@/lib/services/pmTaskService';
import { pmViewService } from '@/lib/services/pmViewService';
import { getDefaultViews, isDefaultView } from '@/lib/pmDefaultViews';
import { createDebouncedBoardFetchScheduler } from './pmBoardFetchScheduler';
import { createPMDnDTraceID, logPMDnD, summarizePMDnDColumn } from '@/lib/pmDnDDebug';

export type BoardFilters = Record<string, string | undefined>;

/** Default number of tasks loaded per column on initial board fetch. */
const PER_STATE_LIMIT = 25;

/** Debounce window for board filter fetches. */
const FILTER_FETCH_DEBOUNCE_MS = 250;

/** Number of additional tasks fetched when a column nears the bottom. */
const COLUMN_PAGE_SIZE = 50;

interface MovePayload {
  workspaceId: string;
  taskId: string;
  fromStateId: string;
  toStateId: string;
  toIndex: number;
  debugTraceID?: string;
}

interface MemberMovePayload {
  workspaceId: string;
  taskId: string;
  fromMemberId: string | null;
  toMemberId: string | null;
  toIndex: number;
}

interface PMBoardState {
  workspaceId: string | null;
  workflows: WorkflowWithStates[];
  workflow: WorkflowWithStates | null;
  columns: TaskStateColumn[];
  loading: boolean;
  error: string | null;
  teamId: string | null;
  filters: BoardFilters;

  /** Map of state IDs currently loading more tasks. */
  columnLoading: Record<string, boolean>;

  // Member board state
  memberColumns: TaskMemberColumn[];
  memberColumnLoading: Record<string, boolean>;

  // View state
  views: PMView[];
  activeViewId: string | null;
  savedViewFilters: BoardFilters;

  loadBoard: (workspaceId: string, workflowId?: string) => Promise<void>;
  setWorkflow: (workflowId: string) => Promise<void>;
  setTeamFilter: (teamId: string | null) => Promise<void>;
  setFilters: (filters: BoardFilters) => Promise<void>;
  refreshBoard: () => Promise<void>;
  createTask: (payload: CreateTaskRequest) => Promise<Task | null>;
  moveTask: (payload: MovePayload) => Promise<void>;
  loadMoreColumn: (stateId: string) => Promise<void>;

  /** Incremental patch: add, update, remove, or move a single task in the board state. Returns true when reconciled locally. */
  patchTask: (action: 'created' | 'updated' | 'deleted' | 'moved', taskId: string, task?: Task) => boolean;

  // Member board actions
  loadMemberBoard: (memberIds?: string[], includeEmpty?: boolean) => Promise<void>;
  loadMoreMemberColumn: (memberId: string | null) => Promise<void>;
  moveMemberTask: (payload: MemberMovePayload) => Promise<void>;


  // View actions
  loadViews: (workspaceId: string, currentMemberId: string) => Promise<void>;
  applyView: (view: PMView) => void;
  saveCurrentAsView: (workspaceId: string, name: string, isShared: boolean) => Promise<PMView | null>;
  saveChangesToView: (workspaceId: string) => Promise<void>;
  discardChanges: () => void;
  deleteView: (workspaceId: string, id: string) => Promise<void>;
  updateView: (workspaceId: string, id: string, payload: Parameters<typeof pmViewService.update>[2]) => Promise<PMView | null>;
}

const cloneColumns = (columns: TaskStateColumn[]) =>
  columns.map((column) => ({
    ...column,
    tasks: [...column.tasks],
    task_groups: column.task_groups?.map((group: TaskGroup) => ({
      ...group,
      tasks: [...group.tasks],
    })) ?? [],
  }));

const groupedStateType: StateType = 'done';

const compareTasks = (a: Task, b: Task, stateType?: StateType) => {
  if (stateType === groupedStateType) {
    const aSortKey = a.completed_at ?? a.moved_at ?? a.updated_at;
    const bSortKey = b.completed_at ?? b.moved_at ?? b.updated_at;
    if (aSortKey !== bSortKey) return bSortKey.localeCompare(aSortKey);
    const updatedCmp = b.updated_at.localeCompare(a.updated_at);
    if (updatedCmp !== 0) return updatedCmp;
  }
  if (a.position !== b.position) return a.position - b.position;
  return b.updated_at.localeCompare(a.updated_at);
};

const sortTasks = (tasks: Task[], stateType?: StateType) => [...tasks].sort((a, b) => compareTasks(a, b, stateType));

const isGroupedColumn = (column: Pick<TaskStateColumn, 'state'>) => column.state.state_type === groupedStateType;

const reindexLoadedTasks = (column: TaskStateColumn) => {
  if (isGroupedColumn(column)) return;
  column.tasks = column.tasks.map((task, index) => ({
    ...task,
    position: index,
  }));
};

const taskMatchesFilters = (task: Task, teamId: string | null, filters: BoardFilters) => {
  const matchesCsv = (actual: string | undefined, value: string | undefined) => {
    if (!value) return true;
    return value.split(',').includes(actual ?? '');
  };

  if (teamId && task.team_id !== teamId) return false;
  if (!matchesCsv(task.priority, filters.priority)) return false;
  if (!matchesCsv(task.severity, filters.severity)) return false;
  if (!matchesCsv(task.task_type, filters.task_type)) return false;
  if (!matchesCsv(task.epic_id, filters.epic_id)) return false;
  if (!matchesCsv(task.sprint_id, filters.sprint_id)) return false;
  if (!matchesCsv(task.owner_member_id, filters.owner_member_id)) return false;
  if (!matchesCsv(task.requester_member_id, filters.requester_member_id)) return false;
  if (filters.blocked && String(task.blocked) !== filters.blocked) return false;
  if (filters.blocking && String(task.is_blocking_other_task ?? false) !== filters.blocking) return false;
  if (filters.updated_after && task.updated_at < filters.updated_after) return false;
  return true;
};

const hasAmbiguousPatchFilters = (filters: BoardFilters) => Boolean(
	filters.label_id ||
	filters.contact_id ||
	filters.company_id ||
	filters.deal_id ||
	filters.support_conversation_id,
);

const updateColumnTotals = (column: TaskStateColumn, countDelta: number, pointDelta: number) => {
  column.task_count = Math.max(0, column.task_count + countDelta);
  column.point_total = Math.max(0, column.point_total + pointDelta);
  column.has_more = column.tasks.length < column.task_count;
};

const removeLoadedTask = (column: TaskStateColumn, taskId: string) => {
  const index = column.tasks.findIndex((candidate) => candidate.id === taskId);
  if (index === -1) return null;
  const [removed] = column.tasks.splice(index, 1);
  return removed ?? null;
};

const replaceGroupedTask = (column: TaskStateColumn, task: Task) => {
  if (!column.task_groups?.length) return false;
  let replaced = false;
  column.task_groups = column.task_groups.map((group) => {
    const taskIndex = group.tasks.findIndex((candidate) => candidate.id === task.id);
    if (taskIndex === -1) return group;
    const tasks = [...group.tasks];
    tasks[taskIndex] = task;
    replaced = true;
    return { ...group, tasks };
  });
  return replaced;
};

const mergeTaskGroups = (existing: TaskGroup[] | undefined, incoming: TaskGroup[] | undefined) => {
  if (!incoming?.length) return existing ?? [];
  if (!existing?.length) {
    return incoming.map((group) => ({ ...group, tasks: [...group.tasks] }));
  }

  const merged = existing.map((group) => ({
    ...group,
    tasks: [...group.tasks],
  }));
  const groupIndexByKey = new Map(merged.map((group, index) => [group.key, index]));

  for (const nextGroup of incoming) {
    const existingIndex = groupIndexByKey.get(nextGroup.key);
    if (existingIndex === undefined) {
      groupIndexByKey.set(nextGroup.key, merged.length);
      merged.push({ ...nextGroup, tasks: [...nextGroup.tasks] });
      continue;
    }

    const existingGroup = merged[existingIndex];
    const existingIds = new Set(existingGroup.tasks.map((task) => task.id));
    existingGroup.tasks = [
      ...existingGroup.tasks,
      ...nextGroup.tasks.filter((task) => !existingIds.has(task.id)),
    ];
  }

  return merged;
};

/** Preserve board-enriched display fields (owner_name, epic_name) from existing task when IDs match. */
const mergeEnrichedFields = (incoming: Task, existing: Task): Task => ({
  ...incoming,
  owner_name: incoming.owner_name ?? (
    incoming.owner_member_id === existing.owner_member_id
      ? existing.owner_name
      : undefined
  ),
  epic_name: incoming.epic_name ?? (incoming.epic_id === existing.epic_id ? existing.epic_name : undefined),
});

const upsertLoadedTask = (column: TaskStateColumn, task: Task) => {
  const nextTasks = sortTasks([
    ...column.tasks.filter((candidate) => candidate.id !== task.id),
    task,
  ], column.state.state_type);
  column.tasks = nextTasks;
};

const buildApiFilters = (teamId: string | null, filters: BoardFilters): Record<string, string | undefined> => {
  const result: Record<string, string | undefined> = { ...filters };
  if (teamId) result.team_id = teamId;
  return result;
};

const sortColumns = (data: TaskStateColumn[]) =>
  [...data].sort((a, b) => a.state.position - b.state.position);

type BoardFetchArgs = {
  workspaceId: string;
  workflowId: string;
  teamId: string | null;
  filters: BoardFilters;
};

// ── localStorage helpers for user preferences ─────────────────────────
const WORKFLOW_KEY = (wsId: string) => `pm_workflow_${wsId}`;
const VIEW_KEY = (wsId: string) => `pm_active_view_${wsId}`;

function getSavedWorkflowId(workspaceId: string): string | null {
  try { return localStorage.getItem(WORKFLOW_KEY(workspaceId)); } catch { return null; }
}
function saveWorkflowId(workspaceId: string, id: string) {
  try { localStorage.setItem(WORKFLOW_KEY(workspaceId), id); } catch {}
}
function getSavedViewId(workspaceId: string): string | null {
  try { return localStorage.getItem(VIEW_KEY(workspaceId)); } catch { return null; }
}
function saveActiveViewId(workspaceId: string, id: string) {
  try { localStorage.setItem(VIEW_KEY(workspaceId), id); } catch {}
}

export const usePMBoardStore = create<PMBoardState>((set, get) => {
  let latestBoardFetchRequest = 0;

  const runBoardFetch = async (
    args: BoardFetchArgs,
    options: {
      errorMessage: string;
      setLoading?: boolean;
      onSuccess?: () => Partial<PMBoardState>;
      onError?: () => Partial<PMBoardState>;
    },
  ) => {
    const requestId = ++latestBoardFetchRequest;
    if (options.setLoading) {
      set({ loading: true, error: null });
    } else {
      set({ error: null });
    }

    const boardRes = await pmTaskService.listBoard(
      args.workspaceId,
      args.workflowId,
      buildApiFilters(args.teamId, args.filters),
      PER_STATE_LIMIT,
    );

    if (requestId != latestBoardFetchRequest) {
      return null;
    }

    if (boardRes.error || !boardRes.data) {
      set({
        loading: false,
        error: boardRes.error ?? options.errorMessage,
        ...(options.onError?.() ?? {}),
      });
      return null;
    }

    const columns = sortColumns(boardRes.data);
    set({
      columns,
      loading: false,
      error: null,
      ...(options.onSuccess?.() ?? {}),
    });
    return columns;
  };

  const filterFetchScheduler = createDebouncedBoardFetchScheduler<BoardFetchArgs>(
    FILTER_FETCH_DEBOUNCE_MS,
    (args) => {
      void runBoardFetch(args, { errorMessage: 'Failed to filter board' });
    },
  );

  const cancelPendingFilterFetch = () => {
    filterFetchScheduler.cancel();
  };

  return {
  workspaceId: null,
  workflows: [],
  workflow: null,
  columns: [],
  loading: false,
  error: null,
  teamId: null,
  filters: {},
  columnLoading: {},

  // Member board state
  memberColumns: [],
  memberColumnLoading: {},

  // View state
  views: [],
  activeViewId: null,
  savedViewFilters: {},

  loadBoard: async (workspaceId, workflowId) => {
    cancelPendingFilterFetch();
    set({ loading: true, error: null, workspaceId });

    const workflowRes = await pmWorkflowService.list(workspaceId);
    if (workflowRes.error || !workflowRes.data) {
      set({ loading: false, error: workflowRes.error ?? 'Failed to load workflows' });
      return;
    }
    const workflows = workflowRes.data;

    const { teamId, filters } = get();

    let selected: WorkflowWithStates | null = null;

    if (workflowId) {
      // Explicit workflow ID provided — use it
      selected = workflows.find((workflow) => workflow.workflow.id === workflowId) ?? workflows[0] ?? null;
    } else if (teamId) {
      // Team filter active — resolve the team's workflow, auto-seeding if needed.
      const resolved = await pmWorkflowService.resolveTeamWorkflow(workspaceId, teamId);
      if (resolved.error || !resolved.data) {
        set({
          loading: false,
          error: resolved.error ?? 'Failed to resolve team workflow',
          workflows,
          workflow: null,
          columns: [],
        });
        return;
      }
      selected = resolved.data;
    } else {
      // No team filter — use saved workflow or first available
      const resolvedId = getSavedWorkflowId(workspaceId);
      selected = resolvedId
        ? workflows.find((workflow) => workflow.workflow.id === resolvedId) ?? workflows[0] ?? null
        : workflows[0] ?? null;
    }

    if (!selected) {
      set({ loading: false, workflows, workflow: null, columns: [] });
      return;
    }

    saveWorkflowId(workspaceId, selected.workflow.id);
    await runBoardFetch(
      {
        workspaceId,
        workflowId: selected.workflow.id,
        teamId,
        filters,
      },
      {
        errorMessage: 'Failed to load board',
        onSuccess: () => ({ workflows, workflow: selected }),
        onError: () => ({ workflows, workflow: selected }),
      },
    );
  },

  setWorkflow: async (workflowId) => {
    const workspaceId = get().workspaceId;
    if (!workspaceId) return;
    await get().loadBoard(workspaceId, workflowId);
  },

  setTeamFilter: async (teamId) => {
    if (teamId === get().teamId) return;
    cancelPendingFilterFetch();
    set({ teamId });
    const { workspaceId, workflows, filters } = get();
    if (!workspaceId) return;

    // Re-resolve workflow for the new team
    let selected: WorkflowWithStates | null = null;
    if (teamId) {
      const resolved = await pmWorkflowService.resolveTeamWorkflow(workspaceId, teamId);
      if (resolved.error || !resolved.data) {
        set({ error: resolved.error ?? 'Failed to resolve team workflow' });
        return;
      }
      selected = resolved.data;
    } else {
      const resolvedId = getSavedWorkflowId(workspaceId);
      selected = resolvedId
        ? workflows.find((w) => w.workflow.id === resolvedId) ?? workflows[0] ?? null
        : workflows[0] ?? null;
    }

    if (!selected) {
      set({ workflow: null, columns: [] });
      return;
    }

    set({ workflow: selected });
    saveWorkflowId(workspaceId, selected.workflow.id);
    await runBoardFetch(
      {
        workspaceId,
        workflowId: selected.workflow.id,
        teamId,
        filters,
      },
      { errorMessage: 'Failed to filter board' },
    );
  },

  setFilters: async (filters) => {
    set({ filters });
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    filterFetchScheduler.schedule({
      workspaceId,
      workflowId,
      teamId,
      filters,
    });
  },

  refreshBoard: async () => {
    cancelPendingFilterFetch();
    const { workspaceId, workflow, teamId, filters } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    await runBoardFetch(
      {
        workspaceId,
        workflowId,
        teamId,
        filters,
      },
      { errorMessage: 'Failed to refresh board' },
    );
  },

  loadMoreColumn: async (stateId: string) => {
    const { workspaceId, teamId, filters, columns, columnLoading } = get();
    if (!workspaceId || columnLoading[stateId]) return;

    const column = columns.find((c) => c.state.id === stateId);
    if (!column || !column.has_more) return;

    set({ columnLoading: { ...columnLoading, [stateId]: true } });

    const offset = column.tasks.length;
    const res = await pmTaskService.listBoardColumn(
      workspaceId,
      stateId,
      offset,
      COLUMN_PAGE_SIZE,
      buildApiFilters(teamId, filters),
    );

    const doneLoading = { ...get().columnLoading };
    delete doneLoading[stateId];

    if (res.error || !res.data) {
      set({ columnLoading: doneLoading, error: res.error ?? 'Failed to load more tasks' });
      return;
    }

    set((state) => {
      const cols = cloneColumns(state.columns);
      const target = cols.find((c) => c.state.id === stateId);
      if (!target) return { columnLoading: doneLoading };

      // Deduplicate: only add tasks not already loaded
      const existingIds = new Set(target.tasks.map((s) => s.id));
      const newTasks = res.data!.tasks.filter((s) => !existingIds.has(s.id));
      target.tasks = sortTasks([...target.tasks, ...newTasks], target.state.state_type);
      if (isGroupedColumn(target)) {
        target.task_groups = mergeTaskGroups(target.task_groups, res.data!.task_groups);
      }
      target.has_more = target.tasks.length < res.data!.total;

      return { columns: cols, columnLoading: doneLoading };
    });
  },

  patchTask: (action, taskId, task) => {
    const { teamId, filters } = get();
    if (hasAmbiguousPatchFilters(filters)) {
      return false;
    }

    let patched = false;

    set((state) => {
      const columns = cloneColumns(state.columns);

      if (action === 'deleted') {
        for (const col of columns) {
          if (isGroupedColumn(col)) {
            if (col.tasks.some((candidate) => candidate.id === taskId)) {
              patched = false;
              return state;
            }
            continue;
          }
          const removed = removeLoadedTask(col, taskId);
          if (removed) {
            updateColumnTotals(col, -1, -(removed.estimate ?? 0));
            patched = true;
            return { columns };
          }
        }
        return state;
      }

      if (!task) return state;

      const matchesFilters = taskMatchesFilters(task, teamId, filters);

      if (action === 'created') {
        const target = columns.find((c) => c.state.id === task.workflow_state_id);
        if (!target) return state;
        if (isGroupedColumn(target)) {
          patched = false;
          return state;
        }
        if (!matchesFilters) {
          patched = true;
          return state;
        }
        updateColumnTotals(target, 1, task.estimate ?? 0);
        if (!target.has_more || target.tasks.length < PER_STATE_LIMIT) {
          upsertLoadedTask(target, task);
        }
        patched = !target.has_more;
        return { columns };
      }

      // updated / moved — find the task in columns, update in-place or move between columns
      let fromCol: TaskStateColumn | undefined;
      let fromIdx = -1;
      for (const col of columns) {
        const idx = col.tasks.findIndex((s) => s.id === taskId);
        if (idx >= 0) {
          fromCol = col;
          fromIdx = idx;
          break;
        }
      }

      if (!fromCol || fromIdx < 0) {
        if (!matchesFilters) {
          patched = true;
          return state;
        }
        return state;
      }

      const existing = fromCol.tasks[fromIdx];
      const existingEstimate = existing?.estimate ?? 0;

      if (!matchesFilters) {
        if (isGroupedColumn(fromCol)) {
          patched = false;
          return state;
        }
        const removed = removeLoadedTask(fromCol, taskId);
        if (!removed) return state;
        updateColumnTotals(fromCol, -1, -(removed.estimate ?? 0));
        patched = true;
        return { columns };
      }

      if (fromCol.state.id === task.workflow_state_id) {
        if (isGroupedColumn(fromCol)) {
          fromCol.tasks[fromIdx] = mergeEnrichedFields(task, existing);
          replaceGroupedTask(fromCol, fromCol.tasks[fromIdx]);
          updateColumnTotals(fromCol, 0, (task.estimate ?? 0) - existingEstimate);
          patched = true;
          return { columns };
        }
        fromCol.tasks[fromIdx] = mergeEnrichedFields(task, existing);
        fromCol.tasks = sortTasks(fromCol.tasks, fromCol.state.state_type);
        updateColumnTotals(fromCol, 0, (task.estimate ?? 0) - existingEstimate);
        patched = true;
        return { columns };
      }

      if (isGroupedColumn(fromCol)) {
        patched = false;
        return state;
      }

      const removed = removeLoadedTask(fromCol, taskId);
      if (!removed) return state;
      updateColumnTotals(fromCol, -1, -(removed.estimate ?? 0));

      const toCol = columns.find((c) => c.state.id === task.workflow_state_id);
      if (!toCol) return { columns };
      if (isGroupedColumn(toCol)) {
        patched = false;
        return state;
      }

      const merged = mergeEnrichedFields(task, removed);
      updateColumnTotals(toCol, 1, task.estimate ?? 0);
      if (!toCol.has_more || toCol.tasks.length < PER_STATE_LIMIT) {
        upsertLoadedTask(toCol, merged);
        patched = !toCol.has_more;
      } else {
        patched = false;
      }

      return { columns };
    });
    return patched;
  },

  createTask: async (payload) => {
    const { data, error } = await pmTaskService.create(payload);
    if (error || !data) {
      set({ error: error ?? 'Failed to create task' });
      return null;
    }

    const task = data.task;
    set((state) => {
      const columns = cloneColumns(state.columns);
      const targetIndex = columns.findIndex((column) => column.state.id === task.workflow_state_id);
      if (targetIndex === -1) return state;
      if (isGroupedColumn(columns[targetIndex])) return state;
      updateColumnTotals(columns[targetIndex], 1, task.estimate ?? 0);
      if (!columns[targetIndex].has_more || columns[targetIndex].tasks.length < PER_STATE_LIMIT) {
        upsertLoadedTask(columns[targetIndex], task);
      }
      return { columns };
    });
    const targetColumn = get().columns.find((column) => column.state.id === task.workflow_state_id);
    if (targetColumn && isGroupedColumn(targetColumn)) {
      await get().refreshBoard();
    }
    return task;
  },

  moveTask: async ({ workspaceId, taskId, fromStateId, toStateId, toIndex, debugTraceID }) => {
    const snapshot = cloneColumns(get().columns);
    const moveCtx = { workflowId: get().workflow?.workflow.id, teamId: get().teamId };
    const targetStateType = snapshot.find((column) => column.state.id === toStateId)?.state.state_type;
    const traceID = debugTraceID ?? createPMDnDTraceID();
    const contextChanged = () => {
      const s = get();
      return s.workflow?.workflow.id !== moveCtx.workflowId || s.teamId !== moveCtx.teamId;
    };

    logPMDnD('store.move.begin', {
      trace_id: traceID,
      workspace_id: workspaceId,
      task_id: taskId,
      from_state_id: fromStateId,
      to_state_id: toStateId,
      to_index: toIndex,
      target_state_type: targetStateType,
      from_column: summarizePMDnDColumn(snapshot.find((column) => column.state.id === fromStateId)),
      to_column: summarizePMDnDColumn(snapshot.find((column) => column.state.id === toStateId)),
    });

    // Always optimistically move the card immediately
    let optimisticColumns: TaskStateColumn[] | null = null;
    set((state) => {
      const columns = cloneColumns(state.columns);
      const fromCol = columns.find((column) => column.state.id === fromStateId);
      const toCol = columns.find((column) => column.state.id === toStateId);
      if (!fromCol || !toCol) return state;

      const sourceIndex = fromCol.tasks.findIndex((candidate) => candidate.id === taskId);
      if (sourceIndex === -1) return state;
      if (fromStateId === toStateId && fromCol.state.state_type === 'done') {
        optimisticColumns = columns;
        return state;
      }

      const [moving] = fromCol.tasks.splice(sourceIndex, 1);
      if (!moving) return state;

      moving.workflow_state_id = toStateId;
      if (fromStateId === toStateId) {
        fromCol.tasks.splice(toIndex, 0, moving);
        reindexLoadedTasks(fromCol);
        optimisticColumns = columns;
        return { columns };
      }

      updateColumnTotals(fromCol, -1, -(moving.estimate ?? 0));
      updateColumnTotals(toCol, 1, moving.estimate ?? 0);
      reindexLoadedTasks(fromCol);
      // For done columns, set completed_at so optimistic sort matches server behavior
      if (toCol.state.state_type === 'done') {
        moving.completed_at = new Date().toISOString();
        moving.moved_at = moving.completed_at;
        moving.completed = true;
        moving.position = toCol.tasks.length;
        toCol.tasks = sortTasks([...toCol.tasks, moving], toCol.state.state_type);
      } else {
        // Clear done metadata when moving out of done
        if (moving.completed_at) {
          moving.completed_at = undefined as unknown as string;
          moving.completed = false;
        }
        toCol.tasks.splice(toIndex, 0, moving);
        reindexLoadedTasks(toCol);
      }
      optimisticColumns = columns;
      return { columns };
    });

    if (fromStateId === toStateId && targetStateType === 'done') {
      logPMDnD('store.move.skip_done_reorder', {
        trace_id: traceID,
        task_id: taskId,
        state_id: fromStateId,
      });
    } else if (optimisticColumns) {
      const loggedColumns = optimisticColumns as TaskStateColumn[];
      logPMDnD('store.move.optimistic_applied', {
        trace_id: traceID,
        task_id: taskId,
        from_column: summarizePMDnDColumn(loggedColumns.find((column) => column.state.id === fromStateId)),
        to_column: summarizePMDnDColumn(loggedColumns.find((column) => column.state.id === toStateId)),
      });
    }

    if (fromStateId === toStateId) {
      if (targetStateType === 'done') {
        return;
      }
      const reorderPayload = { position: toIndex, debug_trace_id: traceID };
      logPMDnD('store.move.reorder_request', {
        trace_id: traceID,
        task_id: taskId,
        payload: reorderPayload,
      });
      const reorderRes = await pmTaskService.reorder(workspaceId, taskId, reorderPayload);
      if (reorderRes.error && !contextChanged()) {
        logPMDnD('store.move.reorder_error', {
          trace_id: traceID,
          task_id: taskId,
          error: reorderRes.error,
        });
        set({ columns: snapshot, error: reorderRes.error ?? 'Failed to reorder task' });
      } else {
        logPMDnD('store.move.reorder_success', {
          trace_id: traceID,
          task_id: taskId,
        });
      }
      return;
    }

    const movePayload = targetStateType === 'done'
      ? { state_id: toStateId, debug_trace_id: traceID }
      : { state_id: toStateId, position: toIndex, debug_trace_id: traceID };
    logPMDnD('store.move.move_request', {
      trace_id: traceID,
      task_id: taskId,
      payload: movePayload,
    });
    const moveRes = await pmTaskService.move(
      workspaceId,
      taskId,
      movePayload,
    );
    if (moveRes.error) {
      if (!contextChanged()) {
        logPMDnD('store.move.move_error', {
          trace_id: traceID,
          task_id: taskId,
          error: moveRes.error,
        });
        set({ columns: snapshot, error: moveRes.error ?? 'Failed to move task' });
      }
      return;
    }

    // Skip patching if user switched board context mid-flight
    if (contextChanged()) return;

    const updatedTask = moveRes.data?.task;
    if (updatedTask) {
      logPMDnD('store.move.move_success', {
        trace_id: traceID,
        task_id: taskId,
        response_task: {
          id: updatedTask.id,
          workflow_state_id: updatedTask.workflow_state_id,
          position: updatedTask.position,
          completed: updatedTask.completed,
          completed_at: updatedTask.completed_at,
          moved_at: updatedTask.moved_at,
          updated_at: updatedTask.updated_at,
        },
      });
      set((state) => {
        const columns = cloneColumns(state.columns);
        const target = columns.find((column) => column.state.id === toStateId);
        if (!target) return state;
        const idx = target.tasks.findIndex((candidate) => candidate.id === taskId);
        if (idx >= 0) {
          target.tasks[idx] = mergeEnrichedFields(updatedTask, target.tasks[idx]);
          target.tasks = sortTasks(target.tasks, target.state.state_type);
        }
        return { columns };
      });
    }

    // If source or target columns were truncated, refresh them to resync positions
    const fromTruncated = snapshot.find((c) => c.state.id === fromStateId)?.has_more;
    const toTruncated = snapshot.find((c) => c.state.id === toStateId)?.has_more;
    if (fromTruncated || toTruncated || targetStateType === 'done') {
      logPMDnD('store.move.refresh_requested', {
        trace_id: traceID,
        task_id: taskId,
        from_truncated: fromTruncated,
        to_truncated: toTruncated,
        target_state_type: targetStateType,
      });
      get().refreshBoard();
    }
  },

  // ── Member board actions ─────────────────────────────────────────

  loadMemberBoard: async (memberIds, includeEmpty) => {
    const { workspaceId, workflow, teamId, filters } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const res = await pmTaskService.listBoardByMember(
      workspaceId,
      workflowId,
      buildApiFilters(teamId, filters),
      PER_STATE_LIMIT,
      includeEmpty,
      memberIds,
    );
    if (res.error || !res.data) {
      set({ error: res.error ?? 'Failed to load member board' });
      return;
    }
    set({ memberColumns: res.data });
  },

  loadMoreMemberColumn: async (memberId: string | null) => {
    const colKey = memberId ?? '__unassigned__';
    const { workspaceId, workflow, teamId, filters, memberColumns, memberColumnLoading } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId || memberColumnLoading[colKey]) return;

    const column = memberColumns.find((c) =>
      memberId ? c.member?.id === memberId : c.member === null,
    );
    if (!column || !column.has_more) return;

    set({ memberColumnLoading: { ...memberColumnLoading, [colKey]: true } });

    const offset = column.tasks.length;
    const res = await pmTaskService.listBoardMemberColumn(
      workspaceId,
      workflowId,
      memberId,
      offset,
      COLUMN_PAGE_SIZE,
      buildApiFilters(teamId, filters),
    );

    const doneLoading = { ...get().memberColumnLoading };
    delete doneLoading[colKey];

    if (res.error || !res.data) {
      set({ memberColumnLoading: doneLoading, error: res.error ?? 'Failed to load more tasks' });
      return;
    }

    set((state) => {
      const cols = state.memberColumns.map((c) => ({ ...c, tasks: [...c.tasks] }));
      const target = cols.find((c) =>
        memberId ? c.member?.id === memberId : c.member === null,
      );
      if (!target) return { memberColumnLoading: doneLoading };

      const existingIds = new Set(target.tasks.map((s) => s.id));
      const newTasks = res.data!.tasks.filter((s) => !existingIds.has(s.id));
      target.tasks = [...target.tasks, ...newTasks];
      target.has_more = target.tasks.length < res.data!.total;

      return { memberColumns: cols, memberColumnLoading: doneLoading };
    });
  },

  moveMemberTask: async ({ workspaceId, taskId, fromMemberId, toMemberId, toIndex }) => {
    if (fromMemberId === toMemberId) {
      return;
    }

    const snapshot = get().memberColumns.map((c) => ({ ...c, tasks: [...c.tasks] }));
    const moveCtx = { workflowId: get().workflow?.workflow.id, teamId: get().teamId };
    const contextChanged = () => {
      const s = get();
      return s.workflow?.workflow.id !== moveCtx.workflowId || s.teamId !== moveCtx.teamId;
    };

    // Optimistic update
    set((state) => {
      const cols = state.memberColumns.map((c) => ({ ...c, tasks: [...c.tasks] }));
      const fromCol = cols.find((c) =>
        fromMemberId ? c.member?.id === fromMemberId : c.member === null,
      );
      const toCol = cols.find((c) =>
        toMemberId ? c.member?.id === toMemberId : c.member === null,
      );
      if (!fromCol || !toCol) return state;

      const sourceIndex = fromCol.tasks.findIndex((s) => s.id === taskId);
      if (sourceIndex === -1) return state;

      const [moving] = fromCol.tasks.splice(sourceIndex, 1);
      if (!moving) return state;

      if (fromMemberId === toMemberId) {
        fromCol.tasks.splice(toIndex, 0, moving);
        return { memberColumns: cols };
      }

      moving.owner_member_id = toMemberId ?? undefined;
      fromCol.task_count = Math.max(0, fromCol.task_count - 1);
      fromCol.point_total = Math.max(0, fromCol.point_total - (moving.estimate ?? 0));
      fromCol.has_more = fromCol.tasks.length < fromCol.task_count;

      toCol.tasks.splice(toIndex, 0, moving);
      toCol.task_count += 1;
      toCol.point_total += moving.estimate ?? 0;
      toCol.has_more = toCol.tasks.length < toCol.task_count;

      return { memberColumns: cols };
    });

    // Reassign owner only. Member-board ordering is derived from workflow state and task position,
    // not a separate per-member manual ranking.
    const updateRes = await pmTaskService.update(workspaceId, taskId, {
      owner_member_id: toMemberId ?? '',
    });
    if (updateRes.error && !contextChanged()) {
      set({ memberColumns: snapshot, error: updateRes.error ?? 'Failed to reassign task' });
      return;
    }
  },

  // ── View actions ──────────────────────────────────────────────────

  loadViews: async (workspaceId, currentMemberId) => {
    const defaults = getDefaultViews(currentMemberId);
    const res = await pmViewService.list(workspaceId);
    const custom = res.data ?? [];
    const allViews = [...defaults, ...custom];
    set({ views: allViews });

    // Restore saved view, or fall back to Everything
    if (!get().activeViewId) {
      const savedId = getSavedViewId(workspaceId);
      const savedView = savedId ? allViews.find((v) => v.id === savedId) : null;
      get().applyView(savedView ?? defaults[0]);
    }
  },

  applyView: (view) => {
    cancelPendingFilterFetch();
    const cleanFilters: Record<string, string> = {};
    for (const [k, v] of Object.entries(view.filters)) {
      if (v) cleanFilters[k] = v;
    }
    const wsId = get().workspaceId;
    if (wsId) saveActiveViewId(wsId, view.id);
    set({
      activeViewId: view.id,
      filters: { ...cleanFilters },
      savedViewFilters: { ...cleanFilters },
    });
    const filters = cleanFilters;
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    void runBoardFetch(
      {
        workspaceId,
        workflowId,
        teamId,
        filters,
      },
      { errorMessage: 'Failed to apply view' },
    );
  },

  saveCurrentAsView: async (workspaceId, name, isShared) => {
    const { filters } = get();
    const cleanFilters: Record<string, string> = {};
    for (const [k, v] of Object.entries(filters)) {
      if (v) cleanFilters[k] = v;
    }
    const res = await pmViewService.create(workspaceId, {
      name,
      filters: cleanFilters,
      is_shared: isShared,
      is_pinned: false,
    });
    if (res.error || !res.data) return null;
    const newView = res.data;
    set((state) => ({ views: [...state.views, newView] }));
    get().applyView(newView);
    return newView;
  },

  saveChangesToView: async (workspaceId) => {
    const { activeViewId, filters, views } = get();
    if (!activeViewId || isDefaultView(activeViewId)) return;
    const cleanFilters: Record<string, string> = {};
    for (const [k, v] of Object.entries(filters)) {
      if (v) cleanFilters[k] = v;
    }
    const res = await pmViewService.update(workspaceId, activeViewId, { filters: cleanFilters });
    if (res.error || !res.data) return;
    const updated = res.data;
    set({
      views: views.map((v) => (v.id === updated.id ? updated : v)),
      savedViewFilters: { ...cleanFilters },
    });
  },

  discardChanges: () => {
    cancelPendingFilterFetch();
    const { savedViewFilters } = get();
    set({ filters: { ...savedViewFilters } });
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    void runBoardFetch(
      {
        workspaceId,
        workflowId,
        teamId,
        filters: savedViewFilters,
      },
      { errorMessage: 'Failed to discard changes' },
    );
  },

  deleteView: async (workspaceId, id) => {
    if (isDefaultView(id)) return;
    const res = await pmViewService.remove(workspaceId, id);
    if (res.error) return;
    const { activeViewId, views } = get();
    const nextViews = views.filter((v) => v.id !== id);
    set({ views: nextViews });
    if (activeViewId === id) {
      const everything = nextViews.find((v) => v.id === '__default_everything__');
      if (everything) get().applyView(everything);
    }
  },

  updateView: async (workspaceId, id, payload) => {
    if (isDefaultView(id)) return null;
    const res = await pmViewService.update(workspaceId, id, payload);
    if (res.error || !res.data) return null;
    const updated = res.data;
    set((state) => ({
      views: state.views.map((v) => (v.id === updated.id ? updated : v)),
    }));
    return updated;
  },

  };
});
