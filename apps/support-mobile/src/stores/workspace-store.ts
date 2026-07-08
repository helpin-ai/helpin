import { create } from 'zustand'

export interface CurrentWorkspace {
  id: string
  slug: string
  name: string
}

interface WorkspaceState {
  currentWorkspace: CurrentWorkspace | null
  setCurrentWorkspace: (workspace: CurrentWorkspace | null) => void
}

/**
 * Holds the workspace the inbox screen has resolved via
 * `workspacesService.getBySlug`. Set once the lookup succeeds; read by
 * anything downstream that needs the current workspace without re-fetching
 * or re-deriving it from the route slug (e.g. Task 16's realtime wiring).
 */
export const useWorkspaceStore = create<WorkspaceState>((set) => ({
  currentWorkspace: null,
  setCurrentWorkspace: (workspace) => set({ currentWorkspace: workspace }),
}))
