import { create } from 'zustand';
import type {
  CreateStoryRequest,
  PMView,
  Story,
  StoryStateColumn,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmViewService } from '@/lib/services/pmViewService';
import { getDefaultViews, isDefaultView } from '@/lib/pmDefaultViews';

export type BoardFilters = Record<string, string | undefined>;

/** Default number of stories loaded per column on initial board fetch. */
const PER_STATE_LIMIT = 50;

/** Number of additional stories fetched per "load more" click. */
const COLUMN_PAGE_SIZE = 50;

interface MovePayload {
  workspaceId: string;
  storyId: string;
  fromStateId: string;
  toStateId: string;
  toIndex: number;
}

interface PMBoardState {
  workspaceId: string | null;
  workflows: WorkflowWithStates[];
  workflow: WorkflowWithStates | null;
  columns: StoryStateColumn[];
  loading: boolean;
  error: string | null;
  teamId: string | null;
  filters: BoardFilters;

  /** Map of state IDs currently loading more stories. */
  columnLoading: Record<string, boolean>;

  // View state
  views: PMView[];
  activeViewId: string | null;
  savedViewFilters: BoardFilters;

  loadBoard: (workspaceId: string, workflowId?: string) => Promise<void>;
  setWorkflow: (workflowId: string) => Promise<void>;
  setTeamFilter: (teamId: string | null) => Promise<void>;
  setFilters: (filters: BoardFilters) => Promise<void>;
  refreshBoard: () => Promise<void>;
  createStory: (payload: CreateStoryRequest) => Promise<Story | null>;
  moveStory: (payload: MovePayload) => Promise<void>;
  loadMoreColumn: (stateId: string) => Promise<void>;

  /** Incremental patch: add, update, remove, or move a single story in the board state. */
  patchStory: (action: 'created' | 'updated' | 'deleted' | 'moved', storyId: string, story?: Story) => void;

  // View actions
  loadViews: (workspaceId: string, currentUserId: string) => Promise<void>;
  applyView: (view: PMView) => void;
  saveCurrentAsView: (workspaceId: string, name: string, isShared: boolean) => Promise<PMView | null>;
  saveChangesToView: (workspaceId: string) => Promise<void>;
  discardChanges: () => void;
  deleteView: (workspaceId: string, id: string) => Promise<void>;
  updateView: (workspaceId: string, id: string, payload: Parameters<typeof pmViewService.update>[2]) => Promise<PMView | null>;
}

const cloneColumns = (columns: StoryStateColumn[]) =>
  columns.map((column) => ({
    ...column,
    stories: [...column.stories],
  }));

const recalcColumn = (column: StoryStateColumn): StoryStateColumn => ({
  ...column,
  stories: column.stories.map((story, index) => ({
    ...story,
    position: index,
  })),
  story_count: column.stories.length,
  point_total: column.stories.reduce((sum, story) => sum + (story.estimate ?? 0), 0),
});

const sumPoints = (stories: Story[]) =>
  stories.reduce((sum, s) => sum + (s.estimate ?? 0), 0);

const buildApiFilters = (teamId: string | null, filters: BoardFilters): Record<string, string | undefined> => {
  const result: Record<string, string | undefined> = { ...filters };
  if (teamId) result.team_id = teamId;
  return result;
};

const sortColumns = (data: StoryStateColumn[]) =>
  [...data].sort((a, b) => a.state.position - b.state.position);

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

