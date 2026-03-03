import { create } from 'zustand';
import type {
  PaginatedResponse,
  Story,
  StoryDetail,
  UpdateStoryRequest,
} from '@/lib/pmTypes';
import { pmStoryService } from '@/lib/services/pmStoryService';

interface PMStoryFilters {
  team_id?: string;
  epic_id?: string;
  iteration_id?: string;
  workflow_id?: string;
  state_id?: string;
  story_type?: string;
  owner_id?: string;
  label_id?: string;
  priority?: string;
  archived?: boolean;
}

interface PMStoryState {
  stories: Story[];
  currentStory: StoryDetail | null;
  filters: PMStoryFilters;
  page: number;
  perPage: number;
  total: number;
  totalPages: number;
  loading: boolean;
  error: string | null;
  loadStories: (workspaceId: string, opts?: { page?: number; perPage?: number; filters?: PMStoryFilters }) => Promise<void>;
  setFilters: (filters: PMStoryFilters) => void;
  loadStory: (workspaceId: string, id: string) => Promise<StoryDetail | null>;
  loadStoryByDisplayId: (workspaceId: string, displayId: number) => Promise<StoryDetail | null>;
  updateCurrentStory: (workspaceId: string, patch: UpdateStoryRequest) => Promise<StoryDetail | null>;
  clearCurrentStory: () => void;
}

const normalize = <T,>(payload: PaginatedResponse<T> | null): PaginatedResponse<T> | null => payload;

export const usePMStoryStore = create<PMStoryState>((set, get) => ({
  stories: [],
  currentStory: null,
  filters: {},
  page: 1,
  perPage: 50,
  total: 0,
  totalPages: 0,
  loading: false,
  error: null,

  loadStories: async (workspaceId, opts) => {
    const page = opts?.page ?? get().page;
    const perPage = opts?.perPage ?? get().perPage;
    const filters = opts?.filters ?? get().filters;

    set({ loading: true, error: null });
    const { data, error } = await pmStoryService.list(workspaceId, {
      page,
      per_page: perPage,
      ...filters,
    });

    const normalized = normalize(data ?? null);
    if (error || !normalized) {
      set({ loading: false, error: error ?? 'Failed to load stories' });
      return;
    }

    set({
      stories: normalized.data,
      page: normalized.page,
      perPage: normalized.per_page,
      total: normalized.total,
      totalPages: normalized.total_pages,
      loading: false,
    });
  },

  setFilters: (filters) => set({ filters }),

  loadStory: async (workspaceId, id) => {
    set({ loading: true, error: null });
    const { data, error } = await pmStoryService.get(workspaceId, id);
    if (error || !data) {
      set({ loading: false, error: error ?? 'Failed to load story' });
      return null;
    }
    set({ currentStory: data, loading: false });
    return data;
  },

  loadStoryByDisplayId: async (workspaceId, displayId) => {
    set({ loading: true, error: null });
    const { data, error } = await pmStoryService.getByDisplayId(workspaceId, displayId);
    if (error || !data) {
      set({ loading: false, error: error ?? 'Failed to load story' });
      return null;
    }
    set({ currentStory: data, loading: false });
    return data;
  },

  updateCurrentStory: async (workspaceId, patch) => {
    const current = get().currentStory;
    if (!current) return null;
    const { data, error } = await pmStoryService.update(workspaceId, current.story.id, patch);
    if (error || !data) {
      set({ error: error ?? 'Failed to update story' });
      return null;
    }
    set({ currentStory: data });
    return data;
  },

  clearCurrentStory: () => set({ currentStory: null }),
}));
