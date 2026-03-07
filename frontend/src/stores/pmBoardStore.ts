import { create } from 'zustand';
import type {
  CreateStoryRequest,
  PMView,
  Story,
  StoryGroup,
  StoryStateColumn,
  StateType,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmStoryService } from '@/lib/services/pmStoryService';
import { pmViewService } from '@/lib/services/pmViewService';
import { getDefaultViews, isDefaultView } from '@/lib/pmDefaultViews';

export type BoardFilters = Record<string, string | undefined>;

/** Default number of stories loaded per column on initial board fetch. */
const PER_STATE_LIMIT = 50;

/** Number of additional stories fetched when a column nears the bottom. */
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

  /** Incremental patch: add, update, remove, or move a single story in the board state. Returns true when reconciled locally. */
  patchStory: (action: 'created' | 'updated' | 'deleted' | 'moved', storyId: string, story?: Story) => boolean;

  // View actions
  loadViews: (workspaceId: string, currentMemberId: string) => Promise<void>;
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
    story_groups: column.story_groups?.map((group) => ({
      ...group,
      stories: [...group.stories],
    })) ?? [],
  }));

const groupedStateType: StateType = 'done';

const compareStories = (a: Story, b: Story, stateType?: StateType) => {
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

const sortStories = (stories: Story[], stateType?: StateType) => [...stories].sort((a, b) => compareStories(a, b, stateType));

const isGroupedColumn = (column: Pick<StoryStateColumn, 'state'>) => column.state.state_type === groupedStateType;

const storyMatchesFilters = (story: Story, teamId: string | null, filters: BoardFilters) => {
  const matchesCsv = (actual: string | undefined, value: string | undefined) => {
    if (!value) return true;
    return value.split(',').includes(actual ?? '');
  };

  if (teamId && story.team_id !== teamId) return false;
  if (!matchesCsv(story.priority, filters.priority)) return false;
  if (!matchesCsv(story.severity, filters.severity)) return false;
  if (!matchesCsv(story.story_type, filters.story_type)) return false;
  if (!matchesCsv(story.epic_id, filters.epic_id)) return false;
  if (!matchesCsv(story.sprint_id, filters.sprint_id)) return false;
  if (!matchesCsv(story.owner_member_id, filters.owner_member_id)) return false;
  if (!matchesCsv(story.requester_member_id, filters.requester_member_id)) return false;
  if (filters.blocked && String(story.blocked) !== filters.blocked) return false;
  if (filters.updated_after && story.updated_at < filters.updated_after) return false;
  return true;
};

const hasAmbiguousPatchFilters = (filters: BoardFilters) => Boolean(filters.label_id);

const updateColumnTotals = (column: StoryStateColumn, countDelta: number, pointDelta: number) => {
  column.story_count = Math.max(0, column.story_count + countDelta);
  column.point_total = Math.max(0, column.point_total + pointDelta);
  column.has_more = column.stories.length < column.story_count;
};

const removeLoadedStory = (column: StoryStateColumn, storyId: string) => {
  const index = column.stories.findIndex((candidate) => candidate.id === storyId);
  if (index === -1) return null;
  const [removed] = column.stories.splice(index, 1);
  return removed ?? null;
};

const replaceGroupedStory = (column: StoryStateColumn, story: Story) => {
  if (!column.story_groups?.length) return false;
  let replaced = false;
  column.story_groups = column.story_groups.map((group) => {
    const storyIndex = group.stories.findIndex((candidate) => candidate.id === story.id);
    if (storyIndex === -1) return group;
    const stories = [...group.stories];
    stories[storyIndex] = story;
    replaced = true;
    return { ...group, stories };
  });
  return replaced;
};

const mergeStoryGroups = (existing: StoryGroup[] | undefined, incoming: StoryGroup[] | undefined) => {
  if (!incoming?.length) return existing ?? [];
  if (!existing?.length) {
    return incoming.map((group) => ({ ...group, stories: [...group.stories] }));
  }

  const merged = existing.map((group) => ({
    ...group,
    stories: [...group.stories],
  }));
  const groupIndexByKey = new Map(merged.map((group, index) => [group.key, index]));

  for (const nextGroup of incoming) {
    const existingIndex = groupIndexByKey.get(nextGroup.key);
    if (existingIndex === undefined) {
      groupIndexByKey.set(nextGroup.key, merged.length);
      merged.push({ ...nextGroup, stories: [...nextGroup.stories] });
      continue;
    }

    const existingGroup = merged[existingIndex];
    const existingIds = new Set(existingGroup.stories.map((story) => story.id));
    existingGroup.stories = [
      ...existingGroup.stories,
      ...nextGroup.stories.filter((story) => !existingIds.has(story.id)),
    ];
  }

  return merged;
};

/** Preserve board-enriched display fields (owner_name, epic_name) from existing story when IDs match. */
const mergeEnrichedFields = (incoming: Story, existing: Story): Story => ({
  ...incoming,
  owner_name: incoming.owner_name ?? (
    incoming.owner_member_id === existing.owner_member_id
      ? existing.owner_name
      : undefined
  ),
  epic_name: incoming.epic_name ?? (incoming.epic_id === existing.epic_id ? existing.epic_name : undefined),
});

const upsertLoadedStory = (column: StoryStateColumn, story: Story) => {
  const nextStories = sortStories([
    ...column.stories.filter((candidate) => candidate.id !== story.id),
    story,
  ], column.state.state_type);
  column.stories = nextStories;
};

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

    const doneLoading = { ...get().columnLoading };
    delete doneLoading[stateId];

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
      target.stories = sortStories([...target.stories, ...newStories], target.state.state_type);
      if (isGroupedColumn(target)) {
        target.story_groups = mergeStoryGroups(target.story_groups, res.data!.story_groups);
      }
      target.has_more = target.stories.length < res.data!.total;

      return { columns: cols, columnLoading: doneLoading };
    });
  },

  patchStory: (action, storyId, story) => {
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
            if (col.stories.some((candidate) => candidate.id === storyId)) {
              patched = false;
              return state;
            }
            continue;
          }
          const removed = removeLoadedStory(col, storyId);
          if (removed) {
            updateColumnTotals(col, -1, -(removed.estimate ?? 0));
            patched = true;
            return { columns };
          }
        }
        return state;
      }

      if (!story) return state;

      const matchesFilters = storyMatchesFilters(story, teamId, filters);

      if (action === 'created') {
        const target = columns.find((c) => c.state.id === story.workflow_state_id);
        if (!target) return state;
        if (isGroupedColumn(target)) {
          patched = false;
          return state;
        }
        if (!matchesFilters) {
          patched = true;
          return state;
        }
        updateColumnTotals(target, 1, story.estimate ?? 0);
        if (!target.has_more || target.stories.length < PER_STATE_LIMIT) {
          upsertLoadedStory(target, story);
        }
        patched = !target.has_more;
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

      if (!fromCol || fromIdx < 0) {
        if (!matchesFilters) {
          patched = true;
          return state;
        }
        return state;
      }

      const existing = fromCol.stories[fromIdx];
      const existingEstimate = existing?.estimate ?? 0;

      if (!matchesFilters) {
        if (isGroupedColumn(fromCol)) {
          patched = false;
          return state;
        }
        const removed = removeLoadedStory(fromCol, storyId);
        if (!removed) return state;
        updateColumnTotals(fromCol, -1, -(removed.estimate ?? 0));
        patched = true;
        return { columns };
      }

      if (fromCol.state.id === story.workflow_state_id) {
        if (isGroupedColumn(fromCol)) {
          fromCol.stories[fromIdx] = mergeEnrichedFields(story, existing);
          replaceGroupedStory(fromCol, fromCol.stories[fromIdx]);
          updateColumnTotals(fromCol, 0, (story.estimate ?? 0) - existingEstimate);
          patched = true;
          return { columns };
        }
        fromCol.stories[fromIdx] = mergeEnrichedFields(story, existing);
        fromCol.stories = sortStories(fromCol.stories, fromCol.state.state_type);
        updateColumnTotals(fromCol, 0, (story.estimate ?? 0) - existingEstimate);
        patched = true;
        return { columns };
      }

      if (isGroupedColumn(fromCol)) {
        patched = false;
        return state;
      }

      const removed = removeLoadedStory(fromCol, storyId);
      if (!removed) return state;
      updateColumnTotals(fromCol, -1, -(removed.estimate ?? 0));

      const toCol = columns.find((c) => c.state.id === story.workflow_state_id);
      if (!toCol) return { columns };
      if (isGroupedColumn(toCol)) {
        patched = false;
        return state;
      }

      const merged = mergeEnrichedFields(story, removed);
      updateColumnTotals(toCol, 1, story.estimate ?? 0);
      if (!toCol.has_more || toCol.stories.length < PER_STATE_LIMIT) {
        upsertLoadedStory(toCol, merged);
        patched = !toCol.has_more;
      } else {
        patched = false;
      }

      return { columns };
    });
    return patched;
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
      if (isGroupedColumn(columns[targetIndex])) return state;
      updateColumnTotals(columns[targetIndex], 1, story.estimate ?? 0);
      if (!columns[targetIndex].has_more || columns[targetIndex].stories.length < PER_STATE_LIMIT) {
        upsertLoadedStory(columns[targetIndex], story);
      }
      return { columns };
    });
    const targetColumn = get().columns.find((column) => column.state.id === story.workflow_state_id);
    if (targetColumn && isGroupedColumn(targetColumn)) {
      await get().refreshBoard();
    }
    return story;
  },

  moveStory: async ({ workspaceId, storyId, fromStateId, toStateId, toIndex }) => {
    const snapshot = cloneColumns(get().columns);
    const currentColumns = get().columns;
    const fromCurrent = currentColumns.find((column) => column.state.id === fromStateId);
    const toCurrent = currentColumns.find((column) => column.state.id === toStateId);
    const canOptimisticallyReconcile = !!fromCurrent && !!toCurrent &&
      !fromCurrent.has_more &&
      !toCurrent.has_more &&
      !isGroupedColumn(fromCurrent) &&
      !isGroupedColumn(toCurrent);

    if (canOptimisticallyReconcile) {
      set((state) => {
        const columns = cloneColumns(state.columns);
        const fromCol = columns.find((column) => column.state.id === fromStateId);
        const toCol = columns.find((column) => column.state.id === toStateId);
        if (!fromCol || !toCol) return state;

        const sourceIndex = fromCol.stories.findIndex((candidate) => candidate.id === storyId);
        if (sourceIndex === -1) return state;

        const [moving] = fromCol.stories.splice(sourceIndex, 1);
        if (!moving) return state;

        moving.workflow_state_id = toStateId;
        if (fromStateId === toStateId) {
          fromCol.stories.splice(toIndex, 0, moving);
          fromCol.stories = sortStories(fromCol.stories, fromCol.state.state_type);
          return { columns };
        }

        updateColumnTotals(fromCol, -1, -(moving.estimate ?? 0));
        updateColumnTotals(toCol, 1, moving.estimate ?? 0);
        toCol.stories.splice(toIndex, 0, moving);
        toCol.stories = sortStories(toCol.stories, toCol.state.state_type);
        return { columns };
      });
    }

    if (fromStateId === toStateId) {
      const targetColumn = get().columns.find((column) => column.state.id === toStateId);
      const nextStory = targetColumn?.stories[toIndex];
      const position = nextStory?.position ?? toIndex;
      const reorderRes = await pmStoryService.reorder(workspaceId, storyId, { position });
      if (reorderRes.error) {
        set({ columns: snapshot, error: reorderRes.error ?? 'Failed to reorder story' });
      } else if (!canOptimisticallyReconcile) {
        await get().refreshBoard();
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
      if (canOptimisticallyReconcile) {
        set((state) => {
          const columns = cloneColumns(state.columns);
          const target = columns.find((column) => column.state.id === toStateId);
          if (!target) return state;
          const idx = target.stories.findIndex((candidate) => candidate.id === storyId);
          if (idx >= 0) {
            target.stories[idx] = updatedStory;
            target.stories = sortStories(target.stories, target.state.state_type);
          }
          return { columns };
        });
      } else {
        await get().refreshBoard();
      }
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
