import { create } from 'zustand';
import type { Workspace } from '@/lib/types';
import { workspacesService } from '@/lib/services/workspacesService';

interface WorkspaceState {
  workspaces: Workspace[];
  currentWorkspace: Workspace | null;
  loading: boolean;
  error: string | null;
  loadWorkspaces: () => Promise<void>;
  setCurrentWorkspace: (ws: Workspace) => void;
  loadWorkspaceBySlug: (slug: string) => Promise<Workspace | null>;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  workspaces: [],
  currentWorkspace: null,
  loading: false,
  error: null,

  loadWorkspaces: async () => {
    set({ loading: true });
    const { data, error } = await workspacesService.list();
    set({
      workspaces: data ?? [],
      error: error ?? null,
      loading: false,
    });
  },

  setCurrentWorkspace: (ws: Workspace) => {
    set({ currentWorkspace: ws });
  },

  loadWorkspaceBySlug: async (slug: string) => {
    const { data } = await workspacesService.getBySlug(slug);
    if (data) {
      set({ currentWorkspace: data });
      return data;
    }
    return null;
  },
}));
