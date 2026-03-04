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

const buildApiFilters = (teamId: string | null, filters: BoardFilters): Record<string, string | undefined> => {
  const result: Record<string, string | undefined> = { ...filters };
  if (teamId) result.team_id = teamId;
  return result;
};

export const usePMBoardStore = create<PMBoardState>((set, get) => ({
  workspaceId: null,
  workflows: [],
  workflow: null,
  columns: [],
  loading: false,
  error: null,
  teamId: null,
  filters: {},

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

    const selected = workflowId
      ? workflowRes.data.find((workflow) => workflow.workflow.id === workflowId) ?? workflowRes.data[0] ?? null
      : workflowRes.data[0] ?? null;

    if (!selected) {
      set({ loading: false, workflows: workflowRes.data, workflow: null, columns: [] });
      return;
    }

    const { teamId, filters } = get();
    const boardRes = await pmStoryService.listBoard(workspaceId, selected.workflow.id, buildApiFilters(teamId, filters));
    if (boardRes.error || !boardRes.data) {
      set({ loading: false, error: boardRes.error ?? 'Failed to load board', workflows: workflowRes.data, workflow: selected });
      return;
    }

    const sorted = [...boardRes.data].sort((a, b) => a.state.position - b.state.position);
    set({ workflows: workflowRes.data, workflow: selected, columns: sorted, loading: false });
  },

  setWorkflow: async (workflowId) => {
    const workspaceId = get().workspaceId;
    if (!workspaceId) return;
    await get().loadBoard(workspaceId, workflowId);
  },

  setTeamFilter: async (teamId) => {
    set({ teamId });
    const { workspaceId, workflow, filters } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters));
    if (boardRes.error || !boardRes.data) {
      set({ error: boardRes.error ?? 'Failed to filter board' });
      return;
    }
    const sorted = [...boardRes.data].sort((a, b) => a.state.position - b.state.position);
    set({ columns: sorted });
  },

  setFilters: async (filters) => {
    set({ filters });
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters));
    if (boardRes.error || !boardRes.data) {
      set({ error: boardRes.error ?? 'Failed to filter board' });
      return;
    }
    const sorted = [...boardRes.data].sort((a, b) => a.state.position - b.state.position);
    set({ columns: sorted });
  },

  refreshBoard: async () => {
    const { workspaceId, workflow, teamId, filters } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters));
    if (boardRes.error || !boardRes.data) {
      set({ error: boardRes.error ?? 'Failed to refresh board' });
      return;
    }
    const sorted = [...boardRes.data].sort((a, b) => a.state.position - b.state.position);
    set({ columns: sorted });
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
    set({ views: [...defaults, ...custom] });

    // Apply Everything view if no view is active
    if (!get().activeViewId) {
      get().applyView(defaults[0]);
    }
  },

  applyView: (view) => {
    const filters: BoardFilters = { ...view.filters };
    set({
      activeViewId: view.id,
      filters,
      savedViewFilters: { ...filters },
    });
    // Trigger board refresh with new filters
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, filters)).then((boardRes) => {
      if (boardRes.error || !boardRes.data) {
        set({ error: boardRes.error ?? 'Failed to apply view' });
        return;
      }
      const sorted = [...boardRes.data].sort((a, b) => a.state.position - b.state.position);
      set({ columns: sorted });
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
      savedViewFilters: { ...filters },
    });
  },

  discardChanges: () => {
    const { savedViewFilters } = get();
    set({ filters: { ...savedViewFilters } });
    // Re-fetch board with saved filters
    const { workspaceId, workflow, teamId } = get();
    const workflowId = workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    pmStoryService.listBoard(workspaceId, workflowId, buildApiFilters(teamId, savedViewFilters)).then((boardRes) => {
      if (boardRes.error || !boardRes.data) return;
      const sorted = [...boardRes.data].sort((a, b) => a.state.position - b.state.position);
      set({ columns: sorted });
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