export const usePMBoardStore = create<PMBoardState>((set, get) => ({
  workspaceId: null,
  workflows: [],
  workflow: null,
  columns: [],
  loading: false,
  error: null,
  teamId: null,
  filters: {},
  columnLoading: {},

  // View state
  views: [],
  activeViewId: null,
  savedViewFilters: {},

  loadBoard: async (workspaceId, workflowId) => {
    set({ loading: true, error: null, workspaceId });

    const workflowRes = await pmWorkflowService.list(workspaceId);
    if (workflowRes.error || !workflowRes.data) {
      set({ loading: false, error: workflowRes.error ?? 'Failed to load workflows' });
      return;
    }

    const resolvedId = workflowId ?? getSavedWorkflowId(workspaceId);
    const selected = resolvedId
      ? workflowRes.data.find((workflow) => workflow.workflow.id === resolvedId) ?? workflowRes.data[0] ?? null
      : workflowRes.data[0] ?? null;

    if (!selected) {
      set({ loading: false, workflows: workflowRes.data, workflow: null, columns: [] });
      return;
    }

    const { teamId, filters } = get();
    const boardRes = await pmStoryService.listBoard(workspaceId, selected.workflow.id, buildApiFilters(teamId, filters), PER_STATE_LIMIT);
    if (boardRes.error || !boardRes.data) {
      set({ loading: false, error: boardRes.error ?? 'Failed to load board', workflows: workflowRes.data, workflow: selected });
      return;
    }

    saveWorkflowId(workspaceId, selected.workflow.id);
    set({ workflows: workflowRes.data, workflow: selected, columns: sortColumns(boardRes.data), loading: false });
  },

  setWorkflow: async (workflowId) => {
    const workspaceId = get().workspaceId;
    if (!workspaceId) return;
    await get().loadBoard(workspaceId, workflowId);
  },

  setTeamFilter: async (teamId) => {
    if (teamId === get().teamId) return;
    set({ teamId });
    const { workspaceId, workflow, filters } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters), PER_STATE_LIMIT);
    if (boardRes.error || !boardRes.data) {
      set({ error: boardRes.error ?? 'Failed to filter board' });
      return;
    }
    set({ columns: sortColumns(boardRes.data) });
  },

  setFilters: async (filters) => {
    set({ filters });
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters), PER_STATE_LIMIT);
    if (boardRes.error || !boardRes.data) {
      set({ error: boardRes.error ?? 'Failed to filter board' });
      return;
    }
    set({ columns: sortColumns(boardRes.data) });
  },

  refreshBoard: async () => {
    const { workspaceId, workflow, teamId, filters } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters), PER_STATE_LIMIT);
    if (boardRes.error || !boardRes.data) {
      set({ error: boardRes.error ?? 'Failed to refresh board' });
      return;
    }
    set({ columns: sortColumns(boardRes.data) });
  },

  loadMoreColumn: async (stateId: string) => {
    const { workspaceId, teamId, filters, columns, columnLoading } = get();
    if (!workspaceId || columnLoading[stateId]) return;

    const column = columns.find((c) => c.state.id === stateId);
    if (!column || !column.has_more) return;

    set({ columnLoading: { ...columnLoading, [stateId]: true } });

    const offset = column.stories.length;
    const res = await pmStoryService.listBoardColumn(
      workspaceId,
      stateId,
      offset,
      COLUMN_PAGE_SIZE,
      buildApiFilters(teamId, filters),
    );

    const { [stateId]: _, ...doneLoading } = get().columnLoading;

    if (res.error || !res.data) {
      set({ columnLoading: doneLoading, error: res.error ?? 'Failed to load more stories' });
      return;
    }

    set((state) => {
      const cols = cloneColumns(state.columns);
      const target = cols.find((c) => c.state.id === stateId);
      if (!target) return { columnLoading: doneLoading };

      // Deduplicate: only add stories not already loaded
      const existingIds = new Set(target.stories.map((s) => s.id));
      const newStories = res.data!.stories.filter((s) => !existingIds.has(s.id));
      target.stories.push(...newStories);
      target.has_more = target.stories.length < res.data!.total;

      return { columns: cols, columnLoading: doneLoading };
    });
  },

  patchStory: (action, storyId, story) => {
    set((state) => {
      const columns = cloneColumns(state.columns);

      if (action === 'deleted') {
        for (const col of columns) {
          const idx = col.stories.findIndex((s) => s.id === storyId);
          if (idx >= 0) {
            col.stories.splice(idx, 1);
            col.story_count = Math.max(0, col.story_count - 1);
            col.point_total = sumPoints(col.stories);
            return { columns };
          }
        }
        return state;
      }

      if (!story) return state;

      if (action === 'created') {
        const target = columns.find((c) => c.state.id === story.workflow_state_id);
        if (!target) return state;
        if (!target.stories.some((s) => s.id === storyId)) {
          target.stories.push(story);
          target.story_count += 1;
          target.point_total = sumPoints(target.stories);
        }
        return { columns };
      }

      // updated / moved — find the story in columns, update in-place or move between columns
      let fromCol: StoryStateColumn | undefined;
      let fromIdx = -1;
      for (const col of columns) {
        const idx = col.stories.findIndex((s) => s.id === storyId);
        if (idx >= 0) {
          fromCol = col;
          fromIdx = idx;
          break;
        }
      }

      if (fromCol && fromIdx >= 0) {
        if (fromCol.state.id === story.workflow_state_id) {
          fromCol.stories[fromIdx] = story;
          fromCol.point_total = sumPoints(fromCol.stories);
        } else {
          fromCol.stories.splice(fromIdx, 1);
          fromCol.story_count = Math.max(0, fromCol.story_count - 1);
          fromCol.point_total = sumPoints(fromCol.stories);
          const toCol = columns.find((c) => c.state.id === story.workflow_state_id);
          if (toCol) {
            toCol.stories.push(story);
            toCol.story_count += 1;
            toCol.point_total = sumPoints(toCol.stories);
          }
        }
      }

      return { columns };
    });
  },

  createStory: async (payload) => {
    const { data, error } = await pmStoryService.create(payload);
    if (error || !data) {
      set({ error: error ?? 'Failed to create story' });
      return null;
    }

    const story = data.story;
    set((state) => {
      const columns = cloneColumns(state.columns);
      const targetIndex = columns.findIndex((column) => column.state.id === story.workflow_state_id);
      if (targetIndex === -1) return state;
      columns[targetIndex].stories.push(story);
      columns[targetIndex] = recalcColumn(columns[targetIndex]);
      return { columns };
    });
    return story;
  },

  moveStory: async ({ workspaceId, storyId, fromStateId, toStateId, toIndex }) => {
    const snapshot = cloneColumns(get().columns);

    set((state) => {
      const columns = cloneColumns(state.columns);
      const fromCol = columns.find((column) => column.state.id === fromStateId);
      const toCol = columns.find((column) => column.state.id === toStateId);
      if (!fromCol || !toCol) return state;

      const sourceIndex = fromCol.stories.findIndex((story) => story.id === storyId);
      if (sourceIndex === -1) return state;

      const [moving] = fromCol.stories.splice(sourceIndex, 1);
      if (!moving) return state;

      moving.workflow_state_id = toStateId;
      if (fromStateId === toStateId) {
        fromCol.stories.splice(toIndex, 0, moving);
        const recalculated = recalcColumn(fromCol);
        return {
          columns: columns.map((column) =>
            column.state.id === recalculated.state.id ? recalculated : column
          ),
        };
      }

      toCol.stories.splice(toIndex, 0, moving);
      const nextColumns = columns.map((column) => {
        if (column.state.id === fromCol.state.id) return recalcColumn(fromCol);
        if (column.state.id === toCol.state.id) return recalcColumn(toCol);
        return column;
      });
      return { columns: nextColumns };
    });

    if (fromStateId === toStateId) {
      const targetColumn = get().columns.find((column) => column.state.id === toStateId);
      const nextStory = targetColumn?.stories[toIndex];
      const position = nextStory?.position ?? toIndex;
      const reorderRes = await pmStoryService.reorder(workspaceId, storyId, { position });
      if (reorderRes.error) {
        set({ columns: snapshot, error: reorderRes.error ?? 'Failed to reorder story' });
      }
      return;
    }

    const moveRes = await pmStoryService.move(workspaceId, storyId, { state_id: toStateId, position: toIndex });
    if (moveRes.error) {
      set({ columns: snapshot, error: moveRes.error ?? 'Failed to move story' });
      return;
    }

    const updatedStory = moveRes.data?.story;
    if (updatedStory) {
      set((state) => {
        const columns = cloneColumns(state.columns);
        const target = columns.find((column) => column.state.id === toStateId);
        if (!target) return state;
        const idx = target.stories.findIndex((story) => story.id === storyId);
        if (idx >= 0) {
          target.stories[idx] = updatedStory;
        }
        return { columns };
      });
    }
  },

  // ── View actions ──────────────────────────────────────────────────

  loadViews: async (workspaceId, currentUserId) => {
    const defaults = getDefaultViews(currentUserId);
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
    // Trigger board refresh with new filters
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters), PER_STATE_LIMIT).then((boardRes) => {
      if (boardRes.error || !boardRes.data) {
        set({ error: boardRes.error ?? 'Failed to apply view' });
        return;
      }
      set({ columns: sortColumns(boardRes.data) });
    });
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
    const { savedViewFilters } = get();
    set({ filters: { ...savedViewFilters } });
    // Re-fetch board with saved filters
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, savedViewFilters), PER_STATE_LIMIT).then((boardRes) => {
      if (boardRes.error || !boardRes.data) return;
      set({ columns: sortColumns(boardRes.data) });
    });
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
}));
