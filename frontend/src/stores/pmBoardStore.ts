import { create } from 'zustand';
import type {
  CreateStoryRequest,
  Story,
  StoryStateColumn,
  WorkflowWithStates,
} from '@/lib/pmTypes';
import { pmWorkflowService } from '@/lib/services/pmWorkflowService';
import { pmStoryService } from '@/lib/services/pmStoryService';

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
  loadBoard: (workspaceId: string, workflowId?: string) => Promise<void>;
  setWorkflow: (workflowId: string) => Promise<void>;
  refreshBoard: () => Promise<void>;
  createStory: (payload: CreateStoryRequest) => Promise<Story | null>;
  moveStory: (payload: MovePayload) => Promise<void>;
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

export const usePMBoardStore = create<PMBoardState>((set, get) => ({
  workspaceId: null,
  workflows: [],
  workflow: null,
  columns: [],
  loading: false,
  error: null,

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

    const boardRes = await pmStoryService.listBoard(workspaceId, selected.workflow.id);
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

  refreshBoard: async () => {
    const workspaceId = get().workspaceId;
    const workflowId = get().workflow?.workflow.id;
    if (!workspaceId || !workflowId) return;
    const boardRes = await pmStoryService.listBoard(workspaceId, workflowId);
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
}));
