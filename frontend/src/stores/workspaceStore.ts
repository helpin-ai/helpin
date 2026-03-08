import { create } from 'zustand';
import type { Workspace } from '@/lib/types';

interface WorkspaceState {
  currentWorkspace: Workspace | null;
  setCurrentWorkspace: (ws: Workspace) => void;
}

export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  currentWorkspace: null,

  setCurrentWorkspace: (ws: Workspace) => {
    set({ currentWorkspace: ws });
  },
}));
